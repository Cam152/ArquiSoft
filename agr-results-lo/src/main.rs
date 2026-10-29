//! Punto de entrada de agr-results-lo: HTTP (REST y SSE), consumidor AMQP y Redis.
use std::sync::Arc;
use std::time::Duration;

use results_service::config::Config;
use results_service::consumer::{self, ConsumerConfig};
use results_service::handler::{self, AppState};
use results_service::repository::RedisRepository;
use results_service::service::ResultsService;
use tokio::sync::watch;

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt().with_ansi(false).init();

    let config = match Config::from_env() {
        Ok(config) => config,
        Err(e) => {
            tracing::error!("configuración inválida: {e}");
            std::process::exit(1);
        }
    };

    let repository = match RedisRepository::connect(&config.redis_url, 15).await {
        Ok(repository) => repository,
        Err(e) => {
            tracing::error!("no se pudo conectar con agr-results-db: {e}");
            std::process::exit(1);
        }
    };
    let service = Arc::new(ResultsService::new(Arc::new(repository)));

    let (stop, shutdown) = watch::channel(false);
    let consumer_task = tokio::spawn(consumer::run(
        ConsumerConfig {
            amqp_url: config.amqp_url.clone(),
            queue: config.queue.clone(),
            prefetch: config.prefetch,
        },
        service.clone(),
        shutdown.clone(),
    ));

    let app = handler::router(Arc::new(AppState {
        service,
        keepalive: config.keepalive,
        shutdown: shutdown.clone(),
    }));
    let listener = match tokio::net::TcpListener::bind(("0.0.0.0", config.port)).await {
        Ok(listener) => listener,
        Err(e) => {
            tracing::error!("no se pudo abrir el puerto {}: {e}", config.port);
            std::process::exit(1);
        }
    };
    tracing::info!("agr-results-lo escuchando en el puerto {}", config.port);

    let served = axum::serve(listener, app).with_graceful_shutdown(async move {
        wait_for_signal().await;
        tracing::info!("apagando");
        let _ = stop.send(true);
    });
    if let Err(e) = served.await {
        tracing::error!("error del servidor HTTP: {e}");
    }
    let _ = tokio::time::timeout(Duration::from_secs(5), consumer_task).await;
}

async fn wait_for_signal() {
    use tokio::signal::unix::{signal, SignalKind};
    let mut term = signal(SignalKind::terminate()).expect("no se pudo escuchar SIGTERM");
    tokio::select! {
        _ = term.recv() => {}
        _ = tokio::signal::ctrl_c() => {}
    }
}
