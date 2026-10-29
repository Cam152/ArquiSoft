//! Errores del servicio.
use std::fmt;

/// Falló la base de datos de resultados (Redis). Es un error transitorio:
/// el consumidor reintenta y el HTTP responde 503.
#[derive(Debug)]
pub struct StorageError(pub String);

impl fmt::Display for StorageError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "error de almacenamiento: {}", self.0)
    }
}

impl std::error::Error for StorageError {}

impl From<redis::RedisError> for StorageError {
    fn from(e: redis::RedisError) -> Self {
        StorageError(e.to_string())
    }
}

/// El evento recibido no cumple el contrato `VoteRegistered` v1. Reintentarlo
/// no sirve de nada: el mensaje se manda a la cola de mensajes fallidos.
#[derive(Debug, PartialEq, Eq)]
pub struct InvalidEvent(pub String);

impl fmt::Display for InvalidEvent {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "evento inválido: {}", self.0)
    }
}

impl std::error::Error for InvalidEvent {}
