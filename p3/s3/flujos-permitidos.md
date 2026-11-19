# S3 · Network Segmentation: zonas y flujos permitidos

**Patrón:** Network Segmentation. **Responsable:** David Fernando Benjumea Mora.

Esta matriz es el contrato de la red de AGORA: todo flujo que no aparece aquí debe quedar **bloqueado**. Se aplica en
`docker-compose.yml` (sección REDES) y se comprueba con [`verify-segmentation.sh`](verify-segmentation.sh). Las URLs de
cada flujo salen de `docker-compose.yml`, `agr-gateway-or/src/config.ts` y `agr-web-fr/lib/config.ts`.

Las filas marcadas con **(S2)** entran cuando se integra el reverse proxy `agr-proxy-rp`.

## 1. Zonas de red

| Zona | Red | `internal` | Miembros |
|---|---|---|---|
| Pública **(S2)** | `agr-public-net` | no (publica 80 y 443) | `agr-proxy-rp` |
| Borde | `agr-edge-net` | no; **sí** con S2, cuando la web y el gateway dejan de publicar puertos | `agr-web-fr`, `agr-gateway-or`, `agr-admin-fr` y, con S2, `agr-proxy-rp` |
| Lógica | `agr-back-net` | sí | `agr-gateway-or`, `agr-auth-lo`, `agr-voter-lo`, `agr-election-lo`, `agr-vote-lo`, `agr-results-lo`, `agr-audit-lo` |
| Datos | `agr-auth-data-net` | sí | `agr-auth-lo`, `agr-auth-db` |
| Datos | `agr-elections-data-net` | sí | `agr-election-lo`, `agr-elections-db` |
| Datos | `agr-voters-data-net` | sí | `agr-voter-lo`, `agr-voters-db` |
| Datos | `agr-votes-data-net` | sí | `agr-vote-lo`, `agr-votes-db` |
| Datos | `agr-results-data-net` | sí | `agr-results-lo`, `agr-results-db` |
| Datos | `agr-audit-data-net` | sí | `agr-audit-lo`, `agr-audit-db` |
| Mensajería | `agr-bus-net` | sí | `agr-broker-as`, `agr-vote-lo`, `agr-results-lo`, `agr-audit-lo` |
| Administración | `agr-mgmt-net` | no (publica `127.0.0.1:15672`) | `agr-broker-as` |

## 2. Redes de cada contenedor

| Contenedor | `networks:` |
|---|---|
| `agr-proxy-rp` **(S2)** | `[agr-public-net, agr-edge-net]` |
| `agr-web-fr` | `[agr-edge-net]` |
| `agr-admin-fr` | `[agr-edge-net]` |
| `agr-gateway-or` | `[agr-edge-net, agr-back-net]` |
| `agr-auth-lo` | `[agr-back-net, agr-auth-data-net]` |
| `agr-auth-db` | `[agr-auth-data-net]` |
| `agr-election-lo` | `[agr-back-net, agr-elections-data-net]` |
| `agr-elections-db` | `[agr-elections-data-net]` |
| `agr-voter-lo` | `[agr-back-net, agr-voters-data-net]` |
| `agr-voters-db` | `[agr-voters-data-net]` |
| `agr-vote-lo` | `[agr-back-net, agr-votes-data-net, agr-bus-net]` |
| `agr-votes-db` | `[agr-votes-data-net]` |
| `agr-broker-as` | `[agr-bus-net, agr-mgmt-net]` |
| `agr-results-lo` | `[agr-back-net, agr-results-data-net, agr-bus-net]` |
| `agr-results-db` | `[agr-results-data-net]` |
| `agr-audit-lo` | `[agr-back-net, agr-audit-data-net, agr-bus-net]` |
| `agr-audit-db` | `[agr-audit-data-net]` |

## 3. Flujos permitidos

| # | Origen | Destino | Puerto | Protocolo | Red | Para qué |
|---|---|---|---|---|---|---|
| F1 | Navegador (host) | `agr-web-fr`; con S2, `agr-proxy-rp` | 3000; con S2, 443 (80 redirige) | HTTP; con S1, HTTPS | publicado | Interfaz web |
| F2 | Navegador (host) | `agr-gateway-or`; con S2, `agr-proxy-rp` (`/api/`) | 8080; con S2, 443 | HTTP / SSE; con S1, HTTPS | publicado | Votar y resultados en vivo |
| F3 | `agr-web-fr` (SSR) | `agr-gateway-or` | 8080 | HTTP | `agr-edge-net` | Render en el servidor |
| F4 | `agr-admin-fr` | `agr-gateway-or` | 8080 | HTTP | `agr-edge-net` | CLI de administración |
| F5 | `agr-gateway-or` | `agr-auth-lo` | 8081 | HTTP | `agr-back-net` | Login y validación del JWT |
| F6 | `agr-gateway-or` | `agr-voter-lo` | 8001 | HTTP | `agr-back-net` | Marcar participación (saga) |
| F7 | `agr-gateway-or` | `agr-election-lo` | 8000 | HTTP | `agr-back-net` | Elecciones y candidatos |
| F8 | `agr-gateway-or` | `agr-vote-lo` | 8002 | HTTP | `agr-back-net` | Registrar voto (saga) |
| F9 | `agr-gateway-or` | `agr-results-lo` | 8004 | HTTP / SSE | `agr-back-net` | Resultados |
| F10 | `agr-gateway-or` | `agr-audit-lo` | 8005 | HTTP | `agr-back-net` | Auditoría |
| F11 | `agr-auth-lo` | `agr-auth-db` | 5432 | PostgreSQL | `agr-auth-data-net` | Credenciales |
| F12 | `agr-election-lo` | `agr-elections-db` | 5432 | PostgreSQL | `agr-elections-data-net` | Elecciones |
| F13 | `agr-voter-lo` | `agr-voters-db` | 5432 | PostgreSQL | `agr-voters-data-net` | `ballots_cast` |
| F14 | `agr-vote-lo` | `agr-votes-db` | 27017 | MongoDB | `agr-votes-data-net` | Votos anónimos y outbox |
| F15 | `agr-results-lo` | `agr-results-db` | 6379 | Redis | `agr-results-data-net` | Conteos |
| F16 | `agr-audit-lo` | `agr-audit-db` | 27017 | MongoDB | `agr-audit-data-net` | Eventos de auditoría |
| F17 | `agr-vote-lo` | `agr-broker-as` | 5672 | AMQP | `agr-bus-net` | Publica `VoteRegistered` |
| F18 | `agr-results-lo` | `agr-broker-as` | 5672 | AMQP | `agr-bus-net` | Consume `results.vote-registered` |
| F19 | `agr-audit-lo` | `agr-broker-as` | 5672 | AMQP | `agr-bus-net` | Consume `audit.vote-registered` |
| F20 | Operador (solo `localhost`) | `agr-broker-as` | 15672 | HTTP | `agr-mgmt-net` | Consola de RabbitMQ |
| F21 **(S2)** | `agr-proxy-rp` | `agr-web-fr` | 3000 | HTTP | `agr-edge-net` | Reenvío de `/` |
| F22 **(S2)** | `agr-proxy-rp` | `agr-gateway-or` | 8080 | HTTP / SSE | `agr-edge-net` | Reenvío de `/api/` |

## 4. Flujos que deben quedar bloqueados

| # | Origen | Destino | Prototipo 2 | Prototipo 3 | Por qué importa |
|---|---|---|---|---|---|
| B1 | `agr-vote-lo` | `agr-voters-db` | alcanzable | **bloqueado** | Cruce votante ↔ voto (anonimato) |
| B2 | `agr-voter-lo` | `agr-votes-db` | alcanzable | **bloqueado** | Cruce votante ↔ voto (anonimato) |
| B3 | Cualquier `*-lo` distinto de auth | `agr-auth-db` | alcanzable | **bloqueado** | Robo de credenciales |
| B4 | `agr-*-lo` y `agr-*-db` | Internet | alcanzable | **bloqueado** | Exfiltración o descarga de malware |
| B5 | `agr-web-fr` | Cualquier `*-lo`, `*-db` o el broker | no | **bloqueado** | El borde solo ve al gateway |
| B6 | `agr-gateway-or` | Cualquier `*-db` o el broker | no | **bloqueado** | El gateway no toca datos |
| B7 | `agr-election-lo`, `agr-voter-lo`, `agr-auth-lo` | `agr-broker-as` | alcanzable | **bloqueado** | Solo los clientes reales del bus |
| B8 | `agr-*-lo` | Otro `agr-*-lo` | alcanzable | alcanzable ⚠ | Comparten `agr-back-net` |
| B9 | Otra máquina de la red local | Consola 15672 | alcanzable | **bloqueado** | Administración expuesta |
| B10 **(S2)** | `agr-proxy-rp` | Cualquier `*-lo`, `*-db` o el broker | — | **bloqueado** | El proxy solo ve la web y el gateway |
| B11 **(S2)** | Navegador (host) | `agr-web-fr:3000` y `agr-gateway-or:8080` | alcanzable | **bloqueado** | Solo se entra por el proxy |

> ⚠ **B8: riesgo residual aceptado.** Separar cada servicio del gateway en su propia red exigiría seis redes más. Los
> servicios no se llaman entre sí y cada uno exige `SERVICE_API_KEY` o la llave de administración.

> **Puertos publicados (celda P de la matriz).** Un contenedor que está en una red no `internal` (en AGORA,
> `agr-broker-as` por `agr-mgmt-net`) puede llegar por IP a un puerto que otro contenedor publica en el host, porque
> el tráfico sale por el host y entra por la misma puerta que usa un cliente externo. No es un camino hacia una zona
> protegida, así que se cuenta aparte. En Docker Desktop aparece como `agr-broker-as → agr-web-fr:3000` y
> `→ agr-gateway-or:8080`; con S2 la web y el gateway dejan de publicar puertos y solo queda el proxy, que es la
> entrada pública.

## 5. Decisiones de diseño

1. **Una red de datos por servicio** en lugar de una sola `agr-data`: *database-per-service* también en la red.
2. **`internal: true` en toda red que no publica puertos.** Docker no le da salida a internet ni al host.
3. **`name:` explícito en cada red**, para que se llamen igual en cualquier máquina (`agr-edge-net` y no
   `arquisoft_agr-edge-net`) y la evidencia coincida.
4. **La consola de RabbitMQ en una red propia** (`agr-mgmt-net`) y publicada solo en `127.0.0.1`.
5. **Con S2, el proxy es el único contenedor con puertos públicos** y `agr-edge-net` pasa a `internal`, así la web y el
   gateway pierden la salida a internet.
