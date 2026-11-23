<p align="center">
  <img src="artifacts/AGORA_logo.png" alt="AGORA, sistema de votación electrónica" width="320">
</p>

# AGORA: sistema de votación electrónica (Prototipo 3)

**AGORA** es un sistema distribuido de votación electrónica para elecciones estudiantiles. Un votante entra a
`https://localhost`, inicia sesión con su documento y su clave, ve las elecciones abiertas, vota **una vez en cada
elección** y sigue los resultados, que se actualizan solos mientras la votación está abierta. Quien administra crea,
abre y cierra elecciones desde una consola de comandos. El voto es **anónimo**: el sistema sabe quién votó y qué se
votó, pero ningún dato permite unir las dos cosas.

Proyecto del equipo **1b** para el curso Arquitectura de Software (UNAL, 2026-II, prof. Jeisson Andrés Vergara Vargas).
El Prototipo 3 rediseña la arquitectura con foco en **seguridad** y **rendimiento**. El artefacto de la entrega es
[`artifacts/p3_1b.md`](artifacts/p3_1b.md) (exportado como `p3_1b.pdf`).

## Contenido

1. [Qué hace el sistema](#1-qué-hace-el-sistema)
2. [Arquitectura](#2-arquitectura)
3. [Seguridad](#3-seguridad)
4. [Rendimiento y escalabilidad](#4-rendimiento-y-escalabilidad)
5. [Cómo ejecutarlo](#5-cómo-ejecutarlo)
6. [CLI de administración](#6-cli-de-administración)
7. [API del gateway](#7-api-del-gateway)
8. [Cómo funciona un voto](#8-cómo-funciona-un-voto)
9. [Variables de entorno](#9-variables-de-entorno)
10. [Pruebas e integración continua](#10-pruebas-e-integración-continua)
11. [Problemas frecuentes](#11-problemas-frecuentes)
12. [Estructura del repositorio](#12-estructura-del-repositorio)
13. [Equipo](#13-equipo)

---

## 1. Qué hace el sistema

| Actor | Función | Dónde |
|---|---|---|
| Votante | Iniciar sesión con documento y clave (token JWT de 1 hora) | Web |
| Votante | Ver las elecciones abiertas y sus candidatos | Web |
| Votante | Votar una vez por elección | Web |
| Votante | Ver los resultados en vivo | Web |
| Administrador | Crear, editar y eliminar elecciones en borrador; agregar y quitar candidatos | CLI |
| Administrador | Abrir (con al menos dos candidatos) y cerrar elecciones | CLI |
| Administrador | Consultar resultados y la auditoría, y **verificar que la auditoría no fue alterada** | CLI |
| Sistema | **Bloquear 15 minutos** una cuenta tras cinco claves erradas y **limitar los intentos por IP** | Proxy y `agr-auth-lo` |

Dos garantías guían el diseño:

- **Una persona, un voto por elección.** La participación se guarda con la llave (`voter_id`, `election_id`); el
  segundo intento en la misma elección se rechaza con 409.
- **Voto anónimo.** *Quién votó* (`agr-voters-db`) y *qué se votó* (`agr-votes-db`) están en bases distintas, de
  servicios distintos y en redes distintas. El voto solo guarda la elección, el candidato y la hora redondeada, con un
  identificador aleatorio.

## 2. Arquitectura

### 2.1 Componentes

17 componentes en contenedores: 9 propios, en seis lenguajes, y 8 de software existente. Siguen la convención
`agr-<nombre>-<nivel>`: `-rp` borde, `-fr` presentación, `-or` orquestación, `-lo` lógica, `-as` asincronía y
`-db` datos. El nombre es el mismo de la carpeta y del servicio en `docker-compose.yml`.

| Componente | Tecnología | Puerto interno | Responsable | Qué hace |
|---|---|---|---|---|
| `agr-proxy-rp` | Nginx (Alpine 3.22) | 443, 80 | Cristian (S2), Miguel (S1), Esteban (S4) | Única entrada: TLS, enrutamiento, encabezados de seguridad, límites |
| `agr-web-fr` | TypeScript, Next.js (SSR) | 3000 | Cristian | Aplicación web del votante |
| `agr-admin-fr` | Go, Cobra | — (bajo demanda) | Maicol | CLI `agora` del administrador |
| `agr-gateway-or` | TypeScript, Node.js 22 | 8080 | David | API Gateway, BFF y saga del voto; sin estado (1 a N réplicas) |
| `agr-auth-lo` | Java 21, Spring Boot | 8081 | Esteban | Inicio de sesión, JWT y bloqueo de cuentas |
| `agr-voter-lo` | Java 21, Spring Boot | 8001 | Camilo | Participación: quién ya votó en qué elección |
| `agr-election-lo` | Python, FastAPI, Strawberry | 8000 | David | Elecciones y candidatos (REST y GraphQL) |
| `agr-vote-lo` | Go | 8002 | Maicol | Voto anónimo y evento `VoteRegistered` (outbox) |
| `agr-results-lo` | Rust (axum) | 8004 | Miguel | Conteo y resultados en vivo (SSE) |
| `agr-audit-lo` | C#, .NET 10 | 8005 | Camilo | Auditoría encadenada con HMAC-SHA256 y su verificación |
| `agr-broker-as` | RabbitMQ 3.13 | 5672, 15672 | Maicol | Exchange `agora.votes` (fanout), colas quorum y cartas muertas |
| `agr-auth-db` | PostgreSQL 16 | 5432 | Esteban | Votantes, claves e intentos fallidos |
| `agr-voters-db` | PostgreSQL 16 | 5432 | Camilo | Participaciones (`ballots_cast`) |
| `agr-elections-db` | PostgreSQL 16 | 5432 | David | Elecciones y candidatos |
| `agr-votes-db` | MongoDB 7 | 27017 | Maicol | Votos anónimos |
| `agr-results-db` | Redis 7 | 6379 | Miguel | Conteos |
| `agr-audit-db` | MongoDB 7 | 27017 | Camilo | Eventos de auditoría |

### 2.2 Vista de componentes y conectores

![Vista C&C](artifacts/p3_cc.png)

- El **navegador** solo habla con `agr-proxy-rp` por HTTPS. El proxy envía `/` a la web y `/api/...` al gateway
  (quitando `/api`). Ni la web ni el gateway publican puertos.
- La **web** arma las páginas en su servidor (SSR) llamando al gateway por la red interna. Desde el navegador salen el
  inicio de sesión, el voto y la suscripción a los resultados en vivo, siempre por el proxy.
- La **CLI** llama al gateway por la red interna; las rutas `/admin/...` no se exponen hacia fuera.
- El **gateway** es el único que llama a los servicios de lógica, y los servicios no se llaman entre sí.
- **RabbitMQ** reparte `VoteRegistered` a resultados y auditoría; cada servicio de lógica tiene su propia base.

Las demás vistas (despliegue, capas y descomposición) están en el [artefacto](artifacts/p3_1b.md).

### 2.3 Redes (S3)

| Red | `internal` | Contenedores |
|---|---|---|
| `agr-public-net` | no (publica 443 y 80) | `agr-proxy-rp` |
| `agr-edge-net` | sí | proxy, web, CLI y gateway |
| `agr-back-net` | sí | gateway y los seis servicios de lógica |
| `agr-<x>-data-net` (6) | sí | cada servicio con su base |
| `agr-bus-net` | sí | broker, `agr-vote-lo`, `agr-results-lo`, `agr-audit-lo` |
| `agr-mgmt-net` | no (publica `127.0.0.1:15672`) | `agr-broker-as` |

Solo se publican **443** (HTTPS), **80** (redirige a HTTPS) y **127.0.0.1:15672** (consola de RabbitMQ, solo desde
el mismo equipo). El diseño completo de flujos permitidos está en
[`p3/s3/flujos-permitidos.md`](p3/s3/flujos-permitidos.md).

## 3. Seguridad

| # | Escenario | Patrón | Qué se hizo | Evidencia |
|---|---|---|---|---|
| S1 | Alguien en la red captura el documento, la clave o el token | **Secure Channel** | TLS 1.2/1.3 en el proxy, 80 → 301 a HTTPS, HSTS. Certificado de desarrollo fuera de Git y de la imagen | [`p3/s1/`](p3/s1/) |
| S2 | Un atacante habla directo con la web o el gateway | **Reverse Proxy** | `agr-proxy-rp` como única entrada; oculta `Server`, `X-Powered-By` y `X-Nextjs-*`; seis encabezados de seguridad; máximo 1 MB; `/api/admin` → 404 | [`p3/s2/`](p3/s2/) |
| S3 | Un servicio comprometido alcanza bases ajenas o internet | **Network Segmentation** | Once redes, nueve `internal`. Caminos no autorizados: **108 → 0**; contenedores con internet: **15 → 2** | [`p3/s3/`](p3/s3/) |
| S4 | Fuerza bruta contra el inicio de sesión | **Rate Limiting + Account Lockout** | 5 intentos por minuto por IP en el proxy (429) y bloqueo de la cuenta 15 minutos al quinto fallo en `agr-auth-lo` (423) | [`p3/s4/`](p3/s4/) |
| S5 | Alguien con acceso a la base cambia la auditoría | **Tamper-evident Log** | Cada evento encadenado con HMAC-SHA256; `agora audit verify` detecta **5 de 5** tipos de alteración con el evento exacto | [`p3/s5/`](p3/s5/) |

Cómo repetir cada verificación, con AGORA levantado y desde la raíz del repositorio:

| Escenario | Comando |
|---|---|
| S1 | `powershell -ExecutionPolicy Bypass -File .\p3\s1\verificar-tls.ps1` y `.\p3\s1\capturar-trafico.ps1` |
| S2 | `powershell -ExecutionPolicy Bypass -File .\p3\s2\verificar-proxy.ps1` |
| S3 | `bash p3/s3/verify-segmentation.sh p3/s3/matriz_proxy.txt` (Git Bash) |
| S4 | `bash p3/s4/test-brute-force.sh` (Git Bash; al final desbloquea la cuenta 1001) |
| S5 | `powershell -ExecutionPolicy Bypass -File .\p3\s5\verificar-auditoria.ps1` |

## 4. Rendimiento y escalabilidad

Pruebas de carga con **k6** (corre en un contenedor). Detalle, scripts y resultados en [`p3/perf/`](p3/perf/README.md).

| Prueba | Resultado |
|---|---|
| R1 · Lectura (`GET /elections` + `GET /results/{id}`, 50 a 1500 usuarios) | Hasta **~1050 peticiones/s sin errores**. La rodilla está entre 500 y 1000 usuarios; el límite es `agr-election-lo` |
| R2 · Escritura (inicio de sesión + voto, 10 a 200 usuarios) | **~110 votos/s sin errores**. La saga tarda menos de 40 ms (p95); lo caro es BCrypt en `agr-auth-lo` |
| R3 · Costo de la seguridad (sin proxy contra con proxy) | El proxy cuesta **~2 %** de capacidad. Su primera versión fallaba con 61 % de errores; se corrigió con `keepalive` |
| R4 · Escalabilidad (1 contra 3 réplicas del gateway) | El reparto es parejo (un tercio cada una), pero la capacidad no sube: el gateway no es el cuello de botella |

```bash
p3/perf/run-all.sh        # R1 a R4 completos (~25 min), desde la raíz del repo, en Linux o WSL
```

## 5. Cómo ejecutarlo

### 5.1 Requisitos

- Docker Desktop (o Docker Engine) con Docker Compose v2.
- Git Bash en Windows, para los scripts `.sh`.
- Los puertos 80 y 443 libres. Si el 80 está ocupado, ver la [sección 11](#11-problemas-frecuentes).

### 5.2 Primera vez

```powershell
# 1. Certificado de desarrollo de S1 (una sola vez por equipo; queda en p3/s1/certs, fuera de Git)
& "C:\Program Files\Git\bin\bash.exe" p3/s1/gen-certs.sh
#    Sin Git Bash, el mismo script corre dentro de Docker:
#    docker run --rm -v "${PWD}:/repo" -w /repo alpine sh -c "apk add --no-cache openssl bash >/dev/null && bash p3/s1/gen-certs.sh"

# 2. Construir y levantar todo (espera a que los 16 contenedores estén sanos)
docker compose up -d --build --wait

# 3. Comprobar
docker compose ps
```

Luego se abre **https://localhost**. El navegador avisa que el certificado no es de confianza porque es autofirmado:
se acepta en "Avanzado" → "Continuar". El tráfico va cifrado igual.

### 5.3 Datos de prueba

| Qué | Valor |
|---|---|
| Votantes | Documentos **1001** a **1005**, todos con la clave **123456** |
| Elección abierta | 1 · "Elección de representante estudiantil 2026" (Ana Torres, Luis Herrera, Camila Duarte) |
| Elección en borrador | 2 · "Consulta sobre el horario de laboratorios" |
| Elección cerrada | 3 · "Elección de monitores 2025-II" |
| Llave de administración | `dev-admin-key` (la usa la CLI) |
| Consola de RabbitMQ | http://127.0.0.1:15672, usuario `agora`, clave `agora` |

### 5.4 Operación diaria

```powershell
docker compose up -d --wait            # levantar (sin reconstruir)
docker compose logs -f agr-proxy-rp    # ver las peticiones que llegan
docker compose down                    # detener, conservando los datos
docker compose down -v                 # detener y BORRAR los datos (vuelven los datos de prueba al levantar)
docker compose up -d --wait --scale agr-gateway-or=3   # gateway con 3 réplicas detrás del proxy
```

## 6. CLI de administración

La CLI `agora` corre en su propio contenedor (perfil `tools`), por la red interna:

```powershell
docker compose run --rm agr-admin-fr <comando>
```

| Comando | Qué hace |
|---|---|
| `elections list [--status draft\|active\|closed]` | Lista las elecciones |
| `elections show <id>` | Muestra una elección con sus candidatos |
| `elections create --name "..." [--description "..."] [--starts ...] [--ends ...]` | Crea una elección en borrador |
| `elections update <id> [--name ...] [--description ...]` | Edita una elección en borrador |
| `elections delete <id> [-y]` | Elimina una elección en borrador |
| `elections open <id>` / `elections close <id>` | Abre (requiere 2 candidatos) o cierra la votación |
| `candidates add --election <id> --name "..." [--description "..."]` | Agrega un candidato (solo en borrador) |
| `candidates remove --election <id> --candidate <id>` | Quita un candidato (solo en borrador) |
| `results show <id>` | Resultados de una elección |
| `audit list [--election <id>] [--limit 50]` | Eventos de auditoría |
| `audit verify` | Verifica la cadena de auditoría (S5); termina con código 1 si fue alterada |

Todos aceptan `--json`. Ejemplo completo:

```powershell
docker compose run --rm agr-admin-fr elections create --name "Elección 2026" --description "Prueba"
docker compose run --rm agr-admin-fr candidates add --election 4 --name "Ana Torres"
docker compose run --rm agr-admin-fr candidates add --election 4 --name "Luis Herrera"
docker compose run --rm agr-admin-fr elections open 4
docker compose run --rm agr-admin-fr audit verify
```

## 7. API del gateway

Desde fuera se llega por `https://localhost/api/...`; dentro de Docker, por `http://agr-gateway-or:8080/...`.

| Método y ruta | Quién la usa | Qué hace |
|---|---|---|
| `POST /auth/login` | Navegador | Documento y clave → JWT. 401 clave errada, **423 cuenta bloqueada**, **429 demasiados intentos (proxy)** |
| `POST /graphql`, `GET /graphql` | Web (SSR) | Elecciones con sus candidatos |
| `GET /elections`, `GET /elections/{id}` | Web, CLI | Elecciones |
| `POST /votes` | Navegador | Votar (JWT en `Authorization`). 201, 409 si ya votó |
| `GET /results/{id}` | Web, CLI | Conteos con nombres y porcentajes |
| `GET /results/{id}/stream` | Navegador | Resultados en vivo (SSE) |
| `POST /admin/elections`, `PATCH`/`DELETE /admin/elections/{id}` | CLI | Administración de elecciones (`X-API-Key`) |
| `POST /admin/elections/{id}/candidates`, `DELETE .../candidates/{cid}` | CLI | Candidatos |
| `POST /admin/elections/{id}/activate`, `.../close` | CLI | Abrir y cerrar |
| `GET /admin/audit`, `GET /admin/audit/verify` | CLI | Auditoría y su verificación |
| `GET /health` | Docker | Healthcheck |

Las rutas `/admin/...` solo funcionan por la red interna: en el proxy, `/api/admin/...` responde 404.

## 8. Cómo funciona un voto

1. El navegador envía `POST /api/votes` con el JWT. El proxy lo reenvía al gateway como `POST /votes`.
2. El gateway pide a `agr-auth-lo` validar el token, comprueba en `agr-election-lo` que la elección esté abierta y el
   candidato le pertenezca, marca la participación en `agr-voter-lo` (409 si ya votó) y guarda el voto en
   `agr-vote-lo` con una `Idempotency-Key`.
3. Si guardar el voto falla de forma confirmada, el gateway **compensa** desmarcando la participación (saga).
4. `agr-vote-lo` publica `VoteRegistered` en el exchange `agora.votes` (con outbox, por si el broker no está).
5. RabbitMQ entrega una copia a `agr-results-lo`, que suma el voto en Redis y lo transmite por SSE, y otra a
   `agr-audit-lo`, que lo guarda encadenado al evento anterior. Ambos ignoran eventos repetidos (`event_id`).

## 9. Variables de entorno

Todas tienen un valor por defecto en `docker-compose.yml`; para cambiarlas se copia `.env.example` a `.env`.

| Variable | Por defecto | Uso |
|---|---|---|
| `ADMIN_API_KEY` | `dev-admin-key` | Llave de administración (gateway, election y CLI) |
| `SERVICE_API_KEY` | `dev-service-key` | Llave entre el gateway y los servicios |
| `AUDIT_CHAIN_KEY` | `dev-audit-chain-key-change-me-...` | Llave del HMAC de la auditoría (S5); mínimo 32 caracteres, solo la recibe `agr-audit-lo` |
| `LOGIN_RATE`, `LOGIN_BURST` | `5r/m`, `5` | Límite de inicio de sesión por IP en el proxy (S4) |
| `PROXY_HTTP_PORT` | `80` | Puerto del equipo para el 80 del proxy, si está ocupado |
| `CORS_ORIGINS` | `https://localhost` | Orígenes permitidos en el gateway |
| `NEXT_PUBLIC_GATEWAY_URL` | `/api` | URL de la API para el navegador (mismo origen) |
| `GATEWAY_INTERNAL_URL` | `http://agr-gateway-or:8080` | URL del gateway para el servidor de la web |
| `*_DB_USER`, `*_DB_PASSWORD`, `BROKER_*`, `RESULTS_DB_PASSWORD` | ver `.env.example` | Credenciales propias de cada base y del broker |
| `SEED_DATA` | `true` | Carga las elecciones de ejemplo |

En producción, las llaves y claves irían en un gestor de secretos y el certificado lo firmaría una autoridad
reconocida.

## 10. Pruebas e integración continua

El flujo [`.github/workflows/ci.yml`](.github/workflows/ci.yml) corre en cada push:

- Pruebas unitarias de cada componente: vote (Go), CLI (Go), election (Python), voter (Java), audit (.NET),
  gateway (Node), web (Next.js) y results (Rust).
- Prueba del sistema completo: genera el certificado, levanta todo con `docker compose up --wait` y comprueba por
  `https://localhost` la web, la salud del gateway, las elecciones y los resultados; que 3000 y 8080 no estén
  publicados; que `/api/admin` responda 404; la redirección 301, HSTS y TLS 1.2/1.3; la consola de RabbitMQ; la CLI y
  `agora audit verify`.

Prueba de punta a punta de `agr-vote-lo` con sus dependencias reales:

```bash
./agr-vote-lo/e2e/run.sh      # usa docker-compose.yml + docker-compose.test.yml
```

## 11. Problemas frecuentes

| Síntoma | Solución |
|---|---|
| `agr-proxy-rp` no arranca: "Faltan los certificados de S1" | Generar el certificado (paso 1 de la sección 5.2) |
| El puerto 80 está ocupado | Crear `.env` con `PROXY_HTTP_PORT=8088` y entrar igual por `https://localhost` |
| Al levantar: "network ... has incorrect label" o redes viejas del Prototipo 2 | `docker compose down --remove-orphans` y `docker network rm arquisoft_agr-front arquisoft_agr-back arquisoft_agr-data` |
| "Documento o clave incorrectos" con 1001 / 123456 en una base antigua | `docker compose exec -T agr-auth-db psql -U postgres -d agr_auth_db -c "ALTER TABLE voters ADD COLUMN IF NOT EXISTS failed_attempts integer NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS locked_until timestamp(6);"` |
| La cuenta quedó bloqueada (423) | Esperar 15 minutos o `docker compose exec -T agr-auth-db psql -U postgres -d agr_auth_db -c "UPDATE voters SET failed_attempts=0, locked_until=NULL WHERE document='1001';"` |
| "Demasiados intentos de ingreso" (429) | Esperar un minuto (límite por IP del proxy) |
| "Ya votaste en esta elección" | Es correcto: cada votante vota una vez. Usar otro documento o `docker compose down -v` para empezar de cero |
| El navegador muestra "No seguro" | Es el certificado autofirmado de desarrollo; el tráfico va cifrado |
| Desarrollar la web con `npm run dev` | El gateway ya no publica el 8080: publicarlo solo en el equipo con un `docker-compose.override.yml` local (ver `agr-web-fr/.env.development`) |

## 12. Estructura del repositorio

```text
.
├── agr-proxy-rp/          Nginx: reverse proxy, TLS y límites (S1, S2, S4)
├── agr-web-fr/            Web del votante (Next.js)
├── agr-admin-fr/          CLI de administración (Go)
├── agr-gateway-or/        API Gateway (Node.js)
├── agr-auth-lo/           Autenticación y bloqueo de cuentas (Spring Boot)
├── agr-voter-lo/          Participación (Spring Boot)
├── agr-election-lo/       Elecciones y candidatos (FastAPI)
├── agr-vote-lo/           Votos anónimos (Go)
├── agr-results-lo/        Resultados en vivo (Rust)
├── agr-audit-lo/          Auditoría encadenada (.NET)
├── agr-broker-as/         RabbitMQ con su topología
├── docker-compose.yml     Despliegue completo: 17 servicios, 11 redes, 7 volúmenes
├── docker-compose.test.yml  Prueba de punta a punta de agr-vote-lo
├── .env.example           Variables de entorno
├── artifacts/             Artefactos de las entregas (p3_1b.md, p2_1b.md y sus vistas)
└── p3/                    Evidencias del Prototipo 3
    ├── s1/  … s5/         Un escenario de seguridad por carpeta: scripts, antes y después
    └── perf/              Pruebas de rendimiento R1 a R4, resultados y gráficas
```

## 13. Equipo

| Integrante | Componentes | Prototipo 3 |
|---|---|---|
| David Fernando Benjumea Mora | `agr-gateway-or`, `agr-election-lo`, `agr-elections-db` | S3 · Network Segmentation |
| Esteban Prieto Lugo | `agr-auth-lo`, `agr-auth-db` | S4 · Bloqueo de cuentas y límite de inicio de sesión |
| Maicol Sebastián Olarte Ramírez | `agr-vote-lo`, `agr-votes-db`, `agr-broker-as`, `agr-admin-fr` | Rendimiento y escalabilidad (R1–R4) |
| Camilo Borja Rojas | `agr-voter-lo`, `agr-voters-db`, `agr-audit-lo`, `agr-audit-db` | S5 · Auditoría a prueba de alteraciones |
| Cristian Felipe Moreno Gomez | `agr-web-fr`, `agr-proxy-rp` | S2 · Reverse Proxy |
| Miguel Angel Zamudio Villa | `agr-results-lo`, `agr-results-db` | S1 · Secure Channel |
