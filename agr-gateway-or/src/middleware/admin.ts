// Protección de las rutas /admin/*: el header X-API-Key debe coincidir con
// ADMIN_API_KEY. La comparación es de tiempo constante.

import { createHash, timingSafeEqual } from "node:crypto";
import type { IncomingMessage } from "node:http";
import { HttpError } from "../http.ts";

function digest(value: string): Buffer {
  return createHash("sha256").update(value, "utf8").digest();
}

export function requireAdmin(req: IncomingMessage, adminApiKey: string): void {
  const received = req.headers["x-api-key"];
  const key = Array.isArray(received) ? received[0] : received;
  // Se comparan los hash para que el largo de la llave no cambie el tiempo de respuesta.
  if (!key || !timingSafeEqual(digest(key), digest(adminApiKey))) {
    throw new HttpError(401, "Llave de administración inválida");
  }
}
