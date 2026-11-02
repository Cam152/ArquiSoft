//! Pruebas contra un Redis y un RabbitMQ reales. No corren con `cargo test`
//! normal; se ejecutan con:
//!
//!   TEST_REDIS_URL=redis://localhost:6379 \
//!   TEST_AMQP_URL=amqp://guest:guest@localhost:5672/%2f \
//!   cargo test --test integration -- --ignored --test-threads=1
//!
//! Cada prueba usa ids de elección únicos, así que no se pisan entre sí ni
//! con datos existentes.
use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use axum::body::Body;
use axum::http::{Request, StatusCode};
use http_body_util::BodyExt;
use lapin::options::{
    BasicGetOptions, BasicPublishOptions, ConfirmSelectOptions, ExchangeDeclareOptions,
    QueueBindOptions, QueueDeclareOptions, QueueDeleteOptions,
};
use lapin::types::{AMQPValue, FieldTable};
use lapin::{BasicProperties, Connection, ConnectionProperties, ExchangeKind};
use results_service::consumer::{self, ConsumerConfig};
use results_service::handler::{self, AppState};
use results_service::repository::{counts_key, seen_key, RedisRepository, ResultsRepository};
use results_service::service::ResultsService;
use serde_json::{json, Value};
use tokio::sync::watch;
use tower::ServiceExt;

fn env(name: &str) -> String {
    std::env::var(name).unwrap_or_else(|_| panic!("falta la variable {name}"))
}

/// Id de elección único por ejecución y por prueba.
fn unique_election_id(offset: i64) -> i64 {
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos() as i64;
    9_000_000_000 + (nanos % 1_000_000_000) * 10 + offset
}

async fn cleanup(election_id: i64) {
    let client = redis::Client::open(env("TEST_REDIS_URL")).unwrap();
    let mut conn = client.get_multiplexed_async_connection().await.unwrap();
    let _: i64 = redis::cmd("DEL")
        .arg(counts_key(election_id))
        .arg(seen_key(election_id))
        .query_async(&mut conn)
        .await
        .unwrap();
}

#[tokio::test]
#[ignore = "requiere Redis (TEST_REDIS_URL)"]
async fn redis_counts_each_event_once_even_under_concurrency() {
    let election = unique_election_id(1);
    let repo = Arc::new(
        RedisRepository::connect(&env("TEST_REDIS_URL"), 3)
            .await
            .unwrap(),
    );

    // 50 entregas simultáneas del mismo evento: solo una debe contarse.
    let tasks: Vec<_> = (0..50)
        .map(|_| {
            let repo = repo.clone();
            tokio::spawn(async move {
                repo.count_vote(election, 7, "evento-concurrente")
                    .await
                    .unwrap()
            })
        })
        .collect();
    let mut counted = 0;
    for task in tasks {
        if task.await.unwrap() {
            counted += 1;
        }
    }
    assert_eq!(counted, 1);

    assert!(repo.count_vote(election, 7, "otro-evento").await.unwrap());
    assert!(repo.count_vote(election, 8, "tercer-evento").await.unwrap());
    let counts = repo.counts(election).await.unwrap();
    assert_eq!(counts.total_votes, 3);
    assert_eq!(counts.counts[&7], 2);
    assert_eq!(counts.counts[&8], 1);

    assert_eq!(repo.counts(election + 1).await.unwrap().total_votes, 0);
    repo.ping().await.unwrap();
    cleanup(election).await;
}

async fn get_json(app: &axum::Router, path: &str) -> (StatusCode, Value) {
    let response = app
        .clone()
        .oneshot(Request::get(path).body(Body::empty()).unwrap())
        .await
        .unwrap();
    let status = response.status();
    let bytes = response.into_body().collect().await.unwrap().to_bytes();
    (
        status,
        serde_json::from_slice(&bytes).unwrap_or(Value::Null),
    )
}

async fn wait_for_total(app: &axum::Router, election: i64, expected: i64) -> Value {
    for _ in 0..100 {
        let (_, body) = get_json(app, &format!("/results/{election}")).await;
        if body["total_votes"] == expected {
            return body;
        }
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
    panic!("la elección {election} no llegó a {expected} votos");
}

fn event(id: &str, election: i64, candidate: i64) -> Vec<u8> {
    json!({"event_id": id, "type": "VoteRegistered", "version": 1,
           "election_id": election, "candidate_id": candidate,
           "occurred_at": "2026-11-03T14:00:00Z"})
    .to_string()
    .into_bytes()
}

#[tokio::test]
#[ignore = "requiere Redis y RabbitMQ (TEST_REDIS_URL, TEST_AMQP_URL)"]
async fn events_from_the_broker_are_counted_once_and_bad_ones_dead_lettered() {
    let election = unique_election_id(2);
    let suffix = election.to_string();
    let amqp_url = env("TEST_AMQP_URL");

    // Topología equivalente a agr-broker-as/definitions.json, con nombres únicos.
    let exchange = format!("test.votes.{suffix}");
    let dlx = format!("test.dlx.{suffix}");
    let queue = format!("test.results.{suffix}");
    let dead = format!("test.results.dead.{suffix}");
    let admin = Connection::connect(&amqp_url, ConnectionProperties::default())
        .await
        .unwrap();
    let ch = admin.create_channel().await.unwrap();
    ch.confirm_select(ConfirmSelectOptions::default())
        .await
        .unwrap();
    for (name, kind) in [
        (&exchange, ExchangeKind::Fanout),
        (&dlx, ExchangeKind::Fanout),
    ] {
        ch.exchange_declare(
            name,
            kind,
            ExchangeDeclareOptions {
                durable: true,
                ..Default::default()
            },
            FieldTable::default(),
        )
        .await
        .unwrap();
    }
    let durable = QueueDeclareOptions {
        durable: true,
        ..Default::default()
    };
    ch.queue_declare(&dead, durable, FieldTable::default())
        .await
        .unwrap();
    ch.queue_bind(
        &dead,
        &dlx,
        "",
        QueueBindOptions::default(),
        FieldTable::default(),
    )
    .await
    .unwrap();
    let mut args = FieldTable::default();
    args.insert(
        "x-queue-type".into(),
        AMQPValue::LongString("quorum".into()),
    );
    args.insert("x-delivery-limit".into(), AMQPValue::LongInt(5));
    args.insert(
        "x-dead-letter-exchange".into(),
        AMQPValue::LongString(dlx.clone().into()),
    );
    ch.queue_declare(&queue, durable, args).await.unwrap();
    ch.queue_bind(
        &queue,
        &exchange,
        "",
        QueueBindOptions::default(),
        FieldTable::default(),
    )
    .await
    .unwrap();

    // El servicio completo: Redis real + consumidor + HTTP.
    let repo = RedisRepository::connect(&env("TEST_REDIS_URL"), 3)
        .await
        .unwrap();
    let service = Arc::new(ResultsService::new(Arc::new(repo)));
    let (stop, shutdown) = watch::channel(false);
    let consumer_task = tokio::spawn(consumer::run(
        ConsumerConfig {
            amqp_url: amqp_url.clone(),
            queue: queue.clone(),
            prefetch: 10,
        },
        service.clone(),
        shutdown.clone(),
    ));
    let app = handler::router(Arc::new(AppState {
        service,
        keepalive: Duration::from_secs(15),
        shutdown,
    }));

    let publish = |body: Vec<u8>| {
        let ch = ch.clone();
        let exchange = exchange.clone();
        async move {
            ch.basic_publish(
                &exchange,
                "",
                BasicPublishOptions::default(),
                &body,
                BasicProperties::default().with_delivery_mode(2),
            )
            .await
            .unwrap()
            .await
            .unwrap();
        }
    };
    publish(event("ev-1", election, 1)).await;
    publish(event("ev-2", election, 2)).await;
    publish(event("ev-3", election, 2)).await;
    publish(event("ev-3", election, 2)).await; // entrega repetida
    publish(b"esto no es json".to_vec()).await; // venenoso
    publish(event("ev-4", election, 1)).await;

    let body = wait_for_total(&app, election, 4).await;
    assert_eq!(body["counts"], json!({"1": 2, "2": 2}));
    // El repetido no se cuenta aunque se espere un rato más.
    tokio::time::sleep(Duration::from_millis(500)).await;
    assert_eq!(
        get_json(&app, &format!("/results/{election}")).await.1["total_votes"],
        4
    );

    // El mensaje inválido terminó en la cola de fallidos, no se perdió ni se reintenta.
    let mut found = None;
    for _ in 0..50 {
        if let Some(msg) = ch
            .basic_get(&dead, BasicGetOptions { no_ack: true })
            .await
            .unwrap()
        {
            found = Some(msg.delivery.data.clone());
            break;
        }
        tokio::time::sleep(Duration::from_millis(100)).await;
    }
    assert_eq!(found.as_deref(), Some(&b"esto no es json"[..]));

    stop.send(true).unwrap();
    tokio::time::timeout(Duration::from_secs(5), consumer_task)
        .await
        .unwrap()
        .unwrap();
    for q in [&queue, &dead] {
        ch.queue_delete(q, QueueDeleteOptions::default())
            .await
            .unwrap();
    }
    cleanup(election).await;
}

#[tokio::test]
#[ignore = "requiere Redis y RabbitMQ (TEST_REDIS_URL, TEST_AMQP_URL)"]
async fn events_published_while_the_service_is_down_are_counted_when_it_returns() {
    let election = unique_election_id(3);
    let suffix = election.to_string();
    let amqp_url = env("TEST_AMQP_URL");
    let queue = format!("test.results.offline.{suffix}");

    let admin = Connection::connect(&amqp_url, ConnectionProperties::default())
        .await
        .unwrap();
    let ch = admin.create_channel().await.unwrap();
    ch.confirm_select(ConfirmSelectOptions::default())
        .await
        .unwrap();
    ch.queue_declare(
        &queue,
        QueueDeclareOptions {
            durable: true,
            ..Default::default()
        },
        FieldTable::default(),
    )
    .await
    .unwrap();
    // Con el servicio apagado los eventos esperan en la cola.
    for i in 0..3 {
        ch.basic_publish(
            "",
            &queue,
            BasicPublishOptions::default(),
            &event(&format!("off-{i}"), election, 5),
            BasicProperties::default().with_delivery_mode(2),
        )
        .await
        .unwrap()
        .await
        .unwrap();
    }

    let repo = RedisRepository::connect(&env("TEST_REDIS_URL"), 3)
        .await
        .unwrap();
    let service = Arc::new(ResultsService::new(Arc::new(repo)));
    let (stop, shutdown) = watch::channel(false);
    let consumer_task = tokio::spawn(consumer::run(
        ConsumerConfig {
            amqp_url,
            queue: queue.clone(),
            prefetch: 10,
        },
        service.clone(),
        shutdown.clone(),
    ));
    let app = handler::router(Arc::new(AppState {
        service,
        keepalive: Duration::from_secs(15),
        shutdown,
    }));

    let body = wait_for_total(&app, election, 3).await;
    assert_eq!(body["counts"], json!({"5": 3}));

    stop.send(true).unwrap();
    tokio::time::timeout(Duration::from_secs(5), consumer_task)
        .await
        .unwrap()
        .unwrap();
    ch.queue_delete(&queue, QueueDeleteOptions::default())
        .await
        .unwrap();
    cleanup(election).await;
}

/// Redis real, pero que se puede "apagar" para simular una caída pasajera.
struct FlakyRepository {
    inner: RedisRepository,
    down: std::sync::atomic::AtomicBool,
}

impl FlakyRepository {
    fn check(&self) -> Result<(), results_service::error::StorageError> {
        if self.down.load(std::sync::atomic::Ordering::SeqCst) {
            Err(results_service::error::StorageError(
                "caída simulada".into(),
            ))
        } else {
            Ok(())
        }
    }
}

#[async_trait::async_trait]
impl ResultsRepository for FlakyRepository {
    async fn count_vote(
        &self,
        election_id: i64,
        candidate_id: i64,
        event_id: &str,
    ) -> Result<bool, results_service::error::StorageError> {
        self.check()?;
        self.inner
            .count_vote(election_id, candidate_id, event_id)
            .await
    }

    async fn counts(
        &self,
        election_id: i64,
    ) -> Result<results_service::model::Counts, results_service::error::StorageError> {
        self.check()?;
        self.inner.counts(election_id).await
    }

    async fn ping(&self) -> Result<(), results_service::error::StorageError> {
        self.check()?;
        self.inner.ping().await
    }
}

/// Regresión: una caída pasajera de Redis no debe gastar las entregas de la
/// cola (`x-delivery-limit`) ni mandar votos a la cola de fallidos.
#[tokio::test]
#[ignore = "requiere Redis y RabbitMQ (TEST_REDIS_URL, TEST_AMQP_URL)"]
async fn transient_storage_failure_does_not_dead_letter_the_vote() {
    let election = unique_election_id(4);
    let suffix = election.to_string();
    let amqp_url = env("TEST_AMQP_URL");
    let dlx = format!("test.dlx.flaky.{suffix}");
    let queue = format!("test.results.flaky.{suffix}");
    let dead = format!("test.results.dead.flaky.{suffix}");

    let admin = Connection::connect(&amqp_url, ConnectionProperties::default())
        .await
        .unwrap();
    let ch = admin.create_channel().await.unwrap();
    ch.confirm_select(ConfirmSelectOptions::default())
        .await
        .unwrap();
    let durable = QueueDeclareOptions {
        durable: true,
        ..Default::default()
    };
    ch.exchange_declare(
        &dlx,
        ExchangeKind::Fanout,
        ExchangeDeclareOptions {
            durable: true,
            ..Default::default()
        },
        FieldTable::default(),
    )
    .await
    .unwrap();
    ch.queue_declare(&dead, durable, FieldTable::default())
        .await
        .unwrap();
    ch.queue_bind(
        &dead,
        &dlx,
        "",
        QueueBindOptions::default(),
        FieldTable::default(),
    )
    .await
    .unwrap();
    let mut args = FieldTable::default();
    args.insert(
        "x-queue-type".into(),
        AMQPValue::LongString("quorum".into()),
    );
    args.insert("x-delivery-limit".into(), AMQPValue::LongInt(2)); // límite muy bajo a propósito
    args.insert(
        "x-dead-letter-exchange".into(),
        AMQPValue::LongString(dlx.clone().into()),
    );
    ch.queue_declare(&queue, durable, args).await.unwrap();

    let repo = Arc::new(FlakyRepository {
        inner: RedisRepository::connect(&env("TEST_REDIS_URL"), 3)
            .await
            .unwrap(),
        down: std::sync::atomic::AtomicBool::new(true),
    });
    let service = Arc::new(ResultsService::new(repo.clone()));
    let (stop, shutdown) = watch::channel(false);
    let consumer_task = tokio::spawn(consumer::run(
        ConsumerConfig {
            amqp_url,
            queue: queue.clone(),
            prefetch: 10,
        },
        service.clone(),
        shutdown.clone(),
    ));

    ch.basic_publish(
        "",
        &queue,
        BasicPublishOptions::default(),
        &event("flaky-1", election, 3),
        BasicProperties::default().with_delivery_mode(2),
    )
    .await
    .unwrap()
    .await
    .unwrap();

    // 5 s de caída: varios reintentos, más de los que permite x-delivery-limit.
    tokio::time::sleep(Duration::from_secs(5)).await;
    assert!(
        ch.basic_get(&dead, BasicGetOptions { no_ack: true })
            .await
            .unwrap()
            .is_none(),
        "el voto no debe ir a la cola de fallidos por una caída pasajera"
    );
    repo.down.store(false, std::sync::atomic::Ordering::SeqCst);

    let app = handler::router(Arc::new(AppState {
        service,
        keepalive: Duration::from_secs(15),
        shutdown,
    }));
    let body = wait_for_total(&app, election, 1).await;
    assert_eq!(body["counts"], json!({"3": 1}));
    assert!(ch
        .basic_get(&dead, BasicGetOptions { no_ack: true })
        .await
        .unwrap()
        .is_none());

    stop.send(true).unwrap();
    tokio::time::timeout(Duration::from_secs(5), consumer_task)
        .await
        .unwrap()
        .unwrap();
    for q in [&queue, &dead] {
        ch.queue_delete(q, QueueDeleteOptions::default())
            .await
            .unwrap();
    }
    cleanup(election).await;
}
