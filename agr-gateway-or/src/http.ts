// Utilidades HTTP compartidas: leer el cuerpo, responder JSON y el error común
// {"detail": "..."} que usan todos los componentes de AGORA.

import type { IncomingMessage, ServerResponse } from "node:http";

export class HttpError extends Error {
  readonly status: number;

  constructor(status: number, detail: string) {
    super(detail);
    this.status = status;
  }
}

export function sendJson(res: ServerResponse, status: number, body: unknown): void {
  const payload = JSON.stringify(body);
  res.statusCode = status;
  res.setHeader("Content-Type", "application/json; charset=utf-8");
  res.setHeader("Content-Length", Buffer.byteLength(payload));
  res.end(payload);
}

export function sendError(res: ServerResponse, status: number, detail: string): void {
  sendJson(res, status, { detail });
}

/** Lee el cuerpo completo; responde 413 si supera el límite. */
export async function readBody(req: IncomingMessage, maxBytes: number): Promise<Buffer> {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of req) {
    const buf = chunk as Buffer;
    size += buf.length;
    if (size > maxBytes) {
      throw new HttpError(413, "El cuerpo de la petición es demasiado grande");
    }
    chunks.push(buf);
  }
  return Buffer.concat(chunks);
}

/** Interpreta el cuerpo como JSON; responde 422 si no lo es. */
export function parseJson(raw: Buffer): unknown {
  if (raw.length === 0) {
    throw new HttpError(422, "El cuerpo de la petición está vacío");
  }
  try {
    return JSON.parse(raw.toString("utf8"));
  } catch {
    throw new HttpError(422, "El cuerpo de la petición no es JSON válido");
  }
}

export function isPositiveInt(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value > 0;
}
