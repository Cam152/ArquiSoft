using System.Globalization;
using System.Text.Json;

namespace Agora.Audit.Models;

/// <summary>
/// Evento VoteRegistered v1 que publica agr-vote-lo (README, sección 5.3).
/// candidate_id viene en el mensaje pero la auditoría no lo usa.
/// </summary>
public sealed record VoteRegistered(string EventId, string Type, int Version, long ElectionId, DateTimeOffset OccurredAt)
{
    public const string EventType = "VoteRegistered";
    public const int SupportedVersion = 1;

    /// <summary>Lee y valida el cuerpo del mensaje. Si no sirve, devuelve false y el motivo.</summary>
    public static bool TryParse(ReadOnlySpan<byte> body, out VoteRegistered? evt, out string error)
    {
        evt = null;
        try
        {
            using var doc = JsonDocument.Parse(body.ToArray());
            var root = doc.RootElement;
            if (root.ValueKind != JsonValueKind.Object)
            {
                error = "el mensaje no es un objeto JSON";
                return false;
            }

            if (!TryString(root, "event_id", out var eventId) || eventId.Length is 0 or > 128)
            {
                error = "event_id falta o no es válido";
                return false;
            }
            if (!TryString(root, "type", out var type) || type != EventType)
            {
                error = $"type no es {EventType}";
                return false;
            }
            if (!root.TryGetProperty("version", out var v) || v.ValueKind != JsonValueKind.Number
                || !v.TryGetInt32(out var version) || version != SupportedVersion)
            {
                error = $"version no es {SupportedVersion}";
                return false;
            }
            if (!root.TryGetProperty("election_id", out var e) || e.ValueKind != JsonValueKind.Number
                || !e.TryGetInt64(out var electionId) || electionId <= 0)
            {
                error = "election_id falta o no es un entero positivo";
                return false;
            }
            if (!TryString(root, "occurred_at", out var occurredRaw)
                || !DateTimeOffset.TryParse(occurredRaw, CultureInfo.InvariantCulture,
                    DateTimeStyles.AssumeUniversal | DateTimeStyles.AdjustToUniversal, out var occurredAt))
            {
                error = "occurred_at falta o no es una fecha";
                return false;
            }

            evt = new VoteRegistered(eventId, type, version, electionId, occurredAt);
            error = "";
            return true;
        }
        catch (JsonException)
        {
            error = "el mensaje no es JSON válido";
            return false;
        }
    }

    private static bool TryString(JsonElement root, string name, out string value)
    {
        value = "";
        if (!root.TryGetProperty(name, out var p) || p.ValueKind != JsonValueKind.String)
        {
            return false;
        }
        value = p.GetString() ?? "";
        return true;
    }
}
