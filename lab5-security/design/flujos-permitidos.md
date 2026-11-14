# Diseño de la segmentación — Flujos permitidos (Fase 1.1)

**Responsable:** Maicol Sebastián Olarte Ramírez (arquitecto líder)
**Patrón:** Network Segmentation
**Fuente:** comunicaciones reales del Prototipo 2. Las URLs se tomaron de `docker-compose.yml`, `agr-gateway-or/src/config.ts` y `agr-web-fr/lib/config.ts`.

Esta matriz es el **contrato** de la Fase 2: todo flujo que no aparece aquí debe quedar **bloqueado** por la red.

---

## 1. Escenario de calidad

| Elemento | Valor |
|---|---|
| **Atributo** | Confidencialidad (anonimato del voto) |
| **Fuente** | Atacante que compromete un servicio interno, p. ej. `agr-vote-lo` (por una vulnerabilidad o dependencia comprometida) |
| **Estímulo** | Desde ese contenedor intenta resolver y conectarse a bases o servicios que no le corresponden (`agr-voters-db`, `agr-auth-db`, `agr-election-lo`, …) o salir a internet |
| **Artefacto** | Red de despliegue de AGORA (redes de Docker Compose) |
| **Ambiente** | Operación normal, con una elección abierta |
| **Respuesta** | El nombre del destino no resuelve o la conexión no se establece. El compromiso queda contenido en el segmento del servicio |
| **Medida** | **0** flujos no autorizados en la matriz de alcanzabilidad (frente a los N de la línea base). **100 %** de los flujos de negocio siguen funcionando: login, votar, resultados en vivo, auditoría y CLI |

**Riesgo que se mitiga:** el README (§9) advierte que quien acceda a la vez a `agr-voters-db` (quién votó) y a `agr-votes-db` (los votos) podría cruzar el orden de inserción y desanonimizar votos. Hoy ambas bases están en la misma red plana (`agr-data`) junto con todos los servicios.

---

## 2. Zonas de red

| Zona | Red | `internal` | Miembros |
|---|---|---|---|
| Borde | `agr-edge-net` | no (publica 3000 y 8080) | `agr-web-fr`, `agr-gateway-or`, `agr-admin-fr` |
| Lógica | `agr-back-net` | **sí** | `agr-gateway-or`, `agr-auth-lo`, `agr-voter-lo`, `agr-election-lo`, `agr-vote-lo`, `agr-results-lo`, `agr-audit-lo` |
| Datos | `agr-auth-data-net` | **sí** | `agr-auth-lo`, `agr-auth-db` |
| Datos | `agr-elections-data-net` | **sí** | `agr-election-lo`, `agr-elections-db` |
| Datos | `agr-voters-data-net` | **sí** | `agr-voter-lo`, `agr-voters-db` |
| Datos | `agr-votes-data-net` | **sí** | `agr-vote-lo`, `agr-votes-db` |
| Datos | `agr-results-data-net` | **sí** | `agr-results-lo`, `agr-results-db` |
| Datos | `agr-audit-data-net` | **sí** | `agr-audit-lo`, `agr-audit-db` |
| Mensajería | `agr-bus-net` | **sí** | `agr-broker-as`, `agr-vote-lo`, `agr-results-lo`, `agr-audit-lo` |
| Administración | `agr-mgmt-net` | no (publica `127.0.0.1:15672`) | `agr-broker-as` |

### Redes de cada contenedor (lo que se aplica en la Fase 2)

| Contenedor | `networks:` | Responsable |
|---|---|---|
| `agr-web-fr` | `[agr-edge-net]` | Cristian |
| `agr-admin-fr` | `[agr-edge-net]` | David |
| `agr-gateway-or` | `[agr-edge-net, agr-back-net]` | David |
| `agr-auth-lo` | `[agr-back-net, agr-auth-data-net]` | Esteban |
| `agr-auth-db` | `[agr-auth-data-net]` | Esteban |
| `agr-election-lo` | `[agr-back-net, agr-elections-data-net]` | David |
| `agr-elections-db` | `[agr-elections-data-net]` | David |
| `agr-voter-lo` | `[agr-back-net, agr-voters-data-net]` | Camilo |
| `agr-voters-db` | `[agr-voters-data-net]` | Camilo |
| `agr-vote-lo` | `[agr-back-net, agr-votes-data-net, agr-bus-net]` | Sebastián |
| `agr-votes-db` | `[agr-votes-data-net]` | Sebastián |
| `agr-broker-as` | `[agr-bus-net, agr-mgmt-net]` | Sebastián |
| `agr-results-lo` | `[agr-back-net, agr-results-data-net, agr-bus-net]` | Miguel |
| `agr-results-db` | `[agr-results-data-net]` | Miguel |
| `agr-audit-lo` | `[agr-back-net, agr-audit-data-net, agr-bus-net]` | Camilo |
| `agr-audit-db` | `[agr-audit-data-net]` | Camilo |

---

## 3. Matriz de flujos permitidos

Solo estos flujos deben funcionar después de la segmentación.

| # | Origen | Destino | Puerto | Protocolo | Red | Para qué |
|---|---|---|---|---|---|---|
| F1 | Navegador (host) | `agr-web-fr` | 3000 | HTTP | publicado | Interfaz web |
| F2 | Navegador (host) | `agr-gateway-or` | 8080 | HTTP / SSE | publicado | Votar y resultados en vivo |
| F3 | `agr-web-fr` (SSR) | `agr-gateway-or` | 8080 | HTTP | `agr-edge-net` | Render en el servidor |
| F4 | `agr-admin-fr` | `agr-gateway-or` | 8080 | HTTP | `agr-edge-net` | CLI de administración |
| F5 | `agr-gateway-or` | `agr-auth-lo` | 8081 | HTTP | `agr-back-net` | Login / JWT |
| F6 | `agr-gateway-or` | `agr-voter-lo` | 8001 | HTTP | `agr-back-net` | Marcar participación (saga) |
| F7 | `agr-gateway-or` | `agr-election-lo` | 8000 | HTTP | `agr-back-net` | Elecciones y candidatos |
| F8 | `agr-gateway-or` | `agr-vote-lo` | 8002 | HTTP | `agr-back-net` | Registrar voto (saga) |
| F9 | `agr-gateway-or` | `agr-results-lo` | 8004 | HTTP / SSE | `agr-back-net` | Resultados |
| F10 | `agr-gateway-or` | `agr-audit-lo` | 8005 | HTTP | `agr-back-net` | `GET /admin/audit` |
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

---

## 4. Flujos que deben quedar bloqueados (casos de prueba de la Fase 4)

| # | Origen | Destino | Hoy (Prototipo 2) | Después | Por qué importa | Prueba |
|---|---|---|---|---|---|---|
| B1 | `agr-vote-lo` | `agr-voters-db` | alcanzable | **bloqueado** | Cruce votante ↔ voto (anonimato) | Camilo |
| B2 | `agr-voter-lo` | `agr-votes-db` | alcanzable | **bloqueado** | Cruce votante ↔ voto (anonimato) | Camilo |
| B3 | Cualquier `*-lo` ≠ auth | `agr-auth-db` | alcanzable | **bloqueado** | Robo de credenciales | Esteban |
| B4 | `agr-*-lo` | Internet (p. ej. `example.com`) | alcanzable | **bloqueado** | Exfiltración / descarga de malware | Esteban |
| B5 | `agr-web-fr` | Cualquier `*-lo` o `*-db` | no (ya estaba) | **bloqueado** | El borde solo ve al gateway | Cristian |
| B6 | `agr-gateway-or` | Cualquier `*-db` o broker | no (ya estaba) | **bloqueado** | El gateway no toca datos | David |
| B7 | `agr-election-lo`, `agr-voter-lo`, `agr-auth-lo` | `agr-broker-as` | alcanzable | **bloqueado** | Solo los clientes reales del bus | Miguel |
| B8 | `agr-*-lo` | Otro `agr-*-lo` (servicio a servicio) | alcanzable | **alcanzable** ⚠ | Comparten `agr-back-net` | Miguel |
| B9 | Red local (otra máquina) | Consola 15672 | alcanzable | **bloqueado** | Administración expuesta | Sebastián |

> ⚠ **B8: riesgo residual aceptado.** Separar cada servicio del gateway en su propia red exigiría 6 redes más. Hoy los servicios no se llaman entre sí y cada uno exige `SERVICE_API_KEY` o la llave de administración. Esto se documenta en las recomendaciones de la guía.

---

## 5. Decisiones de diseño

1. **Una red de datos por servicio** en lugar de una sola `agr-data`. Así se cumple *database-per-service* también a nivel de red, no solo de credenciales.
2. **`internal: true` en todas las redes que no publican puertos.** Docker no les da gateway hacia el host ni salida a internet.
3. **`name:` explícito en cada red.** Así se llaman `agr-edge-net`, etc., y no `arquisoft_agr-edge-net`, lo que hace legibles `docker network ls` y la evidencia.
4. **La consola de RabbitMQ va en una red propia (`agr-mgmt-net`) y se publica solo en `127.0.0.1`.** El tráfico AMQP (`agr-bus-net`) queda aislado del tráfico de administración.
5. **Migración gradual.** Las redes viejas conviven con las nuevas durante la Fase 2 y se eliminan en la Fase 3.
