using System.Text;
using Agora.Audit.Services;
using Xunit;

namespace Agora.Audit.Tests;

public class AuditServiceTests
{
    private static readonly DateTimeOffset Now = new(2026, 11, 3, 15, 42, 17, TimeSpan.Zero);

    private readonly InMemoryAuditRepository _repo = new();
    private readonly AuditService _service;

    public AuditServiceTests()
    {
        _service = new AuditService(_repo, new FixedClock(Now));
    }

    private static ReadOnlyMemory<byte> Body(string eventId, long electionId, string occurredAt = "2026-11-03T14:25:09Z") =>
        Encoding.UTF8.GetBytes(
            $$"""{"event_id":"{{eventId}}","type":"VoteRegistered","version":1,"election_id":{{electionId}},"candidate_id":2,"occurred_at":"{{occurredAt}}"}""");

    [Fact]
    public async Task GuardaElEventoConLasHorasRedondeadas()
    {
        var result = await _service.RecordAsync(Body("e1", 1), default);

        Assert.Equal(RecordOutcome.Recorded, result.Outcome);
        Assert.Equal("e1", result.EventId);
        var saved = Assert.Single(_repo.Events);
        Assert.Equal("e1", saved.Id);
        Assert.Equal("e1", saved.EventId);
        Assert.Equal("VoteRegistered", saved.Type);
        Assert.Equal(1, saved.ElectionId);
        Assert.Equal(new DateTime(2026, 11, 3, 14, 0, 0, DateTimeKind.Utc), saved.OccurredAt);
        Assert.Equal(new DateTime(2026, 11, 3, 15, 0, 0, DateTimeKind.Utc), saved.RecordedAt);
    }

    [Fact]
    public async Task UnEventoRepetidoNoSeGuardaDosVeces()
    {
        Assert.Equal(RecordOutcome.Recorded, (await _service.RecordAsync(Body("e1", 1), default)).Outcome);
        Assert.Equal(RecordOutcome.Duplicate, (await _service.RecordAsync(Body("e1", 1), default)).Outcome);
        Assert.Single(_repo.Events);
    }

    [Fact]
    public async Task UnMensajeInvalidoNoLlegaALaBase()
    {
        var result = await _service.RecordAsync(Encoding.UTF8.GetBytes("{}"), default);

        Assert.Equal(RecordOutcome.Invalid, result.Outcome);
        Assert.NotNull(result.Error);
        Assert.Empty(_repo.Events);
    }

    [Fact]
    public async Task SiLaBaseFallaLaExcepcionSube()
    {
        _repo.Fail = true;
        await Assert.ThrowsAsync<InvalidOperationException>(() => _service.RecordAsync(Body("e1", 1), default));
    }

    [Fact]
    public async Task ConvierteZonasHorariasAUtc()
    {
        await _service.RecordAsync(Body("e1", 1, "2026-11-03T09:59:59-05:00"), default);
        Assert.Equal(new DateTime(2026, 11, 3, 14, 0, 0, DateTimeKind.Utc), _repo.Events[0].OccurredAt);
    }

    [Fact]
    public async Task ListaYCuentaPorEleccion()
    {
        await _service.RecordAsync(Body("a", 1), default);
        await _service.RecordAsync(Body("b", 1), default);
        await _service.RecordAsync(Body("c", 2), default);

        Assert.Equal(2, await _service.CountAsync(1, default));
        Assert.Equal(1, await _service.CountAsync(2, default));
        Assert.Equal(0, await _service.CountAsync(9, default));
        Assert.Equal(3, (await _service.ListAsync(null, 50, default)).Count);
        Assert.All(await _service.ListAsync(1, 50, default), e => Assert.Equal(1, e.ElectionId));
        Assert.Single(await _service.ListAsync(null, 1, default));
    }
}
