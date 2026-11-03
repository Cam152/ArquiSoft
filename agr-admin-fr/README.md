# agr-admin-fr: CLI de administración `agora` (Go, Cobra)

Segundo componente de presentación de AGORA. Reemplaza el uso de Swagger del Prototipo 1: con él quien administra
crea, edita, abre y cierra elecciones, gestiona candidatos y consulta resultados y auditoría. Responsable: Maicol.

Solo habla con `agr-gateway-or` (conector c2). Las rutas `/admin/*` llevan el header `X-API-Key`; las de lectura
(`/elections`, `/results`) son públicas y **no** llevan la llave.

## Uso con Docker Compose

La CLI está en el perfil `tools`, así que `docker compose up` no la arranca. Se ejecuta bajo demanda con el
sistema ya levantado:

```bash
docker compose run --rm agr-admin-fr elections list
docker compose run --rm agr-admin-fr elections create --name "Elección 2026" --description "Prueba"
docker compose run --rm agr-admin-fr candidates add --election 4 --name "Ana Torres"
docker compose run --rm agr-admin-fr candidates add --election 4 --name "Luis Herrera"
docker compose run --rm agr-admin-fr elections open 4
docker compose run --rm agr-admin-fr results show 4
docker compose run --rm agr-admin-fr audit list --election 4
```

Tip: un alias evita repetir el prefijo.

```bash
alias agora='docker compose run --rm agr-admin-fr'
agora elections show 1
```

## Comandos

| Comando | Petición al gateway |
|---|---|
| `elections list [--status draft\|active\|closed]` | `GET /elections` |
| `elections show <id>` | `GET /elections/{id}` |
| `elections create --name ... [--description ...] [--starts ... --ends ...]` | `POST /admin/elections` |
| `elections update <id> [--name ...] [--description ...] [--starts ...] [--ends ...]` | `PATCH /admin/elections/{id}` |
| `elections delete <id> [--yes]` | `DELETE /admin/elections/{id}` |
| `elections open <id>` | `POST /admin/elections/{id}/activate` |
| `elections close <id>` | `POST /admin/elections/{id}/close` |
| `candidates add --election <id> --name ... [--description ...]` | `POST /admin/elections/{id}/candidates` |
| `candidates remove --election <id> --candidate <cid>` | `DELETE /admin/elections/{id}/candidates/{cid}` |
| `results show <id>` | `GET /results/{id}` |
| `audit list [--election <id>] [--limit 50]` | `GET /admin/audit` |

Detalles:

- `update` solo envía los campos que se escriben; `--description ""` borra la descripción.
- `delete` pide confirmación (`s/N`); `--yes` la omite, útil en scripts.
- Fechas en RFC 3339 con zona horaria: `--starts 2026-11-03T08:00:00-05:00`.
- `open` exige al menos 2 candidatos (regla de `agr-election-lo`); el error se muestra tal cual.
- Ayuda de cualquier comando: `agora <comando> --help`.

**Opciones globales**

| Opción | Variable de entorno | Por defecto |
|---|---|---|
| `--gateway` | `AGORA_GATEWAY_URL` | `http://localhost:8080` (en el compose, `http://agr-gateway-or:8080`) |
| `--api-key` | `AGORA_ADMIN_KEY` | vacía (en el compose toma `ADMIN_API_KEY`) |
| `--json` | | salida en JSON en vez de tablas, para scripts |
| `--timeout` | | `10s` por petición |

Prefieran la variable de entorno a `--api-key`: lo que se escribe en la línea de comandos queda en el historial
de la terminal.

**Salida y errores.** Las respuestas se muestran como tablas (`text/tabwriter`). Si el gateway responde un error,
se imprime su `detail` con el código HTTP en la salida de error y el programa termina con código 1. Los errores
de validación de FastAPI (422) se resumen como `campo: mensaje`.

## Sin Docker

Con Go 1.23 o superior y el gateway publicado en `localhost:8080`:

```bash
cd agr-admin-fr
go build -o agora ./cmd/agora
AGORA_ADMIN_KEY=dev-admin-key ./agora elections list
```

## Estructura

| Carpeta | Responsabilidad |
|---|---|
| `cmd/agora` | Punto de entrada: señales y código de salida |
| `internal/commands` | Un comando Cobra por operación: lee argumentos, valida y muestra el resultado |
| `internal/client` | Único lugar que habla con el gateway: rutas, `X-API-Key`, errores `detail` |

## Pruebas

```bash
cd agr-admin-fr
go test -race -cover -coverpkg=./internal/... ./...
```

Corren contra un gateway simulado (`httptest`) y comprueban para cada comando el método, la ruta, la *query*, el
cuerpo JSON y que la llave solo viaje en las rutas `/admin`, además de la salida, la confirmación de `delete`, los
mensajes de error y el código de salida.
