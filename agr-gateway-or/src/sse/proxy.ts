// Reenvío del stream SSE de resultados en vivo (agr-results-lo -> navegador).
// Cada evento se escribe en cuanto llega: sin compresión ni acumulación.

import type { IncomingMessage, ServerResponse } from "node:http";
import type { Config } from "../config.ts";
import { sendError } from "../http.ts";

export async function proxyResultsStream(
  cfg: Config,
  electionId: number,
  req: IncomingMessage,
  res: ServerResponse,
): Promise<void> {
  const controller = new AbortController();
  // Si el navegador cierra la conexión, también se cierra la del servicio.
  const onClose = () => controller.abort();
  req.on("close", onClose);
  res.on("close", onClose);

  // El tiempo máximo aplica solo hasta recibir los encabezados; el stream dura lo que quiera.
  const connectTimer = setTimeout(() => controller.abort(), cfg.upstreamTimeoutMs);

  let upstream: Response;
  try {
    upstream = await fetch(`${cfg.resultsUrl}/results/${electionId}/stream`, {
      headers: { Accept: "text/event-stream" },
      signal: controller.signal,
    });
  } catch {
    clearTimeout(connectTimer);
    if (!res.headersSent && !res.destroyed) sendError(res, 502, "El Results Service no responde");
    return;
  }
  clearTimeout(connectTimer);

  if (upstream.status === 404) {
    sendError(res, 404, "La elección no existe");
    return;
  }
  if (upstream.status !== 200 || !upstream.body) {
    sendError(res, 502, "El Results Service no responde");
    return;
  }

  res.statusCode = 200;
  res.setHeader("Content-Type", "text/event-stream; charset=utf-8");
  res.setHeader("Cache-Control", "no-cache, no-transform");
  res.setHeader("Connection", "keep-alive");
  res.setHeader("X-Accel-Buffering", "no");
  res.flushHeaders();

  const reader = upstream.body.getReader();
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      if (res.destroyed) break;
      res.write(value);
    }
  } catch {
    // El servicio o el navegador cerraron la conexión: no hay nada más que hacer.
  } finally {
    req.off("close", onClose);
    controller.abort();
    if (!res.destroyed) res.end();
  }
}
