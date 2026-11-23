namespace Agora.Audit;

/// <summary>Configuración del servicio, solo por variables de entorno.</summary>
public sealed record Settings(
    string MongoUri,
    string MongoDatabase,
    string AmqpUrl,
    string Queue,
    ushort Prefetch,
    TimeSpan RetryDelay,
    string ChainKey)
{
    public static Settings FromEnvironment() => new(
        MongoUri: Env("MONGO_URI", "mongodb://localhost:27017"),
        MongoDatabase: Env("MONGO_DB", "audit"),
        AmqpUrl: Env("AMQP_URL", "amqp://agora:agora@localhost:5672/"),
        Queue: Env("AUDIT_QUEUE", "audit.vote-registered"),
        Prefetch: ushort.Parse(Env("AUDIT_PREFETCH", "10")),
        RetryDelay: TimeSpan.FromSeconds(double.Parse(Env("AUDIT_RETRY_DELAY_SECONDS", "2"),
            System.Globalization.CultureInfo.InvariantCulture)),
        // S5: llave del HMAC de la cadena de hashes. Solo la conoce agr-audit-lo.
        ChainKey: Env("AUDIT_CHAIN_KEY", ""));

    private static string Env(string name, string fallback)
    {
        var value = Environment.GetEnvironmentVariable(name);
        return string.IsNullOrWhiteSpace(value) ? fallback : value.Trim();
    }
}
