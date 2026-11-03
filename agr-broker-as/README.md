# agr-broker-as: Broker de mensajes (RabbitMQ)

Componente de procesos asíncronos de AGORA. Recibe cada `VoteRegistered` que publica `agr-vote-lo` y lo entrega a
`agr-results-lo` y a `agr-audit-lo`, aunque en ese momento estén caídos. Responsable: Maicol.

```mermaid
flowchart LR
  VT["agr-vote-lo"] -- "c8 publica<br/>(confirmado)" --> EX{{"agora.votes<br/>fanout"}}
  EX --> RQ[["results.vote-registered<br/>quorum"]]
  EX --> AQ[["audit.vote-registered<br/>quorum"]]
  RS["agr-results-lo"] -- "c9 consume" --> RQ
  AD["agr-audit-lo"] -- "c10 consume" --> AQ
  RQ -. "5 entregas fallidas" .-> DLX{{"agora.dlx<br/>direct"}}
  AQ -. "5 entregas fallidas" .-> DLX
  DLX -- results.dead --> RD[["results.dead"]]
  DLX -- audit.dead --> ADQ[["audit.dead"]]
```

## Topología

Se declara en [`definitions.template.json`](definitions.template.json) y RabbitMQ la carga al arrancar: nadie crea
colas a mano y ningún servicio tiene que declararlas.

| Elemento | Configuración | Para qué |
|---|---|---|
| Exchange `agora.votes` | `fanout`, durable | Copia cada evento a todas sus colas |
| Cola `results.vote-registered` | `quorum`, `x-delivery-limit: 5`, *dead-letter* a `agora.dlx` con clave `results.dead` | Eventos para `agr-results-lo` |
| Cola `audit.vote-registered` | `quorum`, `x-delivery-limit: 5`, *dead-letter* a `agora.dlx` con clave `audit.dead` | Eventos para `agr-audit-lo` |
| Exchange `agora.dlx` | `direct`, durable | Envía cada mensaje fallido a la cola de su consumidor |
| Colas `results.dead`, `audit.dead` | `quorum` | Guardan los mensajes que fallaron 5 veces, para revisarlos |

- **Colas *quorum*:** replicadas y escritas a disco; con `x-delivery-limit` RabbitMQ cuenta las entregas y, al
  llegar a 5, manda el mensaje a la *dead-letter* en lugar de reintentarlo para siempre (un mensaje "veneno" no
  bloquea la cola).
- **`agora.dlx` es `direct` y no `fanout`:** así los fallidos de resultados no aparecen en la cola de fallidos de
  auditoría, y viceversa.
- Para agregar otro consumidor basta con una cola nueva enlazada a `agora.votes`, sin tocar `agr-vote-lo`.

### Garantías de entrega

| Lado | Cómo |
|---|---|
| Publicador (`agr-vote-lo`) | Mensajes **persistentes**, **confirmación del broker** (*publisher confirms*) y `mandatory`. Si no hay confirmación, el voto queda pendiente en su outbox y se reintenta |
| Broker | Exchange, colas y mensajes durables sobre el volumen `broker-data` |
| Consumidores | ***Ack* manual** después de procesar. Si el consumidor cae a mitad, RabbitMQ reentrega el mensaje |

El resultado es entrega **al menos una vez**: los consumidores descartan los eventos repetidos por `event_id`.

### Qué deben hacer los consumidores

- **No declarar las colas** (o declararlas en modo pasivo). Si las declaran con otros argumentos, RabbitMQ responde
  `PRECONDITION_FAILED` y cierra el canal.
- Hacer `ack` solo después de guardar el efecto del evento; `nack` sin `requeue` para un mensaje que nunca se
  podrá procesar (va directo a la *dead-letter*).
- Usar un `prefetch` pequeño (por ejemplo 10) para no acaparar mensajes.

## Usuario y clave

`BROKER_USER` y `BROKER_PASSWORD` (por defecto `agora` / `agora`). Valen para AMQP y para la consola.

Cuando RabbitMQ carga definiciones al arrancar **no crea el usuario por defecto** (ignora
`RABBITMQ_DEFAULT_USER`). Por eso la imagen tiene un [`entrypoint.sh`](entrypoint.sh) que genera
`/etc/rabbitmq/definitions.json` con el usuario de las variables de entorno y luego arranca RabbitMQ normalmente.
El usuario `guest` no existe.

## Archivos

| Archivo | Contenido |
|---|---|
| `Dockerfile` | `rabbitmq:3.13-management-alpine` con la configuración y el entrypoint |
| `definitions.template.json` | Vhost, usuario, permisos, exchanges, colas y enlaces |
| `rabbitmq.conf` | Carga de las definiciones, logs a la consola, límite de disco |
| `entrypoint.sh` | Genera las definiciones con el usuario y arranca RabbitMQ |

## Uso

```bash
docker compose up --build -d --wait agr-broker-as
docker compose logs -f agr-broker-as
```

- Consola web: http://localhost:15672 (usuario y clave de `.env`). En **Queues** se ven los mensajes de cada cola
  y en **Exchanges → agora.votes** la tasa de publicación.
- AMQP (5672) solo es accesible dentro de Docker: `amqp://agora:agora@agr-broker-as:5672/`.
- El *healthcheck* es `rabbitmq-diagnostics check_port_connectivity`: el contenedor queda `healthy` cuando ya
  acepta conexiones AMQP, es decir, con la topología cargada.
- `hostname: agr-broker-as` está fijo en el compose: RabbitMQ guarda sus datos por nombre de nodo, y con un nombre
  al azar perdería los mensajes del volumen al recrear el contenedor.

Ver los mensajes fallidos:

```bash
docker compose exec agr-broker-as rabbitmqctl list_queues name messages consumers
```
