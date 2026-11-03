// Configuración del gateway. Todo sale de variables de entorno; los valores por
// defecto son los nombres de los contenedores en la red de Docker Compose.

export interface Config {
  port: number;
  authUrl: string;
  voterUrl: string;
  electionUrl: string;
  voteUrl: string;
  resultsUrl: string;
  auditUrl: string;
  adminApiKey: string;
  serviceApiKey: string;
  corsOrigins: string[];
  /** Tiempo máximo de cada llamada a un servicio interno, en milisegundos. */
  upstreamTimeoutMs: number;
  /** Tamaño máximo del cuerpo de una petición, en bytes. */
  maxBodyBytes: number;
}

function env(name: string, fallback: string): string {
  const value = process.env[name];
  return value === undefined || value === "" ? fallback : value;
}

function trimSlash(url: string): string {
  return url.replace(/\/+$/, "");
}

export function loadConfig(): Config {
  return {
    port: Number(env("PORT", "8080")),
    authUrl: trimSlash(env("AUTH_SERVICE_URL", "http://agr-auth-lo:8081")),
    voterUrl: trimSlash(env("VOTER_SERVICE_URL", "http://agr-voter-lo:8001")),
    electionUrl: trimSlash(env("ELECTION_SERVICE_URL", "http://agr-election-lo:8000")),
    voteUrl: trimSlash(env("VOTE_SERVICE_URL", "http://agr-vote-lo:8002")),
    resultsUrl: trimSlash(env("RESULTS_SERVICE_URL", "http://agr-results-lo:8004")),
    auditUrl: trimSlash(env("AUDIT_SERVICE_URL", "http://agr-audit-lo:8005")),
    adminApiKey: env("ADMIN_API_KEY", "dev-admin-key"),
    serviceApiKey: env("SERVICE_API_KEY", "dev-service-key"),
    corsOrigins: env("CORS_ORIGINS", "http://localhost:3000")
      .split(",")
      .map((o) => o.trim())
      .filter(Boolean),
    upstreamTimeoutMs: Number(env("UPSTREAM_TIMEOUT_MS", "5000")),
    maxBodyBytes: Number(env("MAX_BODY_BYTES", String(1024 * 1024))),
  };
}
