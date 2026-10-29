# Results Service (Rust): `agr-results-lo`

Mantiene el conteo de votos de cada elección, lo sirve al instante y avisa en vivo cada vez que cambia. Es el
único dueño de `agr-results-db` (Redis). Escucha en el puerto 8004 y solo lo llama `agr-gateway-or`.

Hace parte de [AGORA](../README.md); el diseño está en la sección 4.9 de ese documento y los contratos en la
sección 5.

```mermaid
flowchart LR
  GW["agr-gateway-or"] -- "c7 REST y SSE" --> RS["agr-results-lo<br/>Rust, :8004"]
  RS -- "c9 AMQP: se suscribe" --> BR[["agr-broker-as<br/>cola results.vote-registered"]]
  RS -. "c15 conector BD" .-> RDB[("agr-results-db<br/>Redis")]
```

## Cómo funciona

| Capa (`src/`) | Archivo | Responsabilidad |
|---|---|---|
| `consumer` | `consumer.rs` | Se suscribe a la cola, decide el destino de cada mensaje y lo confirma (*ack*) |
| `service` | `service.rs`, `hub.rs` | Cuenta cada evento una vez y difunde los conteos nuevos por elección |
| `repository` | `repository.rs` | Único lugar que habla con Redis (script Lua atómico) |
| `sse` | `sse.rs` | Convierte los conteos en el stream `text/event-stream` |
| `handler` | `handler.rs` | Rutas HTTP, validación de entrada y códigos de error |

- **Sin contar dos veces.** Por cada `VoteRegistered` ejecuta en Redis un script Lua atómico:
  `SADD results:{election_id}:seen {event_id}` y, solo si el evento es nuevo,
  `HINCRBY results:{election_id} {candidate_id} 1`. El mensaje se confirma **después** de eso, así que si el
  servicio cae a medias, el broker lo reentrega y el script lo descarta si ya estaba contado.
- **Mensajes inválidos.** Si el cuerpo no es JSON o no cumple el contrato v1 (`type` y `version` distintos, ids
  no positivos, `event_id` vacío), se rechaza con `nack` sin reencolar y el broker lo manda a `results.dead`.
- **Redis caído.** El mensaje queda sin confirmar y se reintenta con espera creciente (1 s hasta 5 s) hasta que
  Redis vuelva. No se devuelve a la cola, para no gastar las entregas de `x-delivery-limit` por un fallo pasajero.
  Mientras tanto, la API responde 503.
- **Broker caído.** El consumidor reconecta con espera creciente (hasta 15 s); los eventos esperan en la cola.
- **Resultados en vivo.** Un canal de difusión por elección. Cada conexión SSE recibe primero los conteos
  actuales y luego uno por cada voto nuevo. Si un cliente es lento y se pierden avisos, recibe los conteos
  vigentes (cada evento trae el estado completo, así que reemplaza a los anteriores).
- **Apagado ordenado.** Con `SIGTERM` cierra los streams SSE y el consumidor, y termina.

## Endpoints

| Método y ruta | Descripción |
|---|---|
| `GET /health` | `200 {"status": "ok"}` si Redis responde; `503` si no |
| `GET /results/{election_id}` | Conteos de la elección |
| `GET /results/{election_id}/stream` | SSE: eventos `counts` |

Los errores son JSON `{"detail": "mensaje"}`: `422` si `election_id` no es un entero, `503` si Redis no
responde, `404` en rutas inexistentes. Una elección sin votos (o inexistente: este servicio no conoce las
elecciones) responde `200` con conteos vacíos.

```json
// GET /results/1
{"election_id": 1, "total_votes": 11, "counts": {"1": 4, "2": 7}}
```

```
// GET /results/1/stream  (Content-Type: text/event-stream)
event: counts
data: {"election_id":1,"total_votes":11,"counts":{"1":4,"2":7}}

: keep-alive
```

No incluye nombres de candidatos ni porcentajes: los agrega el gateway.

## Configuración

| Variable | Por defecto | Uso |
|---|---|---|
| `PORT` | `8004` | Puerto HTTP |
| `REDIS_URL` | `redis://agr-results-db:6379` | Conexión a Redis (con clave: `redis://:clave@host:6379`) |
| `AMQP_URL` | `amqp://agora:agora@agr-broker-as:5672/%2f` | Conexión a RabbitMQ |
| `RESULTS_QUEUE` | `results.vote-registered` | Cola a consumir (la declara `agr-broker-as`) |
| `CONSUMER_PREFETCH` | `10` | Mensajes sin confirmar que el broker entrega a la vez |
| `SSE_KEEPALIVE_SECS` | `15` | Cada cuántos segundos se envía el comentario de mantenimiento |

`docker-compose.results.yml` define `agr-results-db` (Redis con `appendonly yes`, volumen `results-db-data` y
clave `RESULTS_DB_PASSWORD`) y `agr-results-lo`. Ver el encabezado del archivo para integrarlo al compose de la
raíz.

## Pruebas

Requisitos: Rust estable (o Docker, que compila en la imagen).

```bash
cd agr-results-lo
cargo fmt --check && cargo clippy --all-targets -- -D warnings
cargo test                      # unitarias, no necesitan servicios externos
```

Pruebas contra Redis y RabbitMQ reales (conteo sin duplicados con concurrencia, eventos del broker, mensajes
inválidos a la cola de fallidos, eventos publicados con el servicio apagado, caída pasajera de Redis):

```bash
docker run -d --name t-redis -p 6379:6379 redis:7-alpine
docker run -d --name t-rabbit -p 5672:5672 rabbitmq:3-management-alpine

TEST_REDIS_URL=redis://localhost:6379 \
TEST_AMQP_URL=amqp://guest:guest@localhost:5672/%2f \
cargo test --test integration -- --ignored --test-threads=1
```

Con el sistema completo levantado, `curl http://localhost:8080/results/1` (gateway) o, con el puerto 8004
publicado en un `docker-compose.override.yml`, `curl -N http://localhost:8004/results/1/stream`.
