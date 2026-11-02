using MongoDB.Bson.Serialization.Attributes;

namespace Agora.Audit.Models;

/// <summary>
/// Documento de la colección events en agr-audit-db. No lleva nada del votante ni
/// el candidato: solo qué pasó, en qué elección y a qué hora (redondeada).
/// </summary>
[BsonIgnoreExtraElements]
public sealed class AuditEvent
{
    // _id es el mismo event_id: un ObjectId propio revelaría la hora exacta de
    // inserción y rompería el redondeo de recorded_at.
    [BsonId]
    public string Id { get; set; } = "";

    [BsonElement("event_id")]
    public string EventId { get; set; } = "";

    [BsonElement("type")]
    public string Type { get; set; } = "";

    [BsonElement("election_id")]
    public long ElectionId { get; set; }

    [BsonElement("occurred_at")]
    [BsonDateTimeOptions(Kind = DateTimeKind.Utc)]
    public DateTime OccurredAt { get; set; }

    [BsonElement("recorded_at")]
    [BsonDateTimeOptions(Kind = DateTimeKind.Utc)]
    public DateTime RecordedAt { get; set; }
}
