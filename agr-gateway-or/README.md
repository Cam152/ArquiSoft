# agr-gateway-or: API Gateway y orquestador

Único punto de entrada de la API de AGORA (puerto 8080). Enruta cada petición al servicio que corresponde, valida el
JWT con `agr-auth-lo`, protege la administración con `X-API-Key`, ejecuta la saga del voto, une los conteos de
resultados con los nombres de candidatos y reenvía el stream SSE de resultados en vivo. El contrato completo está
en el README principal (secciones 4.3, 5.1 y 6.1).

## Cómo está hecho

TypeScript **sin dependencias**: solo `node:http`, `fetch` y `node:crypto`. Node 22.18 o superior ejecuta los
archivos `.ts` directamente (quita los tipos al cargarlos), así que no hay paso de compilación y la imagen de Docker
no necesita `npm install`. Por eso el código solo usa TypeScript "borrable": `import type`, sin `enum` ni
`namespace`, e importa los archivos con extensión `.ts`.

| Archivo | Responsabilidad |
|---|---|
| `src/server.ts` | Arranque y apagado ordenado |
| `src/config.ts` | Variables de entorno |
| `src/app.ts` | Servidor HTTP, CORS, errores y logs |
| `src/routes/index.ts` | Tabla de rutas públicas |
| `src/routes/forward.ts` | Reenvío simple a un servicio interno |
| `src/middleware/admin.ts` | Verificación de `X-API-Key` (tiempo constante) |
| `src/middleware/cors.ts` | CORS para el origen del web |
| `src/saga/vote.ts` | Saga del voto con compensación y reintento idempotente |
| `src/compose/results.ts` | Composición de resultados (conteos + candidatos) |
| `src/sse/proxy.ts` | Reenvío del stream SSE sin acumular |
| `src/clients/*.ts` | Un cliente HTTP por servicio, con tiempo máximo |
| `test/gateway.test.ts` | Pruebas contra servicios simulados (no necesitan Docker) |

## Cómo ejecutarlo

Con Docker Compose, desde la raíz del repositorio:

```bash
docker compose up --build -d agr-gateway-or
curl.exe http://localhost:8080/health
```

Sin Docker (Node 22.18 o superior):

```bash
cd agr-gateway-or
npm start          # escucha en el puerto 8080
npm test           # pruebas
```

Para revisar los tipos hace falta instalar las dependencias de desarrollo una vez: `npm install` y luego
`npm run typecheck`.

## Variables de entorno

| Variable | Por defecto |
|---|---|
| `PORT` | `8080` |
| `AUTH_SERVICE_URL` | `http://agr-auth-lo:8081` |
| `VOTER_SERVICE_URL` | `http://agr-voter-lo:8001` |
| `ELECTION_SERVICE_URL` | `http://agr-election-lo:8000` |
| `VOTE_SERVICE_URL` | `http://agr-vote-lo:8002` |
| `RESULTS_SERVICE_URL` | `http://agr-results-lo:8004` |
| `AUDIT_SERVICE_URL` | `http://agr-audit-lo:8005` |
| `ADMIN_API_KEY` | `dev-admin-key` |
| `SERVICE_API_KEY` | `dev-service-key` |
| `CORS_ORIGINS` | `http://localhost:3000` |
| `UPSTREAM_TIMEOUT_MS` | `5000` |
| `MAX_BODY_BYTES` | `1048576` |

## Decisiones de la saga

- Si `agr-vote-lo` **responde** que no guardó el voto, se compensa con `unmark-voted` y se responde 500.
- Si **rechaza la conexión** (está apagado), el voto seguro no se guardó: se compensa y se responde 502.
- Si **no responde a tiempo** o corta la conexión, se reintenta una vez con la misma `Idempotency-Key`. Si sigue
  sin respuesta, se responde 502 **sin compensar**, porque el voto pudo haberse guardado.
- Los logs solo registran método, ruta, código y duración. Nunca el cuerpo ni el header `Authorization`.
