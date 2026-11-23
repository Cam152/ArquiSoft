using Agora.Audit.Models;

namespace Agora.Audit.Repositories;

public enum InsertOutcome
{
    Inserted,
    /// <summary>Ese event_id ya estaba guardado (entrega repetida).</summary>
    DuplicateEvent,
    /// <summary>Otro proceso ya ocupó ese seq: hay que volver a leer la cabeza de la cadena.</summary>
    DuplicateSeq,
}

/// <summary>
/// Acceso a agr-audit-db. Solo inserta y consulta: el registro es inmutable.
/// La única escritura sobre un documento existente es SealLegacyAsync, que encadena
/// una vez los eventos guardados antes de S5.
/// </summary>
public interface IAuditRepository
{
    /// <summary>Crea los índices (event_id único, seq único). Idempotente.</summary>
    Task EnsureIndexesAsync(CancellationToken ct);

    /// <summary>Inserta el evento ya encadenado.</summary>
    Task<InsertOutcome> InsertAsync(AuditEvent auditEvent, CancellationToken ct);

    /// <summary>Último evento de la cadena (mayor seq), o null si la cadena está vacía.</summary>
    Task<AuditEvent?> GetHeadAsync(CancellationToken ct);

    /// <summary>Eventos sin seq (guardados antes de S5), en el orden en que se registraron.</summary>
    Task<IReadOnlyList<AuditEvent>> ListUnchainedAsync(CancellationToken ct);

    /// <summary>Escribe seq, prev_hash y hash en un evento que aún no los tiene.</summary>
    Task SealLegacyAsync(AuditEvent auditEvent, CancellationToken ct);

    /// <summary>Todos los eventos: primero los encadenados por seq ascendente y luego los que no tienen seq.</summary>
    IAsyncEnumerable<AuditEvent> StreamAllAsync(CancellationToken ct);

    /// <summary>Eventos más recientes, de una elección o de todas (electionId null).</summary>
    Task<IReadOnlyList<AuditEvent>> ListAsync(long? electionId, int limit, CancellationToken ct);

    /// <summary>Total de eventos de una elección.</summary>
    Task<long> CountAsync(long electionId, CancellationToken ct);
}
