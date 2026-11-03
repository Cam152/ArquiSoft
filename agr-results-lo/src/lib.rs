//! agr-results-lo: conteo de votos y resultados en vivo de AGORA.
//!
//! Capas internas (README, sección 4.9):
//! `handler` (HTTP) → `service` (reglas) → `repository` (Redis), más `consumer`
//! (broker) y `sse` (stream de resultados en vivo).
pub mod config;
pub mod consumer;
pub mod error;
pub mod handler;
pub mod hub;
pub mod model;
pub mod repository;
pub mod service;
pub mod shutdown;
pub mod sse;
