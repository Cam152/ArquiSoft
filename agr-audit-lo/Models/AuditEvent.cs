using MongoDB.Bson.Serialization.Attributes;

namespace Agora.Audit.Models;

/// <summary>
/// Documento de la colección events en agr-audit-db. No lleva nada del votante ni
/// el candidato: solo qué pasó, en qué elección y a qué hora (redondeada).
///
/// S5 (tamper-evident log): cada evento ocupa una posición (seq) en una cadena de
/// hashes. hash = HMAC-SHA256(llave, campos del evento + prev_hash), así que cambiar,
/// borrar o meter un evento a mano en la base rompe la cadena desde ese punto.
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

    /// <summary>Posición en la cadena (1, 2, 3...). Null en un documento que no pasó por el servicio.</summary>
    [BsonElement("seq")]
    [BsonIgnoreIfNull]
    public long? Seq { get; set; }

    /// <summary>hash del evento anterior (64 ceros en el primero).</summary>
    [BsonElement("prev_hash")]
    [BsonIgnoreIfNull]
    public string? PrevHash { get; set; }

    /// <summary>HMAC-SHA256 en hexadecimal de este evento encadenado al anterior.</summary>
    [BsonElement("hash")]
    [BsonIgnoreIfNull]
    public string? Hash { get; set; }
}
