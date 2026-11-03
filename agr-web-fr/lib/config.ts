import "server-only";

// Configuración del servidor de Next.js. Se lee en cada petición (no al compilar),
// así la misma imagen sirve en cualquier entorno sin reconstruirla.

/**
 * Lee una variable de entorno. El nombre se pasa como variable a propósito:
 * Next.js reemplaza al compilar los accesos literales `process.env.NEXT_PUBLIC_*`,
 * pero no los dinámicos, así que el valor sale del entorno del contenedor.
 */
function readEnv(name: string, fallback: string): string {
  const value = process.env[name]?.trim();
  return value ? value : fallback;
}

function trimSlash(url: string): string {
  return url.replace(/\/+$/, "");
}

/** Gateway visto desde el servidor de Next.js (red interna de Docker). */
export function gatewayInternalUrl(): string {
  return trimSlash(readEnv("GATEWAY_INTERNAL_URL", "http://agr-gateway-or:8080"));
}

/** Gateway visto desde el navegador (puerto publicado). Solo para el voto y el SSE. */
export function gatewayPublicUrl(): string {
  return trimSlash(readEnv("NEXT_PUBLIC_GATEWAY_URL", "http://localhost:8080"));
}

/** Tiempo máximo de cada llamada del servidor al gateway, en milisegundos. */
export function gatewayTimeoutMs(): number {
  const value = Number(readEnv("GATEWAY_TIMEOUT_MS", "8000"));
  return Number.isFinite(value) && value > 0 ? value : 8000;
}
