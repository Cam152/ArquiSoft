//! Único lugar que habla con `agr-results-db` (Redis).
//!
//! Claves:
//! - `results:{election_id}`: hash candidato → votos.
//! - `results:{election_id}:seen`: conjunto de `event_id` ya contados.
use std::collections::BTreeMap;
use std::time::Duration;

use async_trait::async_trait;
use redis::aio::{ConnectionManager, ConnectionManagerConfig};
use redis::Script;

use crate::error::StorageError;
use crate::model::Counts;

/// Tiempo máximo de cada operación contra Redis. Sin este tope, con Redis caído
/// las peticiones HTTP (y el healthcheck) se quedarían esperando la reconexión.
const OPERATION_TIMEOUT: Duration = Duration::from_secs(2);

/// Script atómico: solo suma si el evento es nuevo. Un evento que RabbitMQ
/// entrega dos veces (o que se reprocesa tras un fallo antes del ack) no se
/// cuenta dos veces.
const COUNT_ONCE: &str = r#"
if redis.call('SADD', KEYS[2], ARGV[1]) == 1 then
  redis.call('HINCRBY', KEYS[1], ARGV[2], 1)
  return 1
end
return 0
"#;

#[async_trait]
pub trait ResultsRepository: Send + Sync {
    /// Cuenta el voto si `event_id` no se había contado. Devuelve `true` si
    /// el evento era nuevo y `false` si era un repetido.
    async fn count_vote(
        &self,
        election_id: i64,
        candidate_id: i64,
        event_id: &str,
    ) -> Result<bool, StorageError>;

    /// Conteos actuales de la elección (vacíos si nadie ha votado).
    async fn counts(&self, election_id: i64) -> Result<Counts, StorageError>;

    /// Comprueba que la base responde.
    async fn ping(&self) -> Result<(), StorageError>;
}

pub fn counts_key(election_id: i64) -> String {
    format!("results:{election_id}")
}

pub fn seen_key(election_id: i64) -> String {
    format!("results:{election_id}:seen")
}

pub struct RedisRepository {
    conn: ConnectionManager,
    count_once: Script,
}

impl RedisRepository {
    /// Conecta con reintentos: Redis puede tardar unos segundos en aceptar
    /// conexiones aunque el contenedor ya esté arriba.
    pub async fn connect(url: &str, attempts: u32) -> Result<RedisRepository, StorageError> {
        let client = redis::Client::open(url)?;
        let config = ConnectionManagerConfig::new()
            .set_connection_timeout(OPERATION_TIMEOUT)
            .set_response_timeout(OPERATION_TIMEOUT)
            .set_number_of_retries(1);
        let mut last_error = None;
        for attempt in 1..=attempts.max(1) {
            match ConnectionManager::new_with_config(client.clone(), config.clone()).await {
                Ok(conn) => {
                    return Ok(RedisRepository {
                        conn,
                        count_once: Script::new(COUNT_ONCE),
                    })
                }
                Err(e) => {
                    tracing::warn!(attempt, error = %e, "Redis aún no responde");
                    last_error = Some(e);
                    tokio::time::sleep(Duration::from_secs(2)).await;
                }
            }
        }
        Err(StorageError(
            last_error.map(|e| e.to_string()).unwrap_or_default(),
        ))
    }
}

/// Aplica `OPERATION_TIMEOUT` a una operación de Redis.
async fn bounded<T>(
    operation: impl std::future::Future<Output = redis::RedisResult<T>>,
) -> Result<T, StorageError> {
    match tokio::time::timeout(OPERATION_TIMEOUT, operation).await {
        Ok(result) => Ok(result?),
        Err(_) => Err(StorageError("Redis no respondió a tiempo".into())),
    }
}

#[async_trait]
impl ResultsRepository for RedisRepository {
    async fn count_vote(
        &self,
        election_id: i64,
        candidate_id: i64,
        event_id: &str,
    ) -> Result<bool, StorageError> {
        let mut conn = self.conn.clone();
        let counted: i64 = bounded(
            self.count_once
                .key(counts_key(election_id))
                .key(seen_key(election_id))
                .arg(event_id)
                .arg(candidate_id)
                .invoke_async(&mut conn),
        )
        .await?;
        Ok(counted == 1)
    }

    async fn counts(&self, election_id: i64) -> Result<Counts, StorageError> {
        let mut conn = self.conn.clone();
        let raw: Vec<(String, i64)> = bounded(
            redis::cmd("HGETALL")
                .arg(counts_key(election_id))
                .query_async(&mut conn),
        )
        .await?;
        let mut counts = BTreeMap::new();
        for (candidate, votes) in raw {
            let candidate_id = candidate.parse::<i64>().map_err(|_| {
                StorageError(format!("campo de conteo inválido en Redis: {candidate:?}"))
            })?;
            counts.insert(candidate_id, votes);
        }
        Ok(Counts::new(election_id, counts))
    }

    async fn ping(&self) -> Result<(), StorageError> {
        let mut conn = self.conn.clone();
        bounded(redis::cmd("PING").query_async::<String>(&mut conn)).await?;
        Ok(())
    }
}
