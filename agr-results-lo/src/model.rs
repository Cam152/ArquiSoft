//! Estructuras de datos: el evento `VoteRegistered` y los conteos.
use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

use crate::error::InvalidEvent;

pub const EVENT_TYPE: &str = "VoteRegistered";
pub const SUPPORTED_VERSION: u32 = 1;

/// Evento `VoteRegistered` v1 (README, sección 5.3). Solo se leen los campos
/// necesarios para contar; `occurred_at` y cualquier campo nuevo se ignoran.
/// No lleva ningún dato del votante.
#[derive(Debug, Clone, Deserialize)]
pub struct VoteRegistered {
    pub event_id: String,
    #[serde(rename = "type")]
    pub kind: String,
    pub version: u32,
    pub election_id: i64,
    pub candidate_id: i64,
}

impl VoteRegistered {
    /// Interpreta el cuerpo de un mensaje AMQP y comprueba el contrato.
    pub fn parse(body: &[u8]) -> Result<VoteRegistered, InvalidEvent> {
        let event: VoteRegistered =
            serde_json::from_slice(body).map_err(|e| InvalidEvent(e.to_string()))?;
        event.validate()?;
        Ok(event)
    }

    pub fn validate(&self) -> Result<(), InvalidEvent> {
        if self.kind != EVENT_TYPE {
            return Err(InvalidEvent(format!("type no soportado: {:?}", self.kind)));
        }
        if self.version != SUPPORTED_VERSION {
            return Err(InvalidEvent(format!(
                "version no soportada: {}",
                self.version
            )));
        }
        if self.event_id.trim().is_empty() {
            return Err(InvalidEvent("event_id vacío".into()));
        }
        if self.election_id <= 0 || self.candidate_id <= 0 {
            return Err(InvalidEvent(
                "election_id y candidate_id deben ser positivos".into(),
            ));
        }
        Ok(())
    }
}

/// Conteos de una elección. Es la respuesta de `GET /results/{id}` y el dato
/// de cada evento SSE `counts`. No incluye nombres de candidatos: los agrega
/// el gateway.
#[derive(Debug, Clone, PartialEq, Eq, Serialize)]
pub struct Counts {
    pub election_id: i64,
    pub total_votes: i64,
    pub counts: BTreeMap<i64, i64>,
}

impl Counts {
    pub fn new(election_id: i64, counts: BTreeMap<i64, i64>) -> Counts {
        let total_votes = counts.values().sum();
        Counts {
            election_id,
            total_votes,
            counts,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    const VALID: &str = r#"{"event_id":"6720f3a1c2b9e8a4d1f00a17","type":"VoteRegistered","version":1,
        "election_id":1,"candidate_id":2,"occurred_at":"2026-11-03T14:00:00Z"}"#;

    #[test]
    fn parses_the_contract_example() {
        let e = VoteRegistered::parse(VALID.as_bytes()).unwrap();
        assert_eq!(e.event_id, "6720f3a1c2b9e8a4d1f00a17");
        assert_eq!((e.election_id, e.candidate_id), (1, 2));
    }

    #[test]
    fn ignores_unknown_fields() {
        let body = VALID.replace("\"version\":1,", "\"version\":1,\"extra\":true,");
        assert!(VoteRegistered::parse(body.as_bytes()).is_ok());
    }

    #[test]
    fn rejects_bad_events() {
        let cases = [
            "no es json".to_string(),
            "{}".to_string(),
            VALID.replace("VoteRegistered", "Other"),
            VALID.replace("\"version\":1", "\"version\":2"),
            VALID.replace("6720f3a1c2b9e8a4d1f00a17", " "),
            VALID.replace("\"candidate_id\":2", "\"candidate_id\":0"),
            VALID.replace("\"election_id\":1", "\"election_id\":-4"),
            VALID.replace("\"candidate_id\":2", "\"candidate_id\":\"2\""),
        ];
        for body in cases {
            assert!(VoteRegistered::parse(body.as_bytes()).is_err(), "{body}");
        }
    }

    #[test]
    fn counts_total_and_json_shape() {
        let c = Counts::new(1, BTreeMap::from([(2, 7), (1, 4)]));
        assert_eq!(c.total_votes, 11);
        assert_eq!(
            serde_json::to_string(&c).unwrap(),
            r#"{"election_id":1,"total_votes":11,"counts":{"1":4,"2":7}}"#
        );
    }
}
