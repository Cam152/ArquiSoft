//! Consumidor de `VoteRegistered`: único lugar de este servicio que habla con
//! el broker (`agr-broker-as`). Abre la conexión y se suscribe a su cola
//! (conector c9); la topología la declara `agr-broker-as/definitions.json`.
use std::sync::Arc;
use std::time::Duration;

use futures_util::StreamExt;
use lapin::options::{BasicAckOptions, BasicConsumeOptions, BasicNackOptions, BasicQosOptions};
use lapin::types::FieldTable;
use lapin::{Connection, ConnectionProperties};
use tokio::sync::watch;

use crate::model::VoteRegistered;
use crate::service::{Processed, ResultsService};
use crate::shutdown::stopped;

/// Qué hacer con un mensaje una vez procesado.
#[derive(Debug, PartialEq, Eq)]
pub enum Disposition {
    /// Contado (o repetido): se confirma y sale de la cola.
    Ack,
    /// Mensaje que nunca se podrá contar: va a la cola de mensajes fallidos.
    DeadLetter,
    /// Falla transitoria (Redis): se reintenta sin confirmar el mensaje.
    Retry,
}

/// Decide el destino de un mensaje; separado del I/O para poder probarlo.
pub async fn process_message(service: &ResultsService, body: &[u8]) -> Disposition {
    let event = match VoteRegistered::parse(body) {
        Ok(event) => event,
        Err(e) => {
            tracing::error!(error = %e, "mensaje descartado hacia la cola de fallidos");
            return Disposition::DeadLetter;
        }
    };
    match service.process_event(&event).await {
        Ok(Processed::Counted) => {
            tracing::info!(event_id = %event.event_id, election_id = event.election_id, "voto contado");
            Disposition::Ack
        }
        Ok(Processed::Duplicate) => {
            tracing::info!(event_id = %event.event_id, "evento repetido, no se cuenta");
            Disposition::Ack
        }
        Err(e) => {
            tracing::error!(event_id = %event.event_id, error = %e, "no se pudo contar; se reintentará");
            Disposition::Retry
        }
    }
}

pub struct ConsumerConfig {
    pub amqp_url: String,
    pub queue: String,
    pub prefetch: u16,
}

/// Consume hasta que `shutdown` pase a `true`; si se cae la conexión con el
/// broker, reconecta con espera creciente (los mensajes esperan en la cola).
pub async fn run(
    config: ConsumerConfig,
    service: Arc<ResultsService>,
    mut shutdown: watch::Receiver<bool>,
) {
    let mut wait = Duration::from_secs(1);
    while !*shutdown.borrow() {
        match consume(&config, &service, &mut shutdown).await {
            Ok(()) => break,
            Err(e) => {
                tracing::error!(error = %e, retry_in = ?wait, "consumidor desconectado del broker");
                tokio::select! {
                    _ = tokio::time::sleep(wait) => {}
                    _ = stopped(&mut shutdown) => break,
                }
                wait = (wait * 2).min(Duration::from_secs(15));
            }
        }
    }
    tracing::info!("consumidor detenido");
}

async fn consume(
    config: &ConsumerConfig,
    service: &ResultsService,
    shutdown: &mut watch::Receiver<bool>,
) -> Result<(), lapin::Error> {
    let properties = ConnectionProperties::default()
        .with_executor(tokio_executor_trait::Tokio::current())
        .with_reactor(tokio_reactor_trait::Tokio)
        .with_connection_name("agr-results-lo".into());
    let connection = Connection::connect(&config.amqp_url, properties).await?;
    let channel = connection.create_channel().await?;
    channel
        .basic_qos(config.prefetch, BasicQosOptions::default())
        .await?;
    // Ack manual: el mensaje se borra solo cuando el conteo ya está en Redis.
    let mut deliveries = channel
        .basic_consume(
            &config.queue,
            "agr-results-lo",
            BasicConsumeOptions::default(),
            FieldTable::default(),
        )
        .await?;
    tracing::info!(queue = %config.queue, "suscrito a la cola");

    loop {
        tokio::select! {
            _ = stopped(shutdown) => {
                let _ = connection.close(200, "apagado").await;
                return Ok(());
            }
            next = deliveries.next() => {
                let Some(delivery) = next else {
                    return Err(lapin::Error::InvalidChannelState(lapin::ChannelState::Closed));
                };
                let delivery = delivery?;
                let mut disposition = process_message(service, &delivery.data).await;
                // Con Redis caído el mensaje se queda sin confirmar y se vuelve a
                // intentar aquí mismo. Devolverlo a la cola (nack) gastaría una
                // entrega por intento y, tras `x-delivery-limit`, iría a la cola
                // de fallidos aunque el fallo fuera pasajero.
                let mut wait = Duration::from_secs(1);
                while disposition == Disposition::Retry {
                    tokio::select! {
                        _ = tokio::time::sleep(wait) => {}
                        _ = stopped(shutdown) => {
                            // El mensaje sin confirmar vuelve a la cola al cerrar la conexión.
                            let _ = connection.close(200, "apagado").await;
                            return Ok(());
                        }
                    }
                    wait = (wait * 2).min(Duration::from_secs(5));
                    disposition = process_message(service, &delivery.data).await;
                }
                match disposition {
                    Disposition::Ack => delivery.ack(BasicAckOptions::default()).await?,
                    Disposition::DeadLetter | Disposition::Retry => {
                        delivery
                            .nack(BasicNackOptions { requeue: false, ..Default::default() })
                            .await?
                    }
                }
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::service::testing::MemoryRepository;
    use std::sync::atomic::Ordering;

    const EVENT: &str = r#"{"event_id":"e1","type":"VoteRegistered","version":1,
        "election_id":1,"candidate_id":2,"occurred_at":"2026-11-03T14:00:00Z"}"#;

    #[tokio::test]
    async fn dispositions() {
        let repo = Arc::new(MemoryRepository::default());
        let service = ResultsService::new(repo.clone());

        assert_eq!(
            process_message(&service, EVENT.as_bytes()).await,
            Disposition::Ack
        );
        // Entrega repetida: se confirma sin contar otra vez.
        assert_eq!(
            process_message(&service, EVENT.as_bytes()).await,
            Disposition::Ack
        );
        assert_eq!(service.results(1).await.unwrap().total_votes, 1);

        assert_eq!(
            process_message(&service, b"basura").await,
            Disposition::DeadLetter
        );
        let v2 = EVENT.replace("\"version\":1", "\"version\":2");
        assert_eq!(
            process_message(&service, v2.as_bytes()).await,
            Disposition::DeadLetter
        );

        repo.fail.store(true, Ordering::SeqCst);
        let other = EVENT.replace("e1", "e2");
        assert_eq!(
            process_message(&service, other.as_bytes()).await,
            Disposition::Retry
        );
        repo.fail.store(false, Ordering::SeqCst);
        assert_eq!(
            process_message(&service, other.as_bytes()).await,
            Disposition::Ack
        );
        assert_eq!(service.results(1).await.unwrap().total_votes, 2);
    }
}
