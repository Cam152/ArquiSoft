using System.Globalization;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using Agora.Audit.Models;

namespace Agora.Audit.Services;

/// <summary>
/// Cálculo del hash de cada eslabón de la cadena (S5, patrón tamper-evident log).
///
/// hash = HMAC-SHA256(AUDIT_CHAIN_KEY, ["agora-audit-v1", seq, event_id, type,
///                                      election_id, occurred_at, recorded_at, prev_hash])
///
/// Los campos van como arreglo JSON para que no haya ambigüedad entre ellos (un
/// event_id con "|" no puede hacerse pasar por otro campo). La llave solo la conoce
/// agr-audit-lo: quien entre a agr-audit-db puede cambiar un evento, pero no puede
/// recalcular un hash válido para tapar el cambio.
/// </summary>
public sealed class HashChain
{
    /// <summary>prev_hash del primer evento de la cadena.</summary>
    public static readonly string Genesis = new('0', 64);

    public const int MinKeyLength = 32;

    private const string Version = "agora-audit-v1";

    private readonly byte[] _key;

    public HashChain(string key)
    {
        if (string.IsNullOrEmpty(key) || key.Length < MinKeyLength)
        {
            throw new ArgumentException($"AUDIT_CHAIN_KEY debe tener al menos {MinKeyLength} caracteres", nameof(key));
        }
        _key = Encoding.UTF8.GetBytes(key);
    }

    /// <summary>Hash del evento con su seq y el hash del anterior.</summary>
    public string Compute(AuditEvent e, long seq, string prevHash)
    {
        var canonical = JsonSerializer.Serialize(new object[]
        {
            Version,
            seq,
            e.EventId,
            e.Type,
            e.ElectionId,
            Iso(e.OccurredAt),
            Iso(e.RecordedAt),
            prevHash,
        });
        var mac = HMACSHA256.HashData(_key, Encoding.UTF8.GetBytes(canonical));
        return Convert.ToHexStringLower(mac);
    }

    /// <summary>Asigna seq, prev_hash y hash al evento.</summary>
    public void Seal(AuditEvent e, long seq, string prevHash)
    {
        e.Seq = seq;
        e.PrevHash = prevHash;
        e.Hash = Compute(e, seq, prevHash);
    }

    /// <summary>True si el hash guardado coincide con el recalculado (comparación en tiempo constante).</summary>
    public bool Matches(AuditEvent e)
    {
        if (e.Seq is not { } seq || e.PrevHash is null || e.Hash is null)
        {
            return false;
        }
        var expected = Encoding.ASCII.GetBytes(Compute(e, seq, e.PrevHash));
        var actual = Encoding.ASCII.GetBytes(e.Hash);
        return CryptographicOperations.FixedTimeEquals(expected, actual);
    }

    // Milisegundos: es la precisión con la que MongoDB guarda las fechas, así el
    // hash recalculado al leer coincide con el calculado al escribir.
    private static string Iso(DateTime value) =>
        DateTime.SpecifyKind(value, DateTimeKind.Utc).ToString("yyyy-MM-dd'T'HH:mm:ss.fff'Z'", CultureInfo.InvariantCulture);
}
