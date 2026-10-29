//! Configuración por variables de entorno; todas tienen un valor por defecto.
use std::time::Duration;

#[derive(Debug, Clone)]
pub struct Config {
    pub port: u16,
    pub redis_url: String,
    pub amqp_url: String,
    pub queue: String,
    pub prefetch: u16,
    pub keepalive: Duration,
}

impl Config {
    pub fn from_env() -> Result<Config, String> {
        Self::from_lookup(|key| std::env::var(key).ok())
    }

    /// Separado de `from_env` para poder probarlo sin tocar el entorno real.
    pub fn from_lookup(get: impl Fn(&str) -> Option<String>) -> Result<Config, String> {
        let var = |key: &str, default: &str| {
            get(key)
                .filter(|v| !v.trim().is_empty())
                .unwrap_or_else(|| default.to_string())
        };
        let number = |key: &str, default: &str| -> Result<u64, String> {
            let raw = var(key, default);
            raw.trim()
                .parse::<u64>()
                .map_err(|_| format!("{key} debe ser un entero positivo (recibido: {raw:?})"))
        };

        let port = number("PORT", "8004")?;
        let port = u16::try_from(port).map_err(|_| format!("PORT fuera de rango: {port}"))?;
        let prefetch = number("CONSUMER_PREFETCH", "10")?;
        let prefetch = u16::try_from(prefetch)
            .ok()
            .filter(|p| *p > 0)
            .ok_or_else(|| format!("CONSUMER_PREFETCH debe estar entre 1 y 65535: {prefetch}"))?;
        let keepalive = number("SSE_KEEPALIVE_SECS", "15")?;
        if keepalive == 0 {
            return Err("SSE_KEEPALIVE_SECS debe ser mayor que 0".into());
        }

        Ok(Config {
            port,
            redis_url: var("REDIS_URL", "redis://agr-results-db:6379"),
            amqp_url: var("AMQP_URL", "amqp://agora:agora@agr-broker-as:5672/%2f"),
            queue: var("RESULTS_QUEUE", "results.vote-registered"),
            prefetch,
            keepalive: Duration::from_secs(keepalive),
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashMap;

    fn lookup(pairs: &[(&str, &str)]) -> impl Fn(&str) -> Option<String> {
        let map: HashMap<String, String> = pairs
            .iter()
            .map(|(k, v)| (k.to_string(), v.to_string()))
            .collect();
        move |key| map.get(key).cloned()
    }

    #[test]
    fn defaults() {
        let c = Config::from_lookup(lookup(&[])).unwrap();
        assert_eq!(c.port, 8004);
        assert_eq!(c.redis_url, "redis://agr-results-db:6379");
        assert_eq!(c.queue, "results.vote-registered");
        assert_eq!(c.prefetch, 10);
        assert_eq!(c.keepalive, Duration::from_secs(15));
    }

    #[test]
    fn overrides_and_blank_values_fall_back() {
        let c = Config::from_lookup(lookup(&[
            ("PORT", "9000"),
            ("REDIS_URL", "redis://x:1"),
            ("RESULTS_QUEUE", "  "),
        ]))
        .unwrap();
        assert_eq!(c.port, 9000);
        assert_eq!(c.redis_url, "redis://x:1");
        assert_eq!(c.queue, "results.vote-registered");
    }

    #[test]
    fn rejects_invalid_numbers() {
        assert!(Config::from_lookup(lookup(&[("PORT", "abc")])).is_err());
        assert!(Config::from_lookup(lookup(&[("PORT", "70000")])).is_err());
        assert!(Config::from_lookup(lookup(&[("CONSUMER_PREFETCH", "0")])).is_err());
        assert!(Config::from_lookup(lookup(&[("SSE_KEEPALIVE_SECS", "0")])).is_err());
    }
}
