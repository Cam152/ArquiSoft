using Agora.Audit.Models;

namespace Agora.Audit.Repositories;

/// <summary>
/// Acceso a agr-audit-db. Solo inserta y consulta: el registro es inmutable.
/// </summary>
public interface IAuditRepository
{
    /// <summary>Crea los índices (event_id único). Idempotente.</summary>
    Task EnsureIndexesAsync(CancellationToken ct);

    /// <summary>Inserta el evento. Devuelve false si ese event_id ya estaba guardado.</summary>
    Task<bool> InsertAsync(AuditEvent auditEvent, CancellationToken ct);

    /// <summary>Eventos más recientes, de una elección o de todas (electionId null).</summary>
    Task<IReadOnlyList<AuditEvent>> ListAsync(long? electionId, int limit, CancellationToken ct);

    /// <summary>Total de eventos de una elección.</summary>
    Task<long> CountAsync(long electionId, CancellationToken ct);
}
