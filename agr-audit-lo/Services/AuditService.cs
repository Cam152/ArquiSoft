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

public sealed record RecordResult(
    RecordOutcome Outcome,
    string? EventId = null,
    string? Error = null,
    long? Seq = null,
    string? Hash = null);

/// <summary>Reglas de la auditoría: qué se guarda, cómo se encadena y cómo se consulta.</summary>
public sealed class AuditService
{
    public const int DefaultLimit = 50;
    public const int MaxLimit = 1000;

    /// <summary>Máximo de problemas que se listan en un informe de verificación.</summary>
    public const int MaxProblems = 100;

    private const int MaxSeqRetries = 5;

    private readonly IAuditRepository _repository;
    private readonly TimeProvider _clock;
    private readonly HashChain _chain;

    // Una sola escritura a la vez: cada evento necesita el hash del anterior.
    private readonly SemaphoreSlim _appendLock = new(1, 1);

    // Cabeza de la cadena según este proceso (último seq y hash que escribió o leyó).
    // Vive fuera de la base: si alguien borra los últimos eventos en agr-audit-db,
    // la verificación lo nota porque la base termina antes que esta cabeza.
    private ChainHead? _head;

    public AuditService(IAuditRepository repository, TimeProvider clock, HashChain chain)
    {
        _repository = repository;
        _clock = clock;
        _chain = chain;
    }

    /// <summary>Cabeza conocida (null si aún no se ha escrito ni leído).</summary>
    public ChainHead? KnownHead => Volatile.Read(ref _head);

    /// <summary>
    /// Al arrancar: si la cadena está vacía y hay eventos de antes de S5, los encadena
    /// una sola vez (en el orden en que se registraron). Después carga la cabeza.
    /// Si la cadena ya existe, un documento sin seq es una inserción por fuera del
    /// servicio: no se encadena, la verificación lo señala.
    /// </summary>
    public async Task<int> InitializeAsync(CancellationToken ct)
    {
        await _appendLock.WaitAsync(ct);
        try
        {
            var head = await _repository.GetHeadAsync(ct);
            var sealedCount = 0;
            if (head is null)
            {
                var seq = 0L;
                var prev = HashChain.Genesis;
                foreach (var legacy in await _repository.ListUnchainedAsync(ct))
                {
                    _chain.Seal(legacy, ++seq, prev);
                    await _repository.SealLegacyAsync(legacy, ct);
                    prev = legacy.Hash!;
                    sealedCount++;
                }
                head = await _repository.GetHeadAsync(ct);
            }
            Volatile.Write(ref _head, head is null ? null : new ChainHead(head.Seq!.Value, head.Hash!));
            return sealedCount;
        }
        finally
        {
            _appendLock.Release();
        }
    }

    /// <summary>
    /// Registra un VoteRegistered al final de la cadena. Si la base falla, la excepción
    /// sube para que el consumidor devuelva el mensaje a la cola.
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

        await _appendLock.WaitAsync(ct);
        try
        {
            var head = Volatile.Read(ref _head);
            for (var attempt = 1; ; attempt++)
            {
                var seq = (head?.Seq ?? 0) + 1;
                _chain.Seal(auditEvent, seq, head?.Hash ?? HashChain.Genesis);

                switch (await _repository.InsertAsync(auditEvent, ct))
                {
                    case InsertOutcome.Inserted:
                        Volatile.Write(ref _head, new ChainHead(seq, auditEvent.Hash!));
                        return new RecordResult(RecordOutcome.Recorded, evt.EventId, Seq: seq, Hash: auditEvent.Hash);
                    case InsertOutcome.DuplicateEvent:
                        return new RecordResult(RecordOutcome.Duplicate, evt.EventId);
                    default:
                        // Otro proceso ocupó ese seq: se relee la cabeza desde la base.
                        if (attempt >= MaxSeqRetries)
                        {
                            throw new InvalidOperationException("No se pudo obtener un seq libre en la cadena de auditoría");
                        }
                        var dbHead = await _repository.GetHeadAsync(ct);
                        head = dbHead is null ? null : new ChainHead(dbHead.Seq!.Value, dbHead.Hash!);
                        break;
                }
            }
        }
        finally
        {
            _appendLock.Release();
        }
    }

    /// <summary>
    /// Recorre toda la cadena y devuelve qué eventos no cuadran (S5).
    ///   altered     el hash guardado no coincide con el recalculado: se cambió el evento
    ///   missing     falta uno o más seq: se borraron eventos
    ///   broken_link prev_hash no es el hash del evento anterior
    ///   out_of_order seq repetido o hacia atrás
    ///   unchained   documento sin seq ni hash: se insertó por fuera del servicio
    ///   truncated   la cadena termina antes de la cabeza que conoce el servicio: se borraron los últimos
    /// </summary>
    public async Task<VerifyReport> VerifyAsync(CancellationToken ct)
    {
        var known = Volatile.Read(ref _head);
        var problems = new List<ChainProblem>();
        var problemCount = 0;
        long total = 0, chained = 0, lastSeq = 0;
        string lastHash = HashChain.Genesis;

        void Report(ChainProblem p)
        {
            problemCount++;
            if (problems.Count < MaxProblems) problems.Add(p);
        }

        await foreach (var e in _repository.StreamAllAsync(ct))
        {
            total++;
            if (e.Seq is not { } seq || e.Hash is null || e.PrevHash is null)
            {
                Report(new ChainProblem("unchained", e.Seq, e.EventId,
                    "Evento sin seq o sin hash: no lo registró el Audit Service"));
                continue;
            }

            chained++;
            if (!_chain.Matches(e))
            {
                Report(new ChainProblem("altered", seq, e.EventId,
                    "El hash no coincide con el contenido: el evento fue modificado"));
            }

            if (seq <= lastSeq)
            {
                Report(new ChainProblem("out_of_order", seq, e.EventId,
                    $"seq {seq} repetido o fuera de orden (el anterior es {lastSeq})"));
            }
            else if (seq > lastSeq + 1)
            {
                var from = lastSeq + 1;
                var to = seq - 1;
                Report(new ChainProblem("missing", from, null,
                    from == to ? $"Falta el evento seq {from}: fue borrado"
                               : $"Faltan los eventos seq {from} a {to}: fueron borrados"));
            }
            else if (!string.Equals(e.PrevHash, lastHash, StringComparison.Ordinal))
            {
                Report(new ChainProblem("broken_link", seq, e.EventId,
                    "prev_hash no es el hash del evento anterior"));
            }

            lastSeq = Math.Max(lastSeq, seq);
            lastHash = e.Hash;
        }

        if (known is not null && known.Seq > lastSeq)
        {
            Report(new ChainProblem("truncated", lastSeq + 1, null,
                $"La cadena termina en seq {lastSeq}, pero el servicio registró hasta seq {known.Seq}: se borraron los últimos eventos"));
        }

        return new VerifyReport(
            Valid: problemCount == 0,
            TotalEvents: total,
            ChainedEvents: chained,
            HeadSeq: lastSeq,
            HeadHash: lastSeq == 0 ? null : lastHash,
            KnownHeadSeq: known?.Seq,
            CheckedAt: _clock.GetUtcNow().UtcDateTime,
            ProblemCount: problemCount,
            Problems: problems);
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

public sealed record ChainHead(long Seq, string Hash);

public sealed record ChainProblem(string Kind, long? Seq, string? EventId, string Detail);

public sealed record VerifyReport(
    bool Valid,
    long TotalEvents,
    long ChainedEvents,
    long HeadSeq,
    string? HeadHash,
    long? KnownHeadSeq,
    DateTime CheckedAt,
    long ProblemCount,
    IReadOnlyList<ChainProblem> Problems);
