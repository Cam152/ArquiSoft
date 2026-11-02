//! Capa HTTP: traduce las peticiones a llamadas del servicio y devuelve los
//! errores con el formato común de AGORA: `{"detail": "mensaje"}`.
use std::sync::Arc;
use std::time::Duration;

use axum::extract::{Path, State};
use axum::http::{header, StatusCode};
use axum::response::sse::{KeepAlive, Sse};
use axum::response::{IntoResponse, Response};
use axum::routing::get;
use axum::{Json, Router};
use futures_util::StreamExt;
use serde_json::json;
use tokio::sync::watch;

use crate::service::ResultsService;
use crate::shutdown::stopped;
use crate::sse::counts_stream;

pub struct AppState {
    pub service: Arc<ResultsService>,
    pub keepalive: Duration,
    /// Pasa a `true` al apagar el servicio; cierra los streams SSE abiertos.
    pub shutdown: watch::Receiver<bool>,
}

pub fn router(state: Arc<AppState>) -> Router {
    Router::new()
        .route("/health", get(health))
        .route("/results/:election_id", get(results))
        .route("/results/:election_id/stream", get(stream))
        .fallback(not_found)
        .with_state(state)
}

fn error(status: StatusCode, detail: &str) -> Response {
    (status, Json(json!({ "detail": detail }))).into_response()
}

fn storage_unavailable(e: crate::error::StorageError) -> Response {
    tracing::error!(error = %e, "agr-results-db no responde");
    error(
        StatusCode::SERVICE_UNAVAILABLE,
        "Los resultados no están disponibles",
    )
}

fn invalid_election_id() -> Response {
    error(
        StatusCode::UNPROCESSABLE_ENTITY,
        "election_id debe ser un entero",
    )
}

async fn not_found() -> Response {
    error(StatusCode::NOT_FOUND, "Ruta no encontrada")
}

async fn health(State(state): State<Arc<AppState>>) -> Response {
    match state.service.ping().await {
        Ok(()) => Json(json!({ "status": "ok" })).into_response(),
        Err(e) => storage_unavailable(e),
    }
}

async fn results(State(state): State<Arc<AppState>>, Path(raw_id): Path<String>) -> Response {
    let Ok(election_id) = raw_id.parse::<i64>() else {
        return invalid_election_id();
    };
    match state.service.results(election_id).await {
        Ok(counts) => Json(counts).into_response(),
        Err(e) => storage_unavailable(e),
    }
}

async fn stream(State(state): State<Arc<AppState>>, Path(raw_id): Path<String>) -> Response {
    let Ok(election_id) = raw_id.parse::<i64>() else {
        return invalid_election_id();
    };
    let live = match state.service.subscribe(election_id).await {
        Ok(live) => live,
        Err(e) => return storage_unavailable(e),
    };

    let mut shutdown = state.shutdown.clone();
    let closed = async move { stopped(&mut shutdown).await };
    let events = counts_stream(live, state.service.clone()).take_until(closed);
    let sse = Sse::new(events).keep_alive(
        KeepAlive::new()
            .interval(state.keepalive)
            .text("keep-alive"),
    );
    // Evita que un proxy (nginx) acumule el stream antes de entregarlo.
    (
        [(header::HeaderName::from_static("x-accel-buffering"), "no")],
        sse,
    )
        .into_response()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::VoteRegistered;
    use crate::service::testing::MemoryRepository;
    use axum::body::Body;
    use axum::http::Request;
    use http_body_util::BodyExt;
    use std::sync::atomic::Ordering;
    use tower::ServiceExt;

    struct Fixture {
        repo: Arc<MemoryRepository>,
        service: Arc<ResultsService>,
        stop: watch::Sender<bool>,
        app: Router,
    }

    fn fixture() -> Fixture {
        let repo = Arc::new(MemoryRepository::default());
        let service = Arc::new(ResultsService::new(repo.clone()));
        let (stop, shutdown) = watch::channel(false);
        let app = router(Arc::new(AppState {
            service: service.clone(),
            keepalive: Duration::from_secs(15),
            shutdown,
        }));
        Fixture {
            repo,
            service,
            stop,
            app,
        }
    }

    async fn vote(f: &Fixture, id: &str, election: i64, candidate: i64) {
        let event = VoteRegistered {
            event_id: id.into(),
            kind: "VoteRegistered".into(),
            version: 1,
            election_id: election,
            candidate_id: candidate,
        };
        f.service.process_event(&event).await.unwrap();
    }

    async fn get_json(app: &Router, path: &str) -> (StatusCode, serde_json::Value) {
        let response = app
            .clone()
            .oneshot(Request::get(path).body(Body::empty()).unwrap())
            .await
            .unwrap();
        let status = response.status();
        let bytes = response.into_body().collect().await.unwrap().to_bytes();
        (status, serde_json::from_slice(&bytes).unwrap())
    }

    #[tokio::test]
    async fn health_ok_and_unavailable() {
        let f = fixture();
        let (status, body) = get_json(&f.app, "/health").await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(body, json!({"status": "ok"}));

        f.repo.fail.store(true, Ordering::SeqCst);
        let (status, body) = get_json(&f.app, "/health").await;
        assert_eq!(status, StatusCode::SERVICE_UNAVAILABLE);
        assert!(body["detail"].is_string());
    }

    #[tokio::test]
    async fn results_follow_the_contract() {
        let f = fixture();
        vote(&f, "a", 1, 2).await;
        vote(&f, "b", 1, 2).await;
        vote(&f, "c", 1, 1).await;
        let (status, body) = get_json(&f.app, "/results/1").await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(
            body,
            json!({"election_id": 1, "total_votes": 3, "counts": {"1": 1, "2": 2}})
        );
    }

    #[tokio::test]
    async fn election_without_votes_returns_empty_counts() {
        let f = fixture();
        let (status, body) = get_json(&f.app, "/results/3").await;
        assert_eq!(status, StatusCode::OK);
        assert_eq!(
            body,
            json!({"election_id": 3, "total_votes": 0, "counts": {}})
        );
    }

    #[tokio::test]
    async fn invalid_election_id_is_422_with_detail() {
        let f = fixture();
        for path in ["/results/abc", "/results/abc/stream", "/results/1.5"] {
            let (status, body) = get_json(&f.app, path).await;
            assert_eq!(status, StatusCode::UNPROCESSABLE_ENTITY, "{path}");
            assert_eq!(body["detail"], "election_id debe ser un entero");
        }
    }

    #[tokio::test]
    async fn storage_down_is_503_and_unknown_route_is_404() {
        let f = fixture();
        f.repo.fail.store(true, Ordering::SeqCst);
        for path in ["/results/1", "/results/1/stream"] {
            let (status, _) = get_json(&f.app, path).await;
            assert_eq!(status, StatusCode::SERVICE_UNAVAILABLE, "{path}");
        }
        let (status, body) = get_json(&f.app, "/nada").await;
        assert_eq!(status, StatusCode::NOT_FOUND);
        assert!(body["detail"].is_string());
    }

    /// Lee del cuerpo SSE hasta tener `n` bloques completos.
    async fn read_events(body: &mut Body, n: usize) -> Vec<String> {
        let mut text = String::new();
        while text.matches("\n\n").count() < n {
            let frame = tokio::time::timeout(Duration::from_secs(5), body.frame())
                .await
                .expect("timeout leyendo SSE")
                .expect("el stream terminó")
                .unwrap();
            if let Ok(data) = frame.into_data() {
                text.push_str(std::str::from_utf8(&data).unwrap());
            }
        }
        text.split("\n\n")
            .filter(|s| !s.is_empty())
            .map(String::from)
            .collect()
    }

    #[tokio::test]
    async fn stream_sends_current_counts_then_each_new_vote() {
        let f = fixture();
        vote(&f, "a", 1, 2).await;
        let response = f
            .app
            .clone()
            .oneshot(
                Request::get("/results/1/stream")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        assert_eq!(response.status(), StatusCode::OK);
        assert_eq!(
            response.headers()[header::CONTENT_TYPE],
            "text/event-stream"
        );
        let mut body = response.into_body();

        let first = read_events(&mut body, 1).await;
        assert_eq!(
            first[0],
            "event: counts\ndata: {\"election_id\":1,\"total_votes\":1,\"counts\":{\"2\":1}}"
        );

        vote(&f, "a", 1, 2).await; // repetido: no genera evento
        vote(&f, "b", 2, 1).await; // otra elección: no genera evento
        vote(&f, "c", 1, 1).await;
        let next = read_events(&mut body, 1).await;
        assert_eq!(
            next[0],
            "event: counts\ndata: {\"election_id\":1,\"total_votes\":2,\"counts\":{\"1\":1,\"2\":1}}"
        );
    }

    #[tokio::test]
    async fn stream_ends_on_shutdown() {
        let f = fixture();
        let response = f
            .app
            .clone()
            .oneshot(
                Request::get("/results/1/stream")
                    .body(Body::empty())
                    .unwrap(),
            )
            .await
            .unwrap();
        let mut body = response.into_body();
        read_events(&mut body, 1).await;
        f.stop.send(true).unwrap();
        let end = tokio::time::timeout(Duration::from_secs(5), body.frame()).await;
        assert!(end.expect("el stream no se cerró").is_none());
    }
}
