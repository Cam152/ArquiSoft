// Reenvío simple de una petición a un servicio interno, devolviendo su
// respuesta tal cual (código, tipo de contenido y cuerpo).

import type { ServerResponse } from "node:http";
import { callUpstream, UpstreamError } from "../clients/upstream.ts";
import { HttpError } from "../http.ts";

export interface ForwardOptions {
  service: string;
  url: string;
  method: string;
  headers?: Record<string, string>;
  body?: Buffer;
  timeoutMs: number;
}

export async function forward(res: ServerResponse, opts: ForwardOptions): Promise<void> {
  let upstream;
  try {
    upstream = await callUpstream({
      service: opts.service,
      url: opts.url,
      method: opts.method,
      headers: opts.headers,
      body: opts.body && opts.body.length > 0 ? opts.body : undefined,
      timeoutMs: opts.timeoutMs,
    });
  } catch (err) {
    if (err instanceof UpstreamError) {
      throw new HttpError(502, `El ${opts.service} no responde`);
    }
    throw err;
  }
  res.statusCode = upstream.status;
  if (upstream.status !== 204) {
    res.setHeader("Content-Type", upstream.contentType);
    res.setHeader("Content-Length", upstream.body.length);
    res.end(upstream.body);
  } else {
    res.end();
  }
}
