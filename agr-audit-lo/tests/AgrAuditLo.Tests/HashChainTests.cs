using System.Text;
using Agora.Audit.Models;
using Agora.Audit.Services;
using Xunit;

namespace Agora.Audit.Tests;

/// <summary>S5: cadena de hashes y verificación (tamper-evident log).</summary>
public class HashChainTests
{
    private static readonly DateTimeOffset Now = new(2026, 11, 22, 15, 42, 17, TimeSpan.Zero);

    private readonly InMemoryAuditRepository _repo = new();
    private readonly AuditService _service;

    public HashChainTests()
    {
        _service = new AuditService(_repo, new FixedClock(Now), TestChain.Create());
    }

    private static ReadOnlyMemory<byte> Body(string eventId, long electionId = 1) =>
        Encoding.UTF8.GetBytes(
            $$"""{"event_id":"{{eventId}}","type":"VoteRegistered","version":1,"election_id":{{electionId}},"occurred_at":"2026-11-22T14:25:09Z"}""");

    private async Task Record(params string[] ids)
    {
        foreach (var id in ids) await _service.RecordAsync(Body(id), default);
    }

    private AuditEvent Stored(long seq) => _repo.Events.Single(e => e.Seq == seq);

    // ---- Cálculo del hash -------------------------------------------------

    [Fact]
    public void ElHashEsDeterministaYDependeDeCadaCampo()
    {
        var chain = TestChain.Create();
        var e = new AuditEvent
        {
            Id = "e1", EventId = "e1", Type = "VoteRegistered", ElectionId = 1,
            OccurredAt = new DateTime(2026, 11, 22, 14, 0, 0, DateTimeKind.Utc),
            RecordedAt = new DateTime(2026, 11, 22, 15, 0, 0, DateTimeKind.Utc),
        };
        var h = chain.Compute(e, 1, HashChain.Genesis);
        Assert.Matches("^[0-9a-f]{64}$", h);
        Assert.Equal(h, chain.Compute(e, 1, HashChain.Genesis));

        Assert.NotEqual(h, chain.Compute(e, 2, HashChain.Genesis));
        Assert.NotEqual(h, chain.Compute(e, 1, new string('1', 64)));
        e.ElectionId = 2;
        Assert.NotEqual(h, chain.Compute(e, 1, HashChain.Genesis));
    }

    [Fact]
    public void ConOtraLlaveElHashCambia()
    {
        var e = new AuditEvent { Id = "e1", EventId = "e1", Type = "VoteRegistered", ElectionId = 1 };
        var other = new HashChain("otra-llave-distinta-0123456789abcdef-xyz");
        Assert.NotEqual(TestChain.Create().Compute(e, 1, HashChain.Genesis), other.Compute(e, 1, HashChain.Genesis));
    }

    [Theory]
    [InlineData("")]
    [InlineData("corta")]
    public void RechazaLlavesCortas(string key)
    {
        Assert.Throws<ArgumentException>(() => new HashChain(key));
    }

    // ---- Encadenado al registrar -------------------------------------------

    [Fact]
    public async Task CadaEventoApuntaAlAnterior()
    {
        await Record("a", "b", "c");

        Assert.Equal([1L, 2L, 3L], _repo.Events.Select(e => e.Seq!.Value));
        Assert.Equal(HashChain.Genesis, Stored(1).PrevHash);
        Assert.Equal(Stored(1).Hash, Stored(2).PrevHash);
        Assert.Equal(Stored(2).Hash, Stored(3).PrevHash);
        Assert.Equal(new ChainHead(3, Stored(3).Hash!), _service.KnownHead);
    }

    [Fact]
    public async Task UnEventoRepetidoNoGastaUnSeq()
    {
        await Record("a", "a", "b");
        Assert.Equal([1L, 2L], _repo.Events.Select(e => e.Seq!.Value));
        Assert.True((await _service.VerifyAsync(default)).Valid);
    }

    [Fact]
    public async Task SiOtroProcesoOcupaElSeqReleeLaCabeza()
    {
        await Record("a");
        _repo.StealNextSeqs = 1;

        var result = await _service.RecordAsync(Body("b"), default);

        Assert.Equal(RecordOutcome.Recorded, result.Outcome);
        Assert.Equal(3, result.Seq);
        Assert.Equal("h2", _repo.Events.Single(e => e.EventId == "b").PrevHash);
    }

    [Fact]
    public async Task ContinuaLaCadenaQueYaEstabaEnLaBase()
    {
        await Record("a", "b");
        var restarted = new AuditService(_repo, new FixedClock(Now), TestChain.Create());
        await restarted.InitializeAsync(default);

        await restarted.RecordAsync(Body("c"), default);

        Assert.Equal(Stored(2).Hash, Stored(3).PrevHash);
        Assert.True((await restarted.VerifyAsync(default)).Valid);
    }

    // ---- Eventos de antes de S5 ---------------------------------------------

    [Fact]
    public async Task AlArrancarEncadenaLosEventosDeAntesDeS5()
    {
        _repo.Events.Add(Legacy("viejo-2", hour: 11));
        _repo.Events.Add(Legacy("viejo-1", hour: 10));

        Assert.Equal(2, await _service.InitializeAsync(default));
        await Record("nuevo");

        Assert.Equal("viejo-1", Stored(1).EventId);
        Assert.Equal("viejo-2", Stored(2).EventId);
        Assert.Equal("nuevo", Stored(3).EventId);
        Assert.True((await _service.VerifyAsync(default)).Valid);
    }

    [Fact]
    public async Task SiYaHayCadenaNoEncadenaDocumentosSueltos()
    {
        await Record("a");
        _repo.Events.Add(Legacy("intruso", hour: 9));

        var restarted = new AuditService(_repo, new FixedClock(Now), TestChain.Create());
        Assert.Equal(0, await restarted.InitializeAsync(default));

        var report = await restarted.VerifyAsync(default);
        Assert.False(report.Valid);
        Assert.Equal("unchained", Assert.Single(report.Problems).Kind);
    }

    // ---- Verificación: cada tipo de alteración -----------------------------

    [Fact]
    public async Task UnaCadenaIntactaEsValida()
    {
        await Record("a", "b", "c");
        var report = await _service.VerifyAsync(default);

        Assert.True(report.Valid);
        Assert.Equal(3, report.TotalEvents);
        Assert.Equal(3, report.ChainedEvents);
        Assert.Equal(3, report.HeadSeq);
        Assert.Equal(Stored(3).Hash, report.HeadHash);
        Assert.Equal(3, report.KnownHeadSeq);
        Assert.Empty(report.Problems);
    }

    [Fact]
    public async Task UnaCadenaVaciaEsValida()
    {
        var report = await _service.VerifyAsync(default);
        Assert.True(report.Valid);
        Assert.Equal(0, report.HeadSeq);
        Assert.Null(report.HeadHash);
    }

    [Theory]
    [InlineData("election_id")]
    [InlineData("type")]
    [InlineData("occurred_at")]
    [InlineData("recorded_at")]
    [InlineData("event_id")]
    public async Task SenalaElEventoModificado(string field)
    {
        await Record("a", "b", "c");
        var target = Stored(2);
        switch (field)
        {
            case "election_id": target.ElectionId = 99; break;
            case "type": target.Type = "VoteDeleted"; break;
            case "occurred_at": target.OccurredAt = target.OccurredAt.AddHours(-3); break;
            case "recorded_at": target.RecordedAt = target.RecordedAt.AddDays(1); break;
            case "event_id": target.EventId = "falso"; break;
        }

        var report = await _service.VerifyAsync(default);

        Assert.False(report.Valid);
        var p = Assert.Single(report.Problems);
        Assert.Equal("altered", p.Kind);
        Assert.Equal(2, p.Seq);
    }

    [Fact]
    public async Task RecalcularElHashSinLaLlaveNoTapaElCambio()
    {
        await Record("a", "b", "c");
        var target = Stored(2);
        target.ElectionId = 99;
        // El atacante recalcula con una llave que no es la del servicio.
        new HashChain("llave-adivinada-por-el-atacante-000000000").Seal(target, 2, target.PrevHash!);

        var report = await _service.VerifyAsync(default);

        Assert.False(report.Valid);
        Assert.Contains(report.Problems, p => p.Kind == "altered" && p.Seq == 2);
    }

    [Fact]
    public async Task SenalaLosEventosBorradosEnMedio()
    {
        await Record("a", "b", "c", "d", "e");
        _repo.Events.RemoveAll(e => e.Seq is 2 or 3);

        var report = await _service.VerifyAsync(default);

        Assert.False(report.Valid);
        var p = Assert.Single(report.Problems);
        Assert.Equal("missing", p.Kind);
        Assert.Equal(2, p.Seq);
        Assert.Contains("2 a 3", p.Detail);
    }

    [Fact]
    public async Task SenalaLosUltimosEventosBorrados()
    {
        await Record("a", "b", "c");
        _repo.Events.RemoveAll(e => e.Seq == 3);

        var report = await _service.VerifyAsync(default);

        Assert.False(report.Valid);
        var p = Assert.Single(report.Problems);
        Assert.Equal("truncated", p.Kind);
        Assert.Equal(3, p.Seq);
        Assert.Equal(2, report.HeadSeq);
        Assert.Equal(3, report.KnownHeadSeq);
    }

    [Fact]
    public async Task SenalaUnEventoInsertadoAMano()
    {
        await Record("a");
        _repo.Events.Add(Legacy("intruso", hour: 15));

        var report = await _service.VerifyAsync(default);

        Assert.False(report.Valid);
        var p = Assert.Single(report.Problems);
        Assert.Equal("unchained", p.Kind);
        Assert.Equal("intruso", p.EventId);
        Assert.Equal(2, report.TotalEvents);
        Assert.Equal(1, report.ChainedEvents);
    }

    [Fact]
    public async Task SenalaUnEnlaceRoto()
    {
        await Record("a", "b", "c");
        // Se cambia prev_hash y se recalcula el hash con la llave correcta:
        // el evento cuadra consigo mismo pero ya no apunta al anterior.
        var target = Stored(3);
        TestChain.Create().Seal(target, 3, new string('f', 64));

        var report = await _service.VerifyAsync(default);

        var p = Assert.Single(report.Problems);
        Assert.Equal("broken_link", p.Kind);
        Assert.Equal(3, p.Seq);
    }

    [Fact]
    public async Task ElInformeListaComoMaximoCienProblemas()
    {
        for (var i = 0; i < AuditService.MaxProblems + 20; i++) await Record($"e{i:D3}");
        foreach (var e in _repo.Events) e.ElectionId = 7;

        var report = await _service.VerifyAsync(default);

        Assert.Equal(AuditService.MaxProblems + 20, report.ProblemCount);
        Assert.Equal(AuditService.MaxProblems, report.Problems.Count);
    }

    private static AuditEvent Legacy(string id, int hour) => new()
    {
        Id = id, EventId = id, Type = "VoteRegistered", ElectionId = 1,
        OccurredAt = new DateTime(2026, 11, 1, hour, 0, 0, DateTimeKind.Utc),
        RecordedAt = new DateTime(2026, 11, 1, hour, 0, 0, DateTimeKind.Utc),
    };
}
