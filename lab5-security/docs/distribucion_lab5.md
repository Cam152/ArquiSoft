# Lab 5 — Network Segmentation · Equipo 1b

| Entrega | Rama | Ya está hecho | Ahora toca |
|---|---|---|---|
| `l5_1b.pdf` en MiCampus · **lun 9 nov, antes de las 23:59** | `lab5-security` | Diseño (`design/flujos-permitidos.md`) y redes migradas en `docker-compose.yml` | **Revisar → probar → documentar** |

---

## 1. Flujo de trabajo

| Paso | Qué | Quién | Cuándo |
|:-:|---|---|---|
| **0** | Preparar el entorno (comandos abajo) | Todos | Ya |
| **1** | Revisar que **mis** contenedores estén en sus redes (tabla 3) | Todos | Lunes, hasta las 11:00 |
| **2** | Hacer **mis** pruebas y guardar las evidencias (tabla 4) | Todos | 11:00 – 16:00 |
| **3** | Escribir **mi** sección de `l5_1b.md` (tabla 5) | Todos | 16:00 – 20:00 |
| **4** | Integrar, revisar, exportar el PDF y entregar | Sebastián (todos leen) | 20:00 – 23:59 |

## 2. Comandos

| Para qué | Comando |
|---|---|
| Preparar | `git checkout lab5-security && git pull` |
| Levantar limpio | `docker compose down -v && docker compose up --build -d --wait` |
| Ver redes de un contenedor | `docker inspect <c> --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}'` |
| ¿Resuelve el nombre? | `docker run --rm --network container:<origen> busybox nslookup <destino>` |
| ¿Conecta al puerto? | `docker run --rm --network container:<origen> busybox nc -zv -w 2 <destino> <puerto>` |
| ¿Sale a internet? | `docker run --rm --network container:<origen> busybox wget -q -T 3 -O /dev/null http://example.com` |
| Ver qué falló | `docker compose logs <servicio> --tail 50` |

> Si sale `permission denied ... docker.sock`, una sola vez: `sudo usermod -aG docker $USER && newgrp docker`.

---

## 3. Paso 1 — Redes esperadas

El resultado de `docker inspect` debe coincidir **exactamente**.

| Responsable | Contenedor | Redes esperadas | Puerto publicado |
|---|---|---|---|
| **Sebastián** | `agr-vote-lo` | `agr-back-net agr-bus-net agr-votes-data-net` | — |
| | `agr-votes-db` | `agr-votes-data-net` | — |
| | `agr-broker-as` | `agr-bus-net agr-mgmt-net` | `127.0.0.1:15672` |
| **David** | `agr-gateway-or` | `agr-back-net agr-edge-net` | `8080` |
| | `agr-election-lo` | `agr-back-net agr-elections-data-net` | — |
| | `agr-elections-db` | `agr-elections-data-net` | — |
| | `agr-admin-fr` | `agr-edge-net` (revisar en el compose) | — |
| **Esteban** | `agr-auth-lo` | `agr-auth-data-net agr-back-net` | — |
| | `agr-auth-db` | `agr-auth-data-net` | — |
| **Camilo** | `agr-voter-lo` | `agr-back-net agr-voters-data-net` | — |
| | `agr-voters-db` | `agr-voters-data-net` | — |
| | `agr-audit-lo` | `agr-audit-data-net agr-back-net agr-bus-net` | — |
| | `agr-audit-db` | `agr-audit-data-net` | — |
| **Miguel** | `agr-results-lo` | `agr-back-net agr-bus-net agr-results-data-net` | — |
| | `agr-results-db` | `agr-results-data-net` | — |
| **Cristian** | `agr-web-fr` | `agr-edge-net` | `3000` |

---

## 4. Paso 2 — Pruebas y evidencias

"Antes" = rama `main` (Prototipo 2) · "Después" = rama `lab5-security`.

| Responsable | Caso | Prueba | Esperado (después) | Evidencia |
|---|:-:|---|---|---|
| **Camilo** ⭐ | B1 | `agr-vote-lo` → `agr-voters-db`, antes y después | No resuelve | `B1_antes.png`, `B1_despues.png` |
| | B2 | `agr-voter-lo` → `agr-votes-db`, antes y después | No resuelve | `B2_antes.png`, `B2_despues.png` |
| **Esteban** | B3 | Todos los `*-lo` salvo auth → `agr-auth-db` | No resuelve | `B3_auth-db-aislada.png` |
| | B4 | Cualquier `*-lo` → internet | Falla | `B4_sin-internet.png` |
| | — | **Código:** `healthcheck` y `restart` en `agr-auth-lo`; gateway con `service_healthy` (PR) | auth sale `healthy` | — |
| **Cristian** | B5 | `agr-web-fr` → cualquier `*-lo`, `*-db` o broker | No resuelve | `B5_web-solo-ve-gateway.png` |
| **David** | B6 | `agr-gateway-or` → cualquier `*-db` o broker | No resuelve | `B6_gateway-no-ve-bases.png` |
| | E2E | Login → votar → resultados en vivo → `/admin/audit` → CLI `elections list` | Todo funciona | `E2E_votacion-completa.png` |
| **Miguel** | Matriz | `scripts/verify-segmentation.sh` (origen × destino), en `main` y en `lab5-security` | Solo flujos F1–F20 abiertos | `evidence/before/matriz.txt`, `evidence/after/matriz.txt` |
| | B7 | election, voter y auth → `agr-broker-as` | No resuelve | `B7_broker-aislado.png` |
| | B8 | Servicio → servicio (riesgo aceptado) | Sigue abierto ⚠ | En la matriz |
| **Sebastián** | B9 | Consola en `127.0.0.1:15672` vs `<ip-lan>:15672` | Solo local | `B9_consola-solo-local.png` |
| | — | `./agr-vote-lo/e2e/run.sh` | Pasa | — |
| **Cristian** | Vista | Despliegue con **cada red como un recuadro**, solo las conexiones permitidas, puertos y convención `internal` | — | `images/diagrams/l5_despliegue.png` y `.svg` |

### Dónde guardar (dentro de `lab5-security/`)

| Tipo | Carpeta |
|---|---|
| Capturas (`<caso>_<descripcion>.png`) | `images/screenshots/` |
| Diagramas | `images/diagrams/` |
| Salidas de texto | `evidence/before/` · `evidence/after/` |

---

## 5. Paso 3 — Entregable (`l5_1b.md`) = lo que pide el profesor

| Pide el enunciado (3.1) | Sección | Quién | Se basa en |
|---|:-:|---|---|
| Nombre completo | §1 | Sebastián | Ya está |
| Vista arquitectónica donde se evidencie el patrón | §2 | Cristian | `l5_despliegue.png` |
| Descripción del patrón y su propósito | §3.1 | Esteban | Teoría + Docker (`internal`, DNS por red) |
| Escenario de calidad | §3.2 | Sebastián | `design/flujos-permitidos.md` §1 |
| Pasos de implementación | §3.3 | David | Lo hecho, paso a paso, con comandos |
| Configuración y código | §3.4 | Camilo | `networks:` antes/después, puerto `127.0.0.1` |
| Resultados y mejoras | §3.5 | Miguel | Matriz antes/después y capturas B1–B9 |
| Recomendaciones para otros equipos | §3.6 | Cristian | Errores típicos y riesgo B8 |
| `.md` → `l5_1b.pdf`, lo sube un solo integrante | — | Sebastián | — |

---

## 6. Reglas

| | |
|---|---|
| ✏️ | Cada uno edita **solo su sección** de `l5_1b.md` (PR desde `lab5/<nombre>` o enviándole el texto a Sebastián) |
| 🖼️ | Las imágenes van con ruta relativa: `images/screenshots/B1_despues.png` |
| 🔒 | Las redes no se cambian sin avisar; si algo no coincide, se reporta en el grupo |
| 🆘 | Si algo falla, avisar de inmediato con `docker compose logs <servicio> --tail 50` |
