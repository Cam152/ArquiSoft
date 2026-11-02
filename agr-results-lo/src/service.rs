//! Reglas de negocio: contar cada evento una sola vez y avisar a quienes
//! siguen los resultados en vivo.
use std::sync::Arc;

use tokio::sync::broadcast;

use crate::error::StorageError;
use crate::hub::Hub;
use crate::model::{Counts, VoteRegistered};
use crate::repository::ResultsRepository;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Processed {
    /// El voto se sumó.
    Counted,
    /// El evento ya se había contado; no se suma de nuevo.
    Duplicate,
}

/// Suscripción a una elección: los conteos de ahora y los cambios futuros.
pub struct LiveCounts {
    pub current: Counts,
    pub updates: broadcast::Receiver<Counts>,
}

pub struct ResultsService {
    repo: Arc<dyn ResultsRepository>,
    hub: Hub,
}

impl ResultsService {
    pub fn new(repo: Arc<dyn ResultsRepository>) -> ResultsService {
        ResultsService {
            repo,
            hub: Hub::new(),
        }
    }

    /// Cuenta un `VoteRegistered`. Si falla el almacenamiento, el error sube
    /// para que el consumidor no confirme el mensaje y el broker lo reentregue.
    pub async fn process_event(&self, event: &VoteRegistered) -> Result<Processed, StorageError> {
        let counted = self
            .repo
            .count_vote(event.election_id, event.candidate_id, &event.event_id)
            .await?;
        if !counted {
            return Ok(Processed::Duplicate);
        }
        // El voto ya está contado: si falla solo la lectura para el SSE no se
        // propaga el error, porque la reentrega sería un duplicado y nunca
        // avisaría. Los clientes reciben el conteo completo en el siguiente voto.
        match self.repo.counts(event.election_id).await {
            Ok(counts) => self.hub.publish(counts),
            Err(e) => tracing::warn!(
                election_id = event.election_id,
                error = %e,
                "voto contado pero no se pudo difundir por SSE"
            ),
        }
        Ok(Processed::Counted)
    }

    pub async fn results(&self, election_id: i64) -> Result<Counts, StorageError> {
        self.repo.counts(election_id).await
    }

    /// Se suscribe antes de leer los conteos actuales: así no se pierde un
    /// voto que llegue entre las dos operaciones.
    pub async fn subscribe(&self, election_id: i64) -> Result<LiveCounts, StorageError> {
        let updates = self.hub.subscribe(election_id);
        let current = self.repo.counts(election_id).await?;
        Ok(LiveCounts { current, updates })
    }

    pub async fn ping(&self) -> Result<(), StorageError> {
        self.repo.ping().await
    }
}

#[cfg(test)]
pub mod testing {
    //! Repositorio en memoria con el mismo comportamiento que el script Lua.
    use super::*;
    use async_trait::async_trait;
    use std::collections::{BTreeMap, HashMap, HashSet};
    use std::sync::Mutex;

    type ElectionState = (BTreeMap<i64, i64>, HashSet<String>);

    #[derive(Default)]
    pub struct MemoryRepository {
        state: Mutex<HashMap<i64, ElectionState>>,
        pub fail: std::sync::atomic::AtomicBool,
    }

    impl MemoryRepository {
        fn check(&self) -> Result<(), StorageError> {
            if self.fail.load(std::sync::atomic::Ordering::SeqCst) {
                Err(StorageError("caído".into()))
            } else {
                Ok(())
            }
        }
    }

    #[async_trait]
    impl ResultsRepository for MemoryRepository {
        async fn count_vote(
            &self,
            election_id: i64,
            candidate_id: i64,
            event_id: &str,
        ) -> Result<bool, StorageError> {
            self.check()?;
            let mut state = self.state.lock().unwrap();
            let (counts, seen) = state.entry(election_id).or_default();
            if !seen.insert(event_id.to_string()) {
                return Ok(false);
            }
            *counts.entry(candidate_id).or_insert(0) += 1;
            Ok(true)
        }

        async fn counts(&self, election_id: i64) -> Result<Counts, StorageError> {
            self.check()?;
            let state = self.state.lock().unwrap();
            let counts = state
                .get(&election_id)
                .map(|(c, _)| c.clone())
                .unwrap_or_default();
            Ok(Counts::new(election_id, counts))
        }

        async fn ping(&self) -> Result<(), StorageError> {
            self.check()
        }
    }
}

#[cfg(test)]
mod tests {
    use super::testing::MemoryRepository;
    use super::*;
    use std::sync::atomic::Ordering;

    fn event(id: &str, election: i64, candidate: i64) -> VoteRegistered {
        VoteRegistered {
            event_id: id.into(),
            kind: "VoteRegistered".into(),
            version: 1,
            election_id: election,
            candidate_id: candidate,
        }
    }

    fn service() -> (Arc<MemoryRepository>, ResultsService) {
        let repo = Arc::new(MemoryRepository::default());
        (repo.clone(), ResultsService::new(repo))
    }

    #[tokio::test]
    async fn counts_each_event_once() {
        let (_, svc) = service();
        assert_eq!(
            svc.process_event(&event("a", 1, 2)).await.unwrap(),
            Processed::Counted
        );
        assert_eq!(
            svc.process_event(&event("a", 1, 2)).await.unwrap(),
            Processed::Duplicate
        );
        assert_eq!(
            svc.process_event(&event("b", 1, 2)).await.unwrap(),
            Processed::Counted
        );
        assert_eq!(
            svc.process_event(&event("c", 1, 3)).await.unwrap(),
            Processed::Counted
        );
        let r = svc.results(1).await.unwrap();
        assert_eq!(r.total_votes, 3);
        assert_eq!(r.counts[&2], 2);
        assert_eq!(r.counts[&3], 1);
    }

    #[tokio::test]
    async fn elections_are_independent() {
        let (_, svc) = service();
        svc.process_event(&event("a", 1, 1)).await.unwrap();
        svc.process_event(&event("b", 2, 1)).await.unwrap();
        assert_eq!(svc.results(1).await.unwrap().total_votes, 1);
        assert_eq!(svc.results(2).await.unwrap().total_votes, 1);
        assert_eq!(svc.results(3).await.unwrap().total_votes, 0);
    }

    #[tokio::test]
    async fn broadcasts_only_new_events() {
        let (_, svc) = service();
        let mut live = svc.subscribe(1).await.unwrap();
        assert_eq!(live.current.total_votes, 0);
        svc.process_event(&event("a", 1, 2)).await.unwrap();
        svc.process_event(&event("a", 1, 2)).await.unwrap();
        assert_eq!(live.updates.recv().await.unwrap().total_votes, 1);
        assert!(live.updates.try_recv().is_err());
    }

    #[tokio::test]
    async fn storage_failure_propagates_so_the_message_is_not_acked() {
        let (repo, svc) = service();
        repo.fail.store(true, Ordering::SeqCst);
        assert!(svc.process_event(&event("a", 1, 2)).await.is_err());
        repo.fail.store(false, Ordering::SeqCst);
        // Tras la reentrega el voto se cuenta normalmente.
        assert_eq!(
            svc.process_event(&event("a", 1, 2)).await.unwrap(),
            Processed::Counted
        );
    }
}
