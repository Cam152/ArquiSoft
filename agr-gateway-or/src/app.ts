// Servidor HTTP del gateway: CORS, enrutamiento, manejo de errores y logs.

import { createServer } from "node:http";
import type { Server } from "node:http";
import type { Config } from "./config.ts";
import { HttpError, sendError } from "./http.ts";
import { applyCors } from "./middleware/cors.ts";
import { buildRoutes, matchRoute } from "./routes/index.ts";

export type Logger = (message: string) => void;

export function createGateway(cfg: Config, log: Logger = (m) => console.log(m)): Server {
  const routes = buildRoutes(cfg, log);

  return createServer((req, res) => {
    const started = Date.now();
    const rawUrl = req.url ?? "/";
    const queryIndex = rawUrl.indexOf("?");
    const path = queryIndex === -1 ? rawUrl : rawUrl.slice(0, queryIndex);
    const query = queryIndex === -1 ? "" : rawUrl.slice(queryIndex);

    // Log de cada petición: método, ruta (sin query), código y duración.
    // Nunca el cuerpo ni los headers: en POST /votes irían juntos el token y el candidato.
    res.on("finish", () => {
      if (path === "/health") return;
      log(JSON.stringify({ method: req.method, path, status: res.statusCode, ms: Date.now() - started }));
    });

    if (applyCors(req, res, cfg.corsOrigins)) return;

    const handle = async () => {
      const found = matchRoute(routes, req.method ?? "GET", path);
      if (!found) throw new HttpError(404, "Recurso no encontrado");
      await found.route.handler({ req, res, params: found.params, query });
    };

    handle().catch((err: unknown) => {
      if (res.headersSent) {
        res.destroy();
        return;
      }
      if (err instanceof HttpError) {
        sendError(res, err.status, err.message);
        return;
      }
      log(`error interno en ${req.method} ${path}: ${err instanceof Error ? err.message : String(err)}`);
      sendError(res, 500, "Error interno del gateway");
    });
  });
}
