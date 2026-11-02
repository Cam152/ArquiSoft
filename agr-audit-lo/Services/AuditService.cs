using Agora.Audit.Models;
using Agora.Audit.Repositories;

namespace Agora.Audit.Services;

public enum RecordOutcome
{
    /// <summary>Guardado por primera vez.</summary>
    Recorded,
    /// <summary>Ya estaba guardado (entrega repetida). Se confirma igual.</summary>
    Duplicate,
    /// <summary>El mensaje no cumple el contrato. No sirve reintentarlo.</summary>
    Invalid,
}

public sealed record RecordResult(RecordOutcome Outcome, string? EventId = null, string? Error = null);

/// <summary>Reglas de la auditoría: qué se guarda y cómo se consulta.</summary>
public sealed class AuditService
{
    public const int DefaultLimit = 50;
    public const int MaxLimit = 1000;

    private readonly IAuditRepository _repository;
    private readonly TimeProvider _clock;

    public AuditService(IAuditRepository repository, TimeProvider clock)
    {
        _repository = repository;
        _clock = clock;
    }

    /// <summary>
    /// Registra un VoteRegistered. Si la base falla, la excepción sube para que el
    /// consumidor devuelva el mensaje a la cola.
    /// </summary>
    public async Task<RecordResult> RecordAsync(ReadOnlyMemory<byte> body, CancellationToken ct)
    {
        if (!VoteRegistered.TryParse(body.Span, out var evt, out var error))
        {
            return new RecordResult(RecordOutcome.Invalid, Error: error);
        }

        var auditEvent = new AuditEvent
        {
            Id = evt!.EventId,
            EventId = evt.EventId,
            Type = evt.Type,
            ElectionId = evt.ElectionId,
            OccurredAt = TruncateToHour(evt.OccurredAt),
            RecordedAt = TruncateToHour(_clock.GetUtcNow()),
        };

        var inserted = await _repository.InsertAsync(auditEvent, ct);
        return new RecordResult(inserted ? RecordOutcome.Recorded : RecordOutcome.Duplicate, evt.EventId);
    }

    public Task<IReadOnlyList<AuditEvent>> ListAsync(long? electionId, int limit, CancellationToken ct) =>
        _repository.ListAsync(electionId, limit, ct);

    public Task<long> CountAsync(long electionId, CancellationToken ct) =>
        _repository.CountAsync(electionId, ct);

    /// <summary>Redondea hacia abajo a la hora, en UTC (anonimato: README, sección 4.10).</summary>
    public static DateTime TruncateToHour(DateTimeOffset value)
    {
        var utc = value.UtcDateTime;
        return new DateTime(utc.Year, utc.Month, utc.Day, utc.Hour, 0, 0, DateTimeKind.Utc);
    }
}
