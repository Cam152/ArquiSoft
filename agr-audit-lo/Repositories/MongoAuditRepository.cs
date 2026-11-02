using Agora.Audit.Models;
using MongoDB.Driver;

namespace Agora.Audit.Repositories;

/// <summary>Colección events en agr-audit-db (MongoDB). Conector c16.</summary>
public sealed class MongoAuditRepository : IAuditRepository
{
    public const string CollectionName = "events";

    private readonly IMongoCollection<AuditEvent> _events;

    public MongoAuditRepository(IMongoDatabase database)
    {
        _events = database.GetCollection<AuditEvent>(CollectionName);
    }

    public async Task EnsureIndexesAsync(CancellationToken ct)
    {
        var keys = Builders<AuditEvent>.IndexKeys;
        await _events.Indexes.CreateManyAsync(
        [
            new CreateIndexModel<AuditEvent>(keys.Ascending(e => e.EventId),
                new CreateIndexOptions { Name = "event_id_unique", Unique = true }),
            new CreateIndexModel<AuditEvent>(
                keys.Ascending(e => e.ElectionId).Descending(e => e.RecordedAt),
                new CreateIndexOptions { Name = "election_recorded" }),
        ], ct);
    }

    public async Task<bool> InsertAsync(AuditEvent auditEvent, CancellationToken ct)
    {
        try
        {
            await _events.InsertOneAsync(auditEvent, cancellationToken: ct);
            return true;
        }
        catch (MongoWriteException ex) when (ex.WriteError?.Category == ServerErrorCategory.DuplicateKey)
        {
            // Evento repetido (RabbitMQ entrega al menos una vez): ya estaba guardado.
            return false;
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
