// CORS: el gateway es el único componente que llama el navegador directamente
// (voto y resultados en vivo), así que es el único que lo necesita.

import type { IncomingMessage, ServerResponse } from "node:http";

const ALLOWED_METHODS = "GET, POST, PATCH, DELETE, OPTIONS";
const ALLOWED_HEADERS = "Authorization, Content-Type, X-API-Key";

/** Agrega los encabezados CORS. Devuelve true si la petición era un preflight ya respondido. */
export function applyCors(req: IncomingMessage, res: ServerResponse, origins: string[]): boolean {
  const origin = req.headers.origin;
  if (origin && origins.includes(origin)) {
    res.setHeader("Access-Control-Allow-Origin", origin);
    res.setHeader("Vary", "Origin");
  }
  if (req.method === "OPTIONS") {
    res.setHeader("Access-Control-Allow-Methods", ALLOWED_METHODS);
    res.setHeader("Access-Control-Allow-Headers", ALLOWED_HEADERS);
    res.setHeader("Access-Control-Max-Age", "600");
    res.statusCode = 204;
    res.end();
    return true;
  }
  return false;
}
