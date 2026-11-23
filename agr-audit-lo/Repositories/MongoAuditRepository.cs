using System.Runtime.CompilerServices;
using Agora.Audit.Models;
using MongoDB.Driver;

namespace Agora.Audit.Repositories;

/// <summary>Colección events en agr-audit-db (MongoDB). Conector c16.</summary>
public sealed class MongoAuditRepository : IAuditRepository
{
    public const string CollectionName = "events";
    private const string EventIdIndex = "event_id_unique";
    private const string SeqIndex = "seq_unique";

    private readonly IMongoCollection<AuditEvent> _events;

    public MongoAuditRepository(IMongoDatabase database)
    {
        _events = database.GetCollection<AuditEvent>(CollectionName);
    }

    private static FilterDefinition<AuditEvent> Chained => Builders<AuditEvent>.Filter.Exists(e => e.Seq);
    private static FilterDefinition<AuditEvent> Unchained => Builders<AuditEvent>.Filter.Exists(e => e.Seq, false);

    public async Task EnsureIndexesAsync(CancellationToken ct)
    {
        var keys = Builders<AuditEvent>.IndexKeys;
        await _events.Indexes.CreateManyAsync(
        [
            new CreateIndexModel<AuditEvent>(keys.Ascending(e => e.EventId),
                new CreateIndexOptions { Name = EventIdIndex, Unique = true }),
            new CreateIndexModel<AuditEvent>(
                keys.Ascending(e => e.ElectionId).Descending(e => e.RecordedAt),
                new CreateIndexOptions { Name = "election_recorded" }),
            // S5: un seq no se repite. Parcial para que los eventos de antes de S5
            // (sin seq) no choquen entre sí mientras se encadenan.
            new CreateIndexModel<AuditEvent>(keys.Ascending(e => e.Seq),
                new CreateIndexOptions<AuditEvent>
                {
                    Name = SeqIndex,
                    Unique = true,
                    PartialFilterExpression = Chained,
                }),
        ], ct);
    }

    public async Task<InsertOutcome> InsertAsync(AuditEvent auditEvent, CancellationToken ct)
    {
        try
        {
            await _events.InsertOneAsync(auditEvent, cancellationToken: ct);
            return InsertOutcome.Inserted;
        }
        catch (MongoWriteException ex) when (ex.WriteError?.Category == ServerErrorCategory.DuplicateKey)
        {
            // Si chocó el seq, otro proceso escribió primero; si chocó el event_id (o el
            // _id, que es el mismo), es una entrega repetida de RabbitMQ.
            return ex.WriteError.Message.Contains(SeqIndex, StringComparison.Ordinal)
                ? InsertOutcome.DuplicateSeq
                : InsertOutcome.DuplicateEvent;
        }
    }

    public async Task<AuditEvent?> GetHeadAsync(CancellationToken ct) =>
        await _events.Find(Chained)
            .Sort(Builders<AuditEvent>.Sort.Descending(e => e.Seq))
            .Limit(1)
            .FirstOrDefaultAsync(ct);

    public async Task<IReadOnlyList<AuditEvent>> ListUnchainedAsync(CancellationToken ct) =>
        await _events.Find(Unchained)
            .Sort(Builders<AuditEvent>.Sort.Ascending(e => e.RecordedAt).Ascending(e => e.OccurredAt).Ascending(e => e.Id))
            .ToListAsync(ct);

    public async Task SealLegacyAsync(AuditEvent auditEvent, CancellationToken ct)
    {
        var filter = Builders<AuditEvent>.Filter.Eq(e => e.Id, auditEvent.Id) & Unchained;
        var update = Builders<AuditEvent>.Update
            .Set(e => e.Seq, auditEvent.Seq)
            .Set(e => e.PrevHash, auditEvent.PrevHash)
            .Set(e => e.Hash, auditEvent.Hash);
        await _events.UpdateOneAsync(filter, update, cancellationToken: ct);
    }

    public async IAsyncEnumerable<AuditEvent> StreamAllAsync([EnumeratorCancellation] CancellationToken ct)
    {
        var bySeq = Builders<AuditEvent>.Sort.Ascending(e => e.Seq);
        using (var chained = await _events.Find(Chained).Sort(bySeq).ToCursorAsync(ct))
        {
            while (await chained.MoveNextAsync(ct))
            {
                foreach (var e in chained.Current) yield return e;
            }
        }
        using var unchained = await _events.Find(Unchained).ToCursorAsync(ct);
        while (await unchained.MoveNextAsync(ct))
        {
            foreach (var e in unchained.Current) yield return e;
        }
    }

    public async Task<IReadOnlyList<AuditEvent>> ListAsync(long? electionId, int limit, CancellationToken ct)
    {
        var filter = electionId is { } id
            ? Builders<AuditEvent>.Filter.Eq(e => e.ElectionId, id)
            : Builders<AuditEvent>.Filter.Empty;
        var sort = Builders<AuditEvent>.Sort
            .Descending(e => e.RecordedAt)
            .Descending(e => e.OccurredAt)
            .Descending(e => e.Id);
        return await _events.Find(filter).Sort(sort).Limit(limit).ToListAsync(ct);
    }

    public Task<long> CountAsync(long electionId, CancellationToken ct) =>
        _events.CountDocumentsAsync(Builders<AuditEvent>.Filter.Eq(e => e.ElectionId, electionId), cancellationToken: ct);
}
