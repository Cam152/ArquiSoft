# Borrador · 3.3 Estructura por capas (Prototipo 3)

Responsable: Maicol. Texto listo para pasar a `artifacts/p3_1b.md`. Parte de la sección 3.3 del P2 y agrega lo que
cambió en P3: el nivel de borde (`agr-proxy-rp`, S2), la segmentación por niveles (S3), el bloqueo de cuenta (S4), la
auditoría encadenada (S5) y los ajustes de rendimiento. **Pendiente:** actualizar la imagen `p2_capas.png` → `p3_capas.png`
con el nivel T0 y las réplicas del gateway; y confirmar los nombres de archivo de S1 y S5 cuando entren a `p3`.

## 3.3.1 Vista por capas

La vista organiza AGORA en **niveles (tiers)** con la relación *allowed to use*. En P3 se agrega un nivel **T0 · Borde**
por encima de la presentación: `agr-proxy-rp` es lo único alcanzable desde fuera de Docker. El nivel inferior T4 sigue
con dos partes al mismo nivel: comunicación asíncrona (T4a) y datos (T4b). Cada nivel corresponde además a una zona de
red de S3, así que la regla *allowed to use* ya no es solo de diseño: la red impide los usos no permitidos.

![Vista por capas](p3_capas.png)

## 3.3.2 Descripción de los elementos arquitectónicos y sus relaciones

**Niveles**

| Nivel | Componentes | Responsabilidad | Puede usar a | Red (S3) |
|---|---|---|---|---|
| T0 · Borde | `agr-proxy-rp` | Único punto de entrada: TLS (S1), encabezados de seguridad, límite de 1 MB, límite de login por IP (S4), reparto entre réplicas del gateway (R4) | T1 (`/`) y T2 (`/api/`) | `agr-public-net`, `agr-edge-net` |
| T1 · Presentación | `agr-web-fr`, `agr-admin-fr` | Interacción con el votante y con el administrador | T2 | `agr-edge-net` |
| T2 · Orquestación | `agr-gateway-or` (1 a N réplicas) | Punto único de la API, composición, saga del voto | T3 | `agr-edge-net`, `agr-back-net` |
| T3 · Lógica | Los seis servicios `-lo` | Reglas del negocio, cada servicio sobre sus propios datos | T4a y T4b | `agr-back-net` + su red de datos (+ `agr-bus-net`) |
| T4a · Comunicación asíncrona | `agr-broker-as` | Recibe y reparte los eventos de voto | — | `agr-bus-net` |
| T4b · Datos | Las seis bases `-db` | Persistencia de cada servicio | — | `agr-<x>-data-net` (una por servicio) |

**Capas internas de cada componente** (en negrita lo que cambió en P3)

| Componente(s) | Estilo interno | Capas (de arriba hacia abajo) |
|---|---|---|
| `agr-proxy-rp` | Pipeline de Nginx por `location` | **Entrada** (`server` 80/443, TLS) → **Filtros** (encabezados, `client_max_body_size`, `limit_req` en `/api/auth/login`) → **Enrutamiento** (`location /`, `/api/`, SSE sin buffer, `/api/admin` bloqueado) → **Resolución de destinos** (`resolver` del DNS de Docker, `$gateway_upstream`) |
| `agr-web-fr` | Next.js App Router con SSR | **Interfaz** (`app/` páginas, `components/`, **`LoginForm.tsx`**) → **Lógica de cliente y acceso a datos** (`lib/api/server.ts`, **`lib/api/browser.ts`** con el login desde el navegador, **`lib/api/errors.ts`** con 413/423/429, `lib/session.ts`, `lib/useResultsStream.ts`, `lib/config.ts`). Se eliminó `ingresar/actions.ts` |
| `agr-admin-fr` | CLI por comandos | **Comandos** (`internal/commands/`) → **Cliente HTTP** (`internal/client/client.go`), por la red interna |
| `agr-gateway-or` | Layered | **Entrada** (`server.ts`, `app.ts`, `routes/`, `middleware/`) → **Aplicación** (`saga/vote.ts`, `compose/results.ts`, `sse/proxy.ts`) → **Integración** (`clients/`, **`clients/auth.ts`** propaga 423 y 429, `http.ts`, `config.ts`). Sin estado: por eso escala con réplicas |
| `agr-auth-lo` | Controller → Service → Repository | **Controlador** (`AuthController`) → **Servicio** (`AuthService` **con el conteo de intentos y el bloqueo de 15 min**, `JwtService`) → **Repositorio y modelo** (`VoterAuthRepository`, **`VoterAuth` con `failedAttempts` y `lockedUntil`**) |
| `agr-voter-lo` | Controller → Service → Repository | `VoterController` → `VoterService` → `BallotRepository` |
| `agr-election-lo` | Controller → Service → Repository | `routers/`, `graphql_api.py` → `services.py` → `models.py`, **`database.py` con pool configurable**. **El router arma la respuesta y libera la conexión antes de serializar** (corrección del Lab 6) |
| `agr-vote-lo` | Layered con publicación de eventos | `internal/handler/` → `internal/service/` → `repository/` (MongoDB), `publisher/` (AMQP), `outbox/` |
| `agr-results-lo` | Layered event-driven | Entrada HTTP y AMQP (`handler.rs`, `sse.rs`, `consumer.rs`) → `service.rs`, `hub.rs` → `repository.rs` (Redis) |
| `agr-audit-lo` | Layered event-driven | Entrada (`Controllers/`, `Consumers/`) → Servicio (`Services/`, **cadena de hashes y verificación de S5 (pendiente de entrar a `p3`)**) → Repositorio (`Repositories/`, MongoDB) |

**Relaciones**

- *Allowed to use* entre niveles: T0 → T1 → T2 → T3 → T4. La excepción es T0, que usa a T1 **y** a T2 (*layering*
  relajado): el navegador pide las páginas a la web (`/`) y llama a la API (`/api/...`) por el mismo origen. El proxy
  no aplica lógica de negocio, solo controles de borde, así que no reemplaza a la presentación.
- La web (T1) usa al gateway (T2) por `agr-edge-net` para el SSR; la CLI hace lo mismo. Ninguna de las dos alcanza
  T3 ni T4: no comparten red con ellos (S3).
- T3 usa T4a para publicar y consumir eventos y T4b para persistir; cada servicio solo ve su base.
- Dentro de cada componente, cada capa solo usa a la siguiente.

## 3.3.3 Descripción de los patrones arquitectónicos utilizados

| Patrón | Cómo se aplica |
|---|---|
| **Arquitectura por niveles (5 tiers)** | Borde, presentación, orquestación, lógica y un nivel inferior con comunicación asíncrona y datos, con *allowed to use* hacia abajo |
| **Layering relajado en el borde** | T0 puede usar T1 y T2 directamente; es la única excepción y está acotada por rutas (`/` y `/api/`) |
| **Niveles alineados con zonas de red** | Cada nivel es una zona de S3 (`public`, `edge`, `back`, `data`, `bus`), así que un nivel no puede saltarse otro aunque lo intente |
| **Layered dentro de cada componente** | Entrada, lógica y acceso a datos separados con dependencia en un solo sentido |
| **Layered event-driven** | Results y audit son también consumidores de eventos AMQP |
| **Nivel de orquestación sin estado (escalable)** | `agr-gateway-or` no guarda sesión ni datos; por eso se replica (`--scale`) y el proxy reparte (R4) |
| **Base de datos por servicio** | En T4b cada base pertenece a un único servicio de T3 y vive en su propia red |
