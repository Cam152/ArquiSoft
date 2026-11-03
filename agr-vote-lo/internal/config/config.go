// Package config lee la configuración del servicio de variables de entorno.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config es la configuración del servicio.
type Config struct {
	Port          string
	MongoURI      string
	MongoDB       string
	AMQPURL       string
	Exchange      string
	ServiceAPIKey string
	// OutboxInterval es cada cuánto se reintentan los votos sin publicar.
	OutboxInterval time.Duration
	// OutboxBatch es cuántos votos sin publicar se toman en cada vuelta.
	OutboxBatch int
}

// Load lee las variables de entorno; todas tienen un valor por defecto que
// coincide con los nombres de Docker Compose.
func Load() Config {
	return Config{
		Port:           getenv("PORT", "8002"),
		MongoURI:       getenv("MONGO_URI", "mongodb://agr-votes-db:27017"),
		MongoDB:        getenv("MONGO_DB", "votes"),
		AMQPURL:        getenv("AMQP_URL", "amqp://agora:agora@agr-broker-as:5672/"),
		Exchange:       getenv("VOTES_EXCHANGE", "agora.votes"),
		ServiceAPIKey:  getenv("SERVICE_API_KEY", "dev-service-key"),
		OutboxInterval: getDuration("OUTBOX_INTERVAL", 5*time.Second),
		OutboxBatch:    getInt("OUTBOX_BATCH", 100),
	}
}

func getenv(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if d, err := time.ParseDuration(getenv(key, "")); err == nil && d > 0 {
		return d
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if n, err := strconv.Atoi(getenv(key, "")); err == nil && n > 0 {
		return n
	}
	return fallback
}
