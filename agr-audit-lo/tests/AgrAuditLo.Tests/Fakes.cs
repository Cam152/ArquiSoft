using Agora.Audit.Models;
using Agora.Audit.Repositories;
using Agora.Audit.Services;

namespace Agora.Audit.Tests;

/// <summary>Repositorio en memoria con el mismo comportamiento que el de MongoDB.</summary>
internal sealed class InMemoryAuditRepository : IAuditRepository
{
    public List<AuditEvent> Events { get; } = [];
    public bool Fail { get; set; }

    /// <summary>Simula otro proceso que ocupa estos seq justo antes de insertar.</summary>
    public int StealNextSeqs { get; set; }

    public Task EnsureIndexesAsync(CancellationToken ct) => Task.CompletedTask;

    public Task<InsertOutcome> InsertAsync(AuditEvent auditEvent, CancellationToken ct)
    {
        if (Fail) throw new InvalidOperationException("base caída");
        if (Events.Any(e => e.EventId == auditEvent.EventId)) return Task.FromResult(InsertOutcome.DuplicateEvent);
        if (StealNextSeqs > 0)
        {
            StealNextSeqs--;
            Events.Add(new AuditEvent { Id = $"otro-{auditEvent.Seq}", EventId = $"otro-{auditEvent.Seq}", Seq = auditEvent.Seq, PrevHash = auditEvent.PrevHash, Hash = $"h{auditEvent.Seq}" });
            return Task.FromResult(InsertOutcome.DuplicateSeq);
        }
        if (auditEvent.Seq is not null && Events.Any(e => e.Seq == auditEvent.Seq)) return Task.FromResult(InsertOutcome.DuplicateSeq);
        Events.Add(Copy(auditEvent));
        return Task.FromResult(InsertOutcome.Inserted);
    }

    public Task<AuditEvent?> GetHeadAsync(CancellationToken ct) =>
        Task.FromResult(Events.Where(e => e.Seq is not null).OrderByDescending(e => e.Seq).Select(Copy).FirstOrDefault());

    public Task<IReadOnlyList<AuditEvent>> ListUnchainedAsync(CancellationToken ct)
    {
        IReadOnlyList<AuditEvent> list = Events.Where(e => e.Seq is null)
            .OrderBy(e => e.RecordedAt).ThenBy(e => e.OccurredAt).ThenBy(e => e.Id, StringComparer.Ordinal)
            .Select(Copy).ToList();
        return Task.FromResult(list);
    }

    public Task SealLegacyAsync(AuditEvent auditEvent, CancellationToken ct)
    {
        var stored = Events.Single(e => e.Id == auditEvent.Id);
        if (stored.Seq is null)
        {
            stored.Seq = auditEvent.Seq;
            stored.PrevHash = auditEvent.PrevHash;
            stored.Hash = auditEvent.Hash;
        }
        return Task.CompletedTask;
    }

    public async IAsyncEnumerable<AuditEvent> StreamAllAsync([System.Runtime.CompilerServices.EnumeratorCancellation] CancellationToken ct)
    {
        await Task.Yield();
        foreach (var e in Events.Where(e => e.Seq is not null).OrderBy(e => e.Seq).ToList()) yield return Copy(e);
        foreach (var e in Events.Where(e => e.Seq is null).ToList()) yield return Copy(e);
    }

    public Task<IReadOnlyList<AuditEvent>> ListAsync(long? electionId, int limit, CancellationToken ct)
    {
        IReadOnlyList<AuditEvent> list = Events
            .Where(e => electionId is null || e.ElectionId == electionId)
            .OrderByDescending(e => e.RecordedAt)
            .ThenByDescending(e => e.OccurredAt)
            .ThenByDescending(e => e.Id, StringComparer.Ordinal)
            .Take(limit)
            .ToList();
        return Task.FromResult(list);
    }

    public Task<long> CountAsync(long electionId, CancellationToken ct) =>
        Task.FromResult((long)Events.Count(e => e.ElectionId == electionId));

    // La base devuelve copias: cambiar lo que se leyó no cambia lo guardado.
    private static AuditEvent Copy(AuditEvent e) => new()
    {
        Id = e.Id, EventId = e.EventId, Type = e.Type, ElectionId = e.ElectionId,
        OccurredAt = e.OccurredAt, RecordedAt = e.RecordedAt, Seq = e.Seq, PrevHash = e.PrevHash, Hash = e.Hash,
    };
}

internal sealed class FixedClock(DateTimeOffset now) : TimeProvider
{
    public DateTimeOffset Now { get; set; } = now;
    public override DateTimeOffset GetUtcNow() => Now;
}

internal static class TestChain
{
    public const string Key = "llave-de-pruebas-0123456789abcdef-0123";
    public static HashChain Create() => new(Key);
}
