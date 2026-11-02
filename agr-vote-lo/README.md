# agr-vote-lo: Vote Service (Go)

Guarda el voto anónimo en `agr-votes-db` (MongoDB) y avisa al resto del sistema publicando el evento
`VoteRegistered` en `agr-broker-as` (RabbitMQ). Responsable: Maicol. Puerto interno: 8002 (no se publica).

En el Prototipo 2 **ya no orquesta**: la validación de la elección, el marcado del votante y la compensación los
hace la saga de `agr-gateway-or`, y los resultados los calcula `agr-results-lo`. Este servicio solo lo llama el
gateway (conector c6), nunca el navegador.

```mermaid
flowchart LR
  GW["agr-gateway-or"] -- "c6 REST<br/>X-Service-Key + Idempotency-Key" --> VT["agr-vote-lo<br/>Go, :8002"]
  VT -. "c14 conector BD" .-> DB[("agr-votes-db<br/>MongoDB")]
  VT -- "c8 AMQP: VoteRegistered<br/>con confirmación" --> BR[["agr-broker-as<br/>RabbitMQ"]]
```

## Endpoints

| Método y ruta | Descripción |
|---|---|
| `GET /health` | `200 {"status": "ok"}` si MongoDB responde; `503` si no |
| `POST /votes` | Guarda un voto. Exige `X-Service-Key` e `Idempotency-Key` |

```json
// POST /votes   (X-Service-Key: <llave>, Idempotency-Key: <uuid aleatorio del gateway>)
{"election_id": 1, "candidate_id": 2, "candidate_name": "Luis Herrera"}
// 201
{"status": "recorded"}
```

| Código | Cuándo |
|---|---|
| 201 | Voto guardado. **También si la `Idempotency-Key` ya existía con el mismo voto** (el gateway reintentó) |
| 401 | Falta `X-Service-Key` o no coincide |
| 409 | La `Idempotency-Key` ya se usó con **otro** voto |
| 422 | Falta `Idempotency-Key`, o el cuerpo es inválido (ids no positivos, sin `candidate_name`) |
| 500 | MongoDB no aceptó el voto. El gateway compensa desmarcando al votante |

Errores siempre como `{"detail": "mensaje"}`.

## Cómo funciona

**Guardado idempotente.** El voto se inserta con su `idempotency_key`, que tiene índice único. Si el gateway no
recibió respuesta y reintenta con la misma llave, la inserción choca con el índice, el servicio comprueba que sea
el mismo voto y responde 201 sin duplicarlo.

**Outbox.** El voto se guarda con `published: false` en el mismo documento (en MongoDB la escritura de un documento
es atómica). Luego:

1. Después de responder, una *goroutine* publica `VoteRegistered` en el exchange `agora.votes` y **espera la
   confirmación del broker** (*publisher confirms*). Con el ack marca `published: true`.
2. Otra *goroutine* revisa cada 5 s (`OUTBOX_INTERVAL`) los votos que sigan en `false` y los vuelve a publicar.
   También lo hace al arrancar.

Así, si RabbitMQ está caído cuando se guarda el voto, el evento sale cuando vuelva. La entrega es **al menos una
vez**: los consumidores descartan repetidos por `event_id`. Un índice parcial sobre `published: false` hace que la
revisión periódica no recorra los votos ya publicados.

**Conexión al broker.** Se abre la primera vez que se necesita y se reabre sola si se cae. Al conectarse comprueba
(de forma pasiva) que el exchange exista: la topología es de `agr-broker-as`, este servicio no la crea. Publica con
`mandatory`: si el exchange no tuviera colas enlazadas, el evento se queda pendiente en vez de perderse.

**Anonimato.**

- El voto no lleva ningún dato del votante: el gateway no los envía y el servicio no los acepta.
- `cast_at` y `occurred_at` se redondean a la hora.
- El `_id` del voto (que viaja como `event_id`) son 12 bytes **aleatorios**, no un `ObjectId` de MongoDB, porque
  el `ObjectId` lleva la hora exacta de creación.
- El mensaje AMQP no lleva `timestamp`, por la misma razón.
- Los logs registran método, ruta, código y duración; nunca el cuerpo.

## Contrato del evento `VoteRegistered`

Exchange `agora.votes` (fanout). Mensaje persistente, `content-type: application/json`, `message-id` y `type`
iguales a `event_id` y `type`:

```json
{
  "event_id": "6720f3a1c2b9e8a4d1f00a17",
  "type": "VoteRegistered",
  "version": 1,
  "election_id": 1,
  "candidate_id": 2,
  "occurred_at": "2026-11-03T14:00:00Z"
}
```

## Datos: `agr-votes-db`

MongoDB 7, base `votes`, colección `votes`:

```json
{"_id": "6720f3a1c2b9e8a4d1f00a17", "election_id": 1, "candidate_id": 2, "candidate_name": "Luis Herrera",
 "cast_at": ISODate("2026-11-03T14:00:00Z"), "published": true, "idempotency_key": "<uuid del gateway>"}
```

| Índice | Para qué |
|---|---|
| `idempotency_key_unique` (único) | Un voto por llave: el reintento no duplica |
| `published_pending` (parcial, `published: false`) | La revisión del outbox solo ve los pendientes |

Tiene usuario y clave propios (`VOTES_DB_USER`, `VOTES_DB_PASSWORD`): solo este servicio los conoce. Volumen
`votes-db-data`.

> Si venían del Prototipo 1, borren el volumen viejo una vez (`docker compose down -v`): aquel MongoDB no tenía
> usuario y el nuevo sí.

## Capas (`internal/`)

| Paquete | Responsabilidad |
|---|---|
| `handler` | HTTP: lee la petición, verifica `X-Service-Key` (tiempo constante), traduce errores a códigos |
| `service` | Reglas de negocio: valida el voto, genera el `_id`, redondea la hora, maneja la llave repetida |
| `repository` | Único acceso a MongoDB: inserción, búsqueda por llave, pendientes del outbox, índices |
| `outbox` | Publicación inmediata y revisión periódica de los votos sin publicar |
| `publisher` | Único acceso a RabbitMQ: conexión, confirmaciones, formato del mensaje |
| `model` | Tipos compartidos: `Vote`, `VoteRegistered` |
| `config` | Variables de entorno |

## Variables de entorno

| Variable | Por defecto |
|---|---|
| `PORT` | `8002` |
| `MONGO_URI` | `mongodb://agr-votes-db:27017` (el compose agrega usuario y clave) |
| `MONGO_DB` | `votes` |
| `AMQP_URL` | `amqp://agora:agora@agr-broker-as:5672/` |
| `VOTES_EXCHANGE` | `agora.votes` |
| `SERVICE_API_KEY` | `dev-service-key` |
| `OUTBOX_INTERVAL` | `5s` |
| `OUTBOX_BATCH` | `100` |

## Cómo ejecutarlo y probarlo

Desde la raíz del repositorio:

```bash
docker compose up --build -d --wait agr-vote-lo   # levanta también agr-votes-db y agr-broker-as
docker compose logs -f agr-vote-lo
```

**Pruebas unitarias** (no necesitan Docker ni base de datos; las dependencias se simulan):

```bash
cd agr-vote-lo
go test -race -cover ./...
```

**Pruebas de punta a punta** contra MongoDB y RabbitMQ reales, en un proyecto de Compose aparte que se borra al
terminar (no toca sus datos):

```bash
./agr-vote-lo/e2e/run.sh
```

Comprueban: la topología del broker, que el voto se guarde con exactamente los campos esperados, que el evento
cumpla el contrato y llegue a las colas de resultados y auditoría, la idempotencia, los rechazos y que el outbox
publique un voto pendiente.
