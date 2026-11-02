using Agora.Audit.Models;
using Agora.Audit.Repositories;

namespace Agora.Audit.Tests;

/// <summary>Repositorio en memoria con el mismo comportamiento que el de MongoDB.</summary>
internal sealed class InMemoryAuditRepository : IAuditRepository
{
    public List<AuditEvent> Events { get; } = [];
    public bool Fail { get; set; }

    public Task EnsureIndexesAsync(CancellationToken ct) => Task.CompletedTask;

    public Task<bool> InsertAsync(AuditEvent auditEvent, CancellationToken ct)
    {
        if (Fail) throw new InvalidOperationException("base caída");
        if (Events.Any(e => e.EventId == auditEvent.EventId)) return Task.FromResult(false);
        Events.Add(auditEvent);
        return Task.FromResult(true);
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
}

internal sealed class FixedClock(DateTimeOffset now) : TimeProvider
{
    public DateTimeOffset Now { get; set; } = now;
    public override DateTimeOffset GetUtcNow() => Now;
}
