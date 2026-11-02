namespace Agora.Audit;

/// <summary>Configuración del servicio, solo por variables de entorno.</summary>
public sealed record Settings(
    string MongoUri,
    string MongoDatabase,
    string AmqpUrl,
    string Queue,
    ushort Prefetch,
    TimeSpan RetryDelay)
{
    public static Settings FromEnvironment() => new(
        MongoUri: Env("MONGO_URI", "mongodb://localhost:27017"),
        MongoDatabase: Env("MONGO_DB", "audit"),
        AmqpUrl: Env("AMQP_URL", "amqp://agora:agora@localhost:5672/"),
        Queue: Env("AUDIT_QUEUE", "audit.vote-registered"),
        Prefetch: ushort.Parse(Env("AUDIT_PREFETCH", "10")),
        RetryDelay: TimeSpan.FromSeconds(double.Parse(Env("AUDIT_RETRY_DELAY_SECONDS", "2"),
            System.Globalization.CultureInfo.InvariantCulture)));

    private static string Env(string name, string fallback)
    {
        var value = Environment.GetEnvironmentVariable(name);
        return string.IsNullOrWhiteSpace(value) ? fallback : value.Trim();
    }
}
