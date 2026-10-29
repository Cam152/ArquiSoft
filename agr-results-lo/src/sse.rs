//! Resultados en vivo: convierte los conteos de una elección en un stream SSE.
use std::convert::Infallible;
use std::sync::Arc;

use axum::response::sse::Event;
use futures_util::stream::{self, Stream};
use tokio::sync::broadcast::error::RecvError;

use crate::model::Counts;
use crate::service::{LiveCounts, ResultsService};

/// Evento `counts` con el JSON del contrato (README, sección 5.1).
pub fn counts_event(counts: &Counts) -> Event {
    Event::default()
        .event("counts")
        .json_data(counts)
        .unwrap_or_else(|_| Event::default().comment("no se pudo serializar"))
}

enum State {
    Initial(Counts),
    Live,
}

/// Primero envía los conteos actuales y luego uno por cada voto contado en la
/// elección. Si el cliente es lento y se pierden avisos, envía los conteos
/// vigentes, que reemplazan por completo a los anteriores.
pub fn counts_stream(
    live: LiveCounts,
    service: Arc<ResultsService>,
) -> impl Stream<Item = Result<Event, Infallible>> {
    let election_id = live.current.election_id;
    stream::unfold(
        (State::Initial(live.current), live.updates, service),
        move |(state, mut updates, service)| async move {
            let counts = match state {
                State::Initial(counts) => counts,
                State::Live => loop {
                    match updates.recv().await {
                        Ok(counts) => break counts,
                        Err(RecvError::Lagged(missed)) => {
                            tracing::debug!(election_id, missed, "cliente SSE lento");
                            match service.results(election_id).await {
                                Ok(counts) => break counts,
                                Err(e) => tracing::warn!(error = %e, "no se pudo refrescar el SSE"),
                            }
                        }
                        Err(RecvError::Closed) => return None,
                    }
                },
            };
            Some((Ok(counts_event(&counts)), (State::Live, updates, service)))
        },
    )
}
