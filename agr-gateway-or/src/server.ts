// Punto de entrada: lee la configuración y levanta el gateway.

import { createGateway } from "./app.ts";
import { loadConfig } from "./config.ts";

const cfg = loadConfig();
const server = createGateway(cfg);

server.listen(cfg.port, () => {
  console.log(`agr-gateway-or escuchando en el puerto ${cfg.port}`);
});

function shutdown(signal: string): void {
  console.log(`${signal} recibido, cerrando el gateway`);
  server.close(() => process.exit(0));
  // Los streams SSE abiertos no dejan cerrar; se fuerza la salida a los 5 segundos.
  setTimeout(() => process.exit(0), 5000).unref();
}

process.on("SIGTERM", () => shutdown("SIGTERM"));
process.on("SIGINT", () => shutdown("SIGINT"));
