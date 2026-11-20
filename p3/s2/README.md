# S2 · Reverse Proxy

Responsable: Cristian. Componente nuevo: [`agr-proxy-rp`](../../agr-proxy-rp/) (Nginx).

## Qué se hizo

Se agregó `agr-proxy-rp`, un Nginx que pasa a ser el único punto de entrada a AGORA desde fuera de Docker. La web y
el gateway dejaron de publicar sus puertos y ahora solo se alcanzan a través del proxy.

- **Rutas.** `/` va a `agr-web-fr:3000` y `/api/...` a `agr-gateway-or:8080` sin el prefijo (`/api/votes` llega
  al gateway como `/votes`). Los resultados en vivo (`/api/results/{id}/stream`) pasan sin buffer y con conexión
  larga. `/api/admin/...` no se expone: la administración se hace con la CLI por la red interna.
- **Seguridad en el borde.** Se ocultan `Server`, `X-Powered-By` y `X-Nextjs-*`; todas las respuestas, incluidos
  los errores, llevan `Content-Security-Policy`, `X-Frame-Options`, `X-Content-Type-Options`, `Referrer-Policy`,
  `Permissions-Policy` y `Cross-Origin-Opener-Policy`. Las peticiones de más de 1 MB se rechazan con 413 antes de
  llegar a los servicios, y los errores del proxy (413, 429, 502) responden con el formato `{"detail": ...}` de AGORA.
- **Límite de login (sección de S4).** `/api/auth/login` acepta 5 intentos por minuto por IP, con ráfaga de 5; al
  superarlo el proxy responde 429.
- **Redes.** Se agregó `agr-public-net`, la única red desde la que se publican puertos, y `agr-edge-net` pasó a
  `internal: true`.
- **Web.** `NEXT_PUBLIC_GATEWAY_URL` ahora es `/api`, una ruta relativa al mismo origen. El login dejó de hacerse
  desde el servidor de Next.js y ahora sale del navegador por `/api/auth/login`, para que el límite vea la IP real
  de cada votante. Se agregaron los mensajes para 413, 423 y 429.
- **Réplicas.** El proxy resuelve los nombres con el DNS de Docker en cada petición, así que encuentra al gateway
  si se reinicia con otra IP y reparte la carga cuando hay varias réplicas (R4).
- **Preparado para S1.** `nginx.conf` tiene marcada la sección de TLS, la redirección de 80 a 443 y HSTS.

## Antes y ahora

| Aspecto | Antes | Ahora |
|---|---|---|
| Entrada al sistema | `localhost:3000` (web) y `localhost:8080` (gateway) | `http://localhost` (con S1, `https://localhost`) |
| Puertos publicados | 3000, 8080 y `127.0.0.1:15672` | 80 y `127.0.0.1:15672` |
| Llamadas del navegador al gateway | `http://localhost:8080/...`, otro origen, con CORS | `/api/...`, mismo origen, sin CORS |
| Login | Server Action: el servidor de Next.js llamaba al gateway | El navegador llama a `/api/auth/login` a través del proxy |
| Tecnología visible en las respuestas | `x-nextjs-cache`, `x-nextjs-prerender`, `Keep-Alive` de Node.js | Ninguna |
| Encabezados de seguridad | Algunos en la web; ninguno en el gateway | Seis en todas las respuestas |
| Tamaño de las peticiones | La web aceptaba cualquier tamaño; el gateway leía hasta 1 MB | El proxy rechaza más de 1 MB con 413 |
| Intentos de login | Sin límite en el borde | 429 al pasar el límite por IP |
| Rutas `/admin/...` | Alcanzables desde fuera en `:8080` (con la llave) | Solo por la red interna (CLI) |
| Redes | `agr-edge-net` sin `internal` | `agr-public-net` (pública) y `agr-edge-net` `internal` |

## Archivos que cambiaron

| Archivo | Cambio |
|---|---|
| `agr-proxy-rp/Dockerfile` | Nuevo: Alpine 3.22 con `nginx` y `nginx-mod-http-headers-more` |
| `agr-proxy-rp/nginx.conf` | Configuración completa del proxy, con secciones marcadas para S1 y S4 |
| `docker-compose.yml` | Servicio `agr-proxy-rp`, red `agr-public-net`, `agr-edge-net` interna, sin puertos 3000 y 8080 |
| `.env.example` | `NEXT_PUBLIC_GATEWAY_URL=/api`, `CORS_ORIGINS=https://localhost` y las variables del proxy |
| `agr-web-fr/components/LoginForm.tsx`, `lib/api/browser.ts` | Login desde el navegador por `/api/auth/login` |
| `agr-web-fr/lib/api/errors.ts` | Mensajes para 413, 423 y 429 |
| `agr-web-fr/app/ingresar/actions.ts` | Eliminado (era la Server Action del login) |

## Cómo ejecutar

Desde la raíz del repositorio:

```powershell
docker compose up -d --build --wait
docker compose ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}"
```

En la tabla, `agr-proxy-rp` debe aparecer `healthy` con `0.0.0.0:80->80/tcp`, y ni `agr-web-fr` ni
`agr-gateway-or` deben tener puertos publicados. Luego se abre **http://localhost**.

Votantes de prueba: documentos **1001** a **1005**, todos con la clave **123456**. Cada uno vota una vez por
elección, y tras cinco claves erradas la cuenta se bloquea 15 minutos.

Variables opcionales en `.env`:

| Variable | Por defecto | Uso |
|---|---|---|
| `PROXY_HTTP_PORT` | `80` | Puerto del equipo para el proxy, si el 80 ya está ocupado (se abre `http://localhost:<puerto>`) |
| `LOGIN_RATE` | `5r/m` | Intentos de login por IP |
| `LOGIN_BURST` | `5` | Intentos extra permitidos en ráfaga |

### Si el login responde "Documento o clave incorrectos" con 1001 / 123456

La base de `agr-auth-lo` viene de una versión anterior al bloqueo de cuenta de S4 y le falta una columna. Se arregla
sin borrar datos (si la columna ya existe, no hace nada):

```powershell
docker compose exec -T agr-auth-db psql -U postgres -d agr_auth_db -c "ALTER TABLE voters ADD COLUMN IF NOT EXISTS failed_attempts integer NOT NULL DEFAULT 0, ADD COLUMN IF NOT EXISTS locked_until timestamp(6);"
```

### Desarrollo de la web fuera de Docker

Como el gateway ya no publica el 8080, quien use `npm run dev` debe publicarlo solo en su equipo con un
`docker-compose.override.yml` local (no se sube). Los detalles están en
[`agr-web-fr/.env.development`](../../agr-web-fr/.env.development).
