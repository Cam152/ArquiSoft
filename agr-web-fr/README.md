# agr-web-fr: interfaz web del votante (Next.js con SSR)

Componente de presentación de AGORA. Muestra las elecciones, permite iniciar sesión, votar y seguir los resultados
en vivo. Las páginas se generan en el servidor de Next.js, que consulta a `agr-gateway-or` por la red interna. Desde
el Prototipo 3 todo entra por `agr-proxy-rp` (S2): el navegador pide las páginas en `/` y llama al gateway en `/api/...`
(mismo origen) solo para tres cosas: iniciar sesión, enviar el voto y recibir los resultados en vivo (SSE).

## 1. Cómo funciona

```mermaid
sequenceDiagram
  actor V as Navegador
  participant P as agr-proxy-rp (Nginx)
  participant W as agr-web-fr (servidor Next.js)
  participant G as agr-gateway-or
  V->>P: GET /elecciones/1/resultados
  P->>W: GET /elecciones/1/resultados
  W->>G: GET /results/1 y POST /graphql (red interna)
  G-->>W: conteos con nombres y la elección
  W-->>P: HTML completo con las barras
  P-->>V: HTML + encabezados de seguridad
  V->>P: GET /api/results/1/stream (SSE, mismo origen)
  P->>G: GET /results/1/stream (sin el prefijo, sin buffer)
  G-->>V: event: counts (cada voto contado)
```

| Ruta | Se genera en | Llamadas al gateway |
|---|---|---|
| `/` | Servidor | `POST /graphql` (elecciones con candidatos), desde el servidor |
| `/ingresar` | Servidor + cliente | `POST /api/auth/login` desde el navegador, a través del proxy (límite de intentos de S4) |
| `/elecciones/[id]/votar` | Servidor + cliente | `POST /graphql` desde el servidor; `POST /api/votes` con el JWT desde el navegador |
| `/elecciones/[id]/resultados` | Servidor + cliente | `GET /results/{id}` y `POST /graphql` desde el servidor; `GET /api/results/{id}/stream` (SSE) desde el navegador |
| `/api/health` | Servidor | ninguna (*healthcheck* del contenedor; desde fuera, `/api/` es del gateway) |

Para comprobar el SSR basta con abrir "Ver código fuente" en `/`: las elecciones ya vienen en el HTML. En
`docker compose logs agr-web-fr` aparece cada llamada del servidor al gateway (método, ruta, código y duración).

## 2. Estructura

```
agr-web-fr/
├── app/                              Páginas (App Router)
│   ├── layout.tsx                    Estructura común: encabezado, contenido y pie
│   ├── page.tsx                      /  lista de elecciones (SSR)
│   ├── ingresar/
│   │   ├── page.tsx                  /ingresar
│   │   └── actions.ts                Server Action: login contra el gateway desde el servidor
│   ├── elecciones/[id]/
│   │   ├── votar/page.tsx            SSR de la elección + formulario de voto
│   │   └── resultados/page.tsx       SSR de los resultados + SSE
│   ├── api/health/route.ts           Healthcheck
│   ├── not-found.tsx, error.tsx
│   └── globals.css
├── components/                       Componentes de interfaz
│   ├── SiteHeader.tsx, SessionMenu.tsx
│   ├── ElectionCard.tsx, StatusBadge.tsx, Notice.tsx, PageError.tsx, RetryButton.tsx
│   ├── LoginForm.tsx                 (cliente) POST /api/auth/login + guarda el JWT
│   ├── VoteForm.tsx                  (cliente) elegir, confirmar y POST /votes
│   ├── LiveResults.tsx               (cliente) SSE + combinación con los candidatos
│   └── ResultBars.tsx
├── lib/
│   ├── api/
│   │   ├── server.ts                 Cliente del gateway para el servidor (server-only, red interna)
│   │   ├── browser.ts                Las tres llamadas del navegador: login, POST /votes y la URL del SSE
│   │   ├── errors.ts                 ApiError y traducción de códigos a mensajes (único lugar)
│   │   └── types.ts                  Tipos de los contratos
│   ├── config.ts                     Variables de entorno, leídas al ejecutar
│   ├── session.ts                    Sesión en sessionStorage + hook useSession
│   ├── useResultsStream.ts           EventSource con reconexión
│   ├── results.ts                    Conteos + candidatos → filas ordenadas con porcentajes
│   ├── navigation.ts                 Ruta de vuelta segura y validación del id
│   └── results.test.ts               Pruebas (npm test)
├── Dockerfile, next.config.ts, package.json, tsconfig.json
└── .env.development                  URLs para `npm run dev` fuera de Docker
```

Regla de dependencias: `app/` usa `components/` y `lib/`; `components/` usa `lib/`. `lib/api/server.ts` importa
`server-only`, así que la compilación falla si un componente de cliente lo importa por error.

## 3. Contratos que consume

Todos pasan por el gateway (README principal, sección 5.1).

| Llamada | Desde | Respuesta que se usa |
|---|---|---|
| `POST /graphql` `elections` y `election(id)` | Servidor | `id, name, description, status, startsAt, endsAt, isOpen, candidateCount, candidates { id number name description }` |
| `POST /auth/login` `{document, password}` | Navegador (`/api/auth/login`) | `{access_token, token_type, expires_in}` |
| `POST /votes` `{election_id, candidate_id}` + `Authorization: Bearer` | Navegador | `201 {"status": "recorded"}` |
| `GET /results/{id}` | Servidor | `{election_id, election_name, total_votes, results[]}` |
| `GET /results/{id}/stream` | Navegador | eventos `counts` con `{election_id, total_votes, counts}` |

**Errores.** Cada respuesta fallida se convierte en un `ApiError` con el código y un mensaje para el votante, todo
en `lib/api/errors.ts`:

| Código | Dónde | Mensaje | Qué hace la interfaz |
|---|---|---|---|
| 401 | Login | "Documento o clave incorrectos." | Conserva el documento y borra la clave |
| 423 | Login | "Tu cuenta está bloqueada temporalmente…" (Auth Service, S4) | Conserva el documento y borra la clave |
| 429 | Login | "Hiciste demasiados intentos seguidos…" (proxy, S4) | Conserva el documento y borra la clave |
| 413 | Cualquiera | "La petición es demasiado grande…" (proxy, S2) | Aviso de error |
| 401 | Voto | "Tu sesión expiró…" | Cierra la sesión y lleva a `/ingresar?motivo=expirada`; después vuelve a votar |
| 404 | Páginas | | Página "No encontramos lo que buscas" |
| 409 | Voto | "Ya votaste en esta elección…" o "La elección no está abierta." (según `detail`) | Rechazo definitivo: ofrece ver resultados |
| 422 | Voto | "El candidato no pertenece a esta elección…" | Rechazo definitivo |
| 500 | Voto | "No se pudo guardar tu voto y no quedó registrado…" | Conserva la elección y permite reintentar |
| 502 | Voto | El `detail` del gateway (dice si el voto quedó o no registrado) | Permite reintentar |
| 503, 504 | Páginas | "AGORA no está disponible…" / "tardó demasiado…" | Aviso con botón "Reintentar" (vuelve a generar la página) |

## 4. Variables de entorno

| Variable | Por defecto | Uso |
|---|---|---|
| `GATEWAY_INTERNAL_URL` | `http://agr-gateway-or:8080` | Gateway visto desde el servidor de Next.js (red `agr-edge-net`) |
| `NEXT_PUBLIC_GATEWAY_URL` | `/api` | Gateway visto desde el navegador (login, voto y SSE). Relativa: sirve con HTTP y con HTTPS |
| `GATEWAY_TIMEOUT_MS` | `8000` | Tiempo máximo de cada llamada del servidor al gateway |

Las tres se leen **al ejecutar**, no al compilar. `NEXT_PUBLIC_GATEWAY_URL` la lee el servidor y la pasa como prop a
los tres componentes que la necesitan, así que cambiarla no exige reconstruir la imagen.

## 5. Cómo ejecutarlo

**Con Docker Compose** (desde la raíz del repositorio):

```bash
docker compose up --build -d --wait agr-proxy-rp   # levanta también la web y el gateway
```

La web queda en `http://localhost` (con S1, `https://localhost`); el puerto 3000 ya no se publica.

Bloque del servicio en `docker-compose.yml`:

```yaml
  agr-web-fr:
    container_name: agr-web-fr
    build: ./agr-web-fr
    environment:
      GATEWAY_INTERNAL_URL: ${GATEWAY_INTERNAL_URL:-http://agr-gateway-or:8080}
      NEXT_PUBLIC_GATEWAY_URL: ${NEXT_PUBLIC_GATEWAY_URL:-/api}
    depends_on:
      agr-gateway-or:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://127.0.0.1:3000/api/health"]
      interval: 10s
      timeout: 5s
      retries: 6
      start_period: 10s
    networks: [agr-edge-net]
    restart: unless-stopped
```

**Sin Docker** (Node 22 o superior). Desde S2 el gateway no publica `8080`; para desarrollar la web fuera de Docker
hay que publicarlo solo en el equipo con un `docker-compose.override.yml` local (ver `.env.development`):

```bash
cd agr-web-fr
npm install
npm run dev          # http://localhost:3000, usa .env.development
npm test             # pruebas de lib/ (no necesitan el gateway)
npm run build        # compila y revisa los tipos
```

La imagen se construye en tres etapas sobre `node:22-alpine`: dependencias (`npm ci`), compilación (`next build`
con `output: "standalone"`) y ejecución (`node server.js` como usuario `node`, puerto 3000). La compilación no
necesita al gateway, porque las páginas se generan en cada petición.

## 6. Decisiones de diseño

- **SSR en cada petición** (`export const dynamic = "force-dynamic"`). El HTML sale con los datos del momento y
  `next build` no depende de que el gateway esté arriba.
- **Login desde el navegador, por el proxy.** En el Prototipo 2 el login era una Server Action: el servidor de
  Next.js llamaba a `POST /auth/login` por la red interna. Con el límite de intentos de S4 en el proxy eso ya no
  sirve, porque todos los intentos saldrían de la IP del contenedor de la web y un solo atacante bloquearía el login
  de todos. Ahora el formulario llama a `POST /api/auth/login` (mismo origen), así el proxy limita por la IP real de
  cada votante. El botón queda deshabilitado hasta que React hidrata la página y el formulario es `method="post"`,
  de modo que la clave nunca termina en la URL.
- **JWT en `sessionStorage`.** El navegador lo necesita para enviar `POST /api/votes` al gateway. La sesión se
  borra al cerrar la pestaña y se cierra sola cuando vence el token (una hora). Las páginas que se generan en el
  servidor no usan el token, porque elecciones y resultados son públicos.
- **Resultados en vivo.** El SSE solo trae conteos por candidato; `lib/results.ts` los combina con los candidatos
  que llegaron en el HTML y calcula total, porcentajes y orden igual que el gateway. Si el gateway responde algo
  que no es un stream (por ejemplo, 502 con `agr-results-lo` caído), `useResultsStream` se reconecta con espera
  creciente hasta 30 segundos. El stream solo se abre si la elección está activa.
- **Logs sin datos sensibles.** El servidor registra método, ruta, código y duración de cada llamada al gateway;
  nunca cuerpos, claves ni tokens.
- **Encabezados de seguridad.** `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`,
  `Referrer-Policy: same-origin` y sin `X-Powered-By`. Desde S2 los pone `agr-proxy-rp` en todas las respuestas
  (también `Content-Security-Policy` y `Permissions-Policy`); los de `next.config.ts` se dejan como respaldo para
  cuando la web corre sin el proxy.

## 7. Límites conocidos

- El JWT queda al alcance de JavaScript (`sessionStorage`). Una cookie `HttpOnly` sería más segura, pero obligaría a
  que `POST /votes` también pasara por el servidor de Next.js.
- Cada pestaña tiene su propia sesión: abrir una pestaña nueva pide ingresar otra vez.
- Después de votar, los resultados pueden tardar un instante en reflejar el voto (consistencia eventual del broker).
