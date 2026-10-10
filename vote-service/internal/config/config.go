// Package config lee la configuración del servicio de variables de entorno.
package config

import (
	"os"
	"strings"
)

// Config es la configuración del servicio.
type Config struct {
	Port               string
	MongoURI           string
	MongoDB            string
	ElectionServiceURL string
	VoterServiceURL    string
	ServiceAPIKey      string
	CORSOrigins        []string
}

// Load lee las variables de entorno; todas tienen un valor por defecto.
func Load() Config {
	var origins []string
	for _, o := range strings.Split(getenv("CORS_ORIGINS", "http://localhost:3000"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	return Config{
		Port:               getenv("PORT", "8002"),
		MongoURI:           getenv("MONGO_URI", "mongodb://votes-db:27017"),
		MongoDB:            getenv("MONGO_DB", "votes"),
		ElectionServiceURL: strings.TrimRight(getenv("ELECTION_SERVICE_URL", "http://election-service:8000"), "/"),
		VoterServiceURL:    strings.TrimRight(getenv("VOTER_SERVICE_URL", "http://voter-service:8001"), "/"),
		ServiceAPIKey:      getenv("SERVICE_API_KEY", "dev-service-key"),
		CORSOrigins:        origins,
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
