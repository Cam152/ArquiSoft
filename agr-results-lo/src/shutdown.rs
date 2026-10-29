//! Señal de apagado compartida (canal `watch` que pasa a `true`).
use tokio::sync::watch;

/// Espera hasta que se pida el apagado. Devuelve `()` (y no la referencia del
/// canal) para que el futuro siga siendo `Send` dentro de `select!`.
pub async fn stopped(rx: &mut watch::Receiver<bool>) {
    // Solo falla si el emisor se destruyó, que también significa apagado.
    let _ = rx.wait_for(|stopping| *stopping).await;
}
