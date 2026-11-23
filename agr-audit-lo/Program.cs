// agr-audit-lo: registro inmutable de lo que pasó en las votaciones (README, sección 4.10).
//   - Consumers/    se suscribe a audit.vote-registered en agr-broker-as
//   - Services/     reglas: validar el evento, redondear horas, descartar repetidos y
//                   encadenar cada evento con HMAC-SHA256 (S5, tamper-evident log)
//   - Repositories/ agr-audit-db (MongoDB)
//   - Controllers/  GET /health, /audit/events, /audit/summary/{election_id} y /audit/verify

using System.Text.Json;
using Agora.Audit;
using Agora.Audit.Consumers;
using Agora.Audit.Controllers;
using Agora.Audit.Repositories;
using Agora.Audit.Services;
using Microsoft.AspNetCore.Diagnostics;
using MongoDB.Driver;

var settings = Settings.FromEnvironment();

var builder = WebApplication.CreateBuilder(args);
// En Docker el puerto lo fija ASPNETCORE_HTTP_PORTS (Dockerfile). Fuera de Docker, 8005.
if (string.IsNullOrEmpty(Environment.GetEnvironmentVariable("ASPNETCORE_HTTP_PORTS"))
    && string.IsNullOrEmpty(Environment.GetEnvironmentVariable("ASPNETCORE_URLS")))
{
    builder.WebHost.UseUrls("http://*:8005");
}
builder.Logging.ClearProviders();
builder.Logging.AddSimpleConsole(o =>
{
    o.SingleLine = true;
    o.TimestampFormat = "yyyy-MM-dd HH:mm:ss ";
    o.UseUtcTimestamp = true;
});

builder.Services.AddSingleton(settings);
builder.Services.AddSingleton(TimeProvider.System);
builder.Services.AddSingleton<IMongoClient>(_ => new MongoClient(settings.MongoUri));
builder.Services.AddSingleton(sp => sp.GetRequiredService<IMongoClient>().GetDatabase(settings.MongoDatabase));
builder.Services.AddSingleton(new HashChain(settings.ChainKey));
builder.Services.AddSingleton<IAuditRepository, MongoAuditRepository>();
builder.Services.AddSingleton<AuditService>();
builder.Services.AddHostedService<VoteRegisteredConsumer>();

builder.Services
    .AddControllers()
    .AddJsonOptions(o => o.JsonSerializerOptions.PropertyNamingPolicy = JsonNamingPolicy.SnakeCaseLower);

var app = builder.Build();

// Errores siempre como {"detail": "..."}.
app.UseExceptionHandler(errorApp => errorApp.Run(async ctx =>
{
    var error = ctx.Features.Get<IExceptionHandlerFeature>()?.Error;
    app.Logger.LogError("Error al atender {Path}: {Message}", ctx.Request.Path, error?.Message);
    ctx.Response.StatusCode = StatusCodes.Status500InternalServerError;
    await ctx.Response.WriteAsJsonAsync(new ErrorDto("Error interno del Audit Service"));
}));
app.UseStatusCodePages(async ctx =>
{
    var response = ctx.HttpContext.Response;
    var detail = response.StatusCode switch
    {
        StatusCodes.Status404NotFound => "Ruta no encontrada",
        StatusCodes.Status405MethodNotAllowed => "Método no permitido",
        _ => "Error",
    };
    await response.WriteAsJsonAsync(new ErrorDto(detail));
});

app.MapControllers();

// Índices de MongoDB antes de recibir tráfico (event_id y seq únicos). Espera a la base.
var repository = app.Services.GetRequiredService<IAuditRepository>();
for (var attempt = 1; ; attempt++)
{
    try
    {
        await repository.EnsureIndexesAsync(CancellationToken.None);
        break;
    }
    catch (Exception ex) when (attempt < 30)
    {
        app.Logger.LogWarning("Esperando a MongoDB (intento {Attempt}/30): {Message}", attempt, ex.Message);
        await Task.Delay(TimeSpan.FromSeconds(2));
    }
}

// S5: encadena los eventos de antes de S5 (solo si la cadena está vacía) y carga la cabeza.
var sealedLegacy = await app.Services.GetRequiredService<AuditService>().InitializeAsync(CancellationToken.None);
if (sealedLegacy > 0)
{
    app.Logger.LogInformation("Se encadenaron {Count} eventos registrados antes de S5", sealedLegacy);
}
var head = app.Services.GetRequiredService<AuditService>().KnownHead;
app.Logger.LogInformation("Cadena de auditoría: seq {Seq}, hash {Hash}", head?.Seq ?? 0, head?.Hash ?? HashChain.Genesis);

app.Run();
