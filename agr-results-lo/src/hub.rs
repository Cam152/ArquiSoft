//! Canal de difusión por elección: el consumidor publica los conteos nuevos y
//! cada conexión SSE de esa elección los recibe.
use std::collections::HashMap;
use std::sync::Mutex;

use tokio::sync::broadcast;

use crate::model::Counts;

const CHANNEL_CAPACITY: usize = 64;

#[derive(Default)]
pub struct Hub {
    channels: Mutex<HashMap<i64, broadcast::Sender<Counts>>>,
}

impl Hub {
    pub fn new() -> Hub {
        Hub::default()
    }

    /// Se suscribe a los cambios de una elección, creando su canal si hace falta.
    pub fn subscribe(&self, election_id: i64) -> broadcast::Receiver<Counts> {
        let mut channels = self.lock();
        channels
            .entry(election_id)
            .or_insert_with(|| broadcast::channel(CHANNEL_CAPACITY).0)
            .subscribe()
    }

    /// Avisa a los suscriptores de la elección. Si ya nadie escucha, el canal
    /// se elimina para que no crezca con cada elección consultada.
    pub fn publish(&self, counts: Counts) {
        let mut channels = self.lock();
        let election_id = counts.election_id;
        if let Some(sender) = channels.get(&election_id) {
            if sender.receiver_count() == 0 {
                channels.remove(&election_id);
            } else {
                // Solo falla si no hay receptores, y ya se descartó arriba.
                let _ = sender.send(counts);
            }
        }
    }

    #[cfg(test)]
    fn channel_count(&self) -> usize {
        self.lock().len()
    }

    fn lock(&self) -> std::sync::MutexGuard<'_, HashMap<i64, broadcast::Sender<Counts>>> {
        // Un pánico con el candado tomado no deja el mapa inconsistente.
        self.channels.lock().unwrap_or_else(|e| e.into_inner())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    fn counts(election_id: i64, votes: i64) -> Counts {
        Counts::new(election_id, BTreeMap::from([(1, votes)]))
    }

    #[tokio::test]
    async fn delivers_only_to_subscribers_of_that_election() {
        let hub = Hub::new();
        let mut one = hub.subscribe(1);
        let mut two = hub.subscribe(2);
        hub.publish(counts(1, 5));
        assert_eq!(one.recv().await.unwrap().total_votes, 5);
        assert!(two.try_recv().is_err());
    }

    #[tokio::test]
    async fn drops_channel_when_nobody_listens() {
        let hub = Hub::new();
        let rx = hub.subscribe(1);
        assert_eq!(hub.channel_count(), 1);
        drop(rx);
        hub.publish(counts(1, 1));
        assert_eq!(hub.channel_count(), 0);
    }

    #[test]
    fn publishing_without_channel_is_a_noop() {
        let hub = Hub::new();
        hub.publish(counts(9, 1));
        assert_eq!(hub.channel_count(), 0);
    }
}
