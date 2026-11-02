using Agora.Audit.Services;
using RabbitMQ.Client;
using RabbitMQ.Client.Events;

namespace Agora.Audit.Consumers;

/// <summary>
/// Se suscribe a audit.vote-registered (conector c10) y guarda cada evento.
/// Ack manual: un mensaje solo sale de la cola cuando quedó guardado.
///   - Guardado o repetido: ack.
///   - No cumple el contrato: nack sin reencolar (va a audit.dead).
///   - Falla la base: espera un momento y nack con reencolado. Tras 5 entregas
///     RabbitMQ lo manda a audit.dead (x-delivery-limit de la cola).
/// Si se cae la conexión, vuelve a conectar.
/// </summary>
public sealed class VoteRegisteredConsumer : BackgroundService
{
    private readonly Settings _settings;
    private readonly AuditService _audit;
    private readonly ILogger<VoteRegisteredConsumer> _log;

    public VoteRegisteredConsumer(Settings settings, AuditService audit, ILogger<VoteRegisteredConsumer> log)
    {
        _settings = settings;
        _audit = audit;
        _log = log;
    }

    protected override async Task ExecuteAsync(CancellationToken stoppingToken)
    {
        var factory = new ConnectionFactory
        {
            Uri = new Uri(_settings.AmqpUrl),
            ClientProvidedName = "agr-audit-lo",
            // La reconexión la maneja este ciclo, así se vuelve a revisar la cola.
            AutomaticRecoveryEnabled = false,
        };

        while (!stoppingToken.IsCancellationRequested)
        {
            try
            {
                await ConsumeUntilDisconnectedAsync(factory, stoppingToken);
            }
            catch (OperationCanceledException) when (stoppingToken.IsCancellationRequested)
            {
                break;
            }
            catch (Exception ex)
            {
                _log.LogWarning("Sin conexión con RabbitMQ ({Message}); reintento en 5 s", ex.Message);
            }

            try
            {
                await Task.Delay(TimeSpan.FromSeconds(5), stoppingToken);
            }
            catch (OperationCanceledException)
            {
                break;
            }
        }
    }

    private async Task ConsumeUntilDisconnectedAsync(ConnectionFactory factory, CancellationToken ct)
    {
        await using var connection = await factory.CreateConnectionAsync(ct);
        await using var channel = await connection.CreateChannelAsync(cancellationToken: ct);

        // La topología la declara agr-broker-as. Declararla aquí con otros argumentos
        // daría PRECONDITION_FAILED, así que solo se comprueba que exista.
        await channel.QueueDeclarePassiveAsync(_settings.Queue, ct);
        await channel.BasicQosAsync(0, _settings.Prefetch, false, ct);

        var closed = new TaskCompletionSource(TaskCreationOptions.RunContinuationsAsynchronously);
        connection.ConnectionShutdownAsync += (_, args) =>
        {
            closed.TrySetResult();
            return Task.CompletedTask;
        };
        channel.ChannelShutdownAsync += (_, args) =>
        {
            closed.TrySetResult();
            return Task.CompletedTask;
        };

        var consumer = new AsyncEventingBasicConsumer(channel);
        consumer.ReceivedAsync += (_, delivery) => HandleAsync(channel, delivery, ct);
        await channel.BasicConsumeAsync(_settings.Queue, autoAck: false, consumer: consumer, cancellationToken: ct);

        _log.LogInformation("Escuchando la cola {Queue}", _settings.Queue);

        await using (ct.Register(() => closed.TrySetCanceled(ct)))
        {
            await closed.Task;
        }
        _log.LogWarning("Se cerró la conexión con RabbitMQ");
    }

    private async Task HandleAsync(IChannel channel, BasicDeliverEventArgs delivery, CancellationToken ct)
    {
        RecordResult result;
        try
        {
            result = await _audit.RecordAsync(delivery.Body, ct);
        }
        catch (OperationCanceledException) when (ct.IsCancellationRequested)
        {
            // Se está apagando: sin ack, RabbitMQ lo reentrega al volver.
            return;
        }
        catch (Exception ex)
        {
            _log.LogError("No se pudo guardar el evento ({Message}); vuelve a la cola", ex.Message);
            try
            {
                await Task.Delay(_settings.RetryDelay, ct);
                await channel.BasicNackAsync(delivery.DeliveryTag, multiple: false, requeue: true, ct);
            }
            catch (Exception)
            {
                // Si el canal se cerró, RabbitMQ reentrega el mensaje solo.
            }
            return;
        }

        switch (result.Outcome)
        {
            case RecordOutcome.Invalid:
                _log.LogWarning("Evento descartado a audit.dead: {Error}", result.Error);
                await channel.BasicNackAsync(delivery.DeliveryTag, multiple: false, requeue: false, ct);
                break;
            case RecordOutcome.Duplicate:
                _log.LogInformation("Evento {EventId} repetido; ya estaba registrado", result.EventId);
                await channel.BasicAckAsync(delivery.DeliveryTag, multiple: false, ct);
                break;
            default:
                _log.LogInformation("Evento {EventId} registrado", result.EventId);
                await channel.BasicAckAsync(delivery.DeliveryTag, multiple: false, ct);
                break;
        }
    }
}
