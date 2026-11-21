<p align="center">
  <img src="artifacts/AGORA_logo.png" alt="AGORA, sistema de votación electrónica" width="320">
</p>

# AGORA: sistema de votación electrónica (Prototipo 3)

**AGORA** es un sistema de votación electrónica para elecciones estudiantiles. Un votante inicia sesión con su
documento y su clave, ve las elecciones abiertas, vota **una vez en cada elección** y sigue los resultados, que se
actualizan solos mientras la votación está abierta. Quien administra crea, abre y cierra elecciones desde una consola
de comandos. El voto es **anónimo**: el sistema sabe quién votó y qué se votó, pero ningún dato permite unir las dos
cosas.

Proyecto del equipo 1b para el curso Arquitectura de Software (UNAL, 2026-II, prof. Jeisson Andrés Vergara Vargas).
Rama de trabajo: `p3`.

**Objetivo.** Rediseñar la arquitectura de AGORA con foco en dos atributos de calidad, **seguridad** y
**rendimiento**, sin perder ninguna de sus funciones.

**Entregas.** Artefacto `p3_1b.pdf` (exportado de [`artifacts/p3_1b.md`](artifacts/p3_1b.md)) en MiCampus el **lunes
23 de noviembre de 2026**. Presentación en clase el **martes 24 de noviembre**.

## Qué pide el enunciado

| Requisito | Cómo lo cubre AGORA |
|---|---|
| Completitud funcional (la define el equipo) | Todas las funciones siguen funcionando de punta a punta, ahora a través del proxy y con HTTPS: iniciar sesión, listar elecciones, votar una vez por elección, ver resultados en vivo, administrar elecciones y candidatos desde la CLI y consultar la auditoría |
| Escenarios de seguridad S1, S2 y S3 (obligatorios) | Secure Channel, Reverse Proxy y Network Segmentation |
| Escenarios de seguridad S4 y S5 (los elige el equipo) | Protección del login contra fuerza bruta y auditoría a prueba de alteraciones |
| Análisis de rendimiento y escalabilidad | Pruebas de carga con k6, costo de la seguridad y escalado horizontal del gateway |
| Artefacto | Equipo, sistema, cuatro estructuras (C&C, despliegue, capas y descomposición) y atributos de calidad (seguridad: escenarios, tácticas y patrones; rendimiento y escalabilidad: análisis y resultados) |

## Escenarios de seguridad

| Escenario | Problema en el Prototipo 2 | Qué se construye | Patrón |
|---|---|---|---|
| S1 · Secure Channel | Todo viaja en HTTP plano: el documento, la clave y el JWT se pueden capturar en la red | TLS 1.2/1.3 en el proxy con un certificado de `localhost` generado por script (la llave privada no se sube al repositorio), redirección de 80 a 443 y HSTS | Secure Channel |
| S2 · Reverse Proxy | La web (3000) y el gateway (8080) se publican directo y revelan su tecnología | Componente nuevo `agr-proxy-rp` (Nginx) como único punto de entrada: `/` va a la web y `/api/` al gateway (sin el prefijo), SSE sin buffer, límite de 1 MB por petición, encabezados de seguridad y sin `Server` ni `X-Powered-By` | Reverse Proxy |
| S3 · Network Segmentation | La red de datos es plana: un servicio comprometido alcanza todas las bases | Diez redes en `docker-compose.yml`, ocho de ellas `internal`: una red de datos por servicio, el broker solo con sus clientes y la consola de RabbitMQ solo en `127.0.0.1`. Con el proxy se agrega `agr-public-net` y `agr-edge-net` pasa a `internal`. Diseño en [`p3/s3/flujos-permitidos.md`](p3/s3/flujos-permitidos.md) | Network Segmentation |
| S4 · Fuerza bruta contra el login | `/auth/login` acepta intentos sin límite | Límite por IP en el proxy (`limit_req`, respuesta 429) y bloqueo de la cuenta en `agr-auth-lo` tras 5 intentos fallidos (columnas `failed_attempts` y `locked_until`, respuesta 423). El gateway deja pasar 423 y 429 y la web muestra un mensaje claro | Rate Limiting |
| S5 · Alteración de la auditoría | Quien entre a `agr-audit-db` puede borrar o cambiar eventos sin que se note | Cadena de hashes en `agr-audit-lo` (`seq`, `prev_hash` y `hash` SHA-256), endpoint `GET /audit/verify` y ruta `GET /admin/audit/verify` en el gateway | Tamper-evident log |

Cada escenario se documenta en seis partes (fuente, estímulo, artefacto, entorno, respuesta y medida de la respuesta),
con sus tácticas, su patrón y evidencias **antes** (en el Prototipo 2) y **después** (en `p3`).

## Cambios de arquitectura

| Aspecto | Prototipo 2 | Prototipo 3 |
|---|---|---|
| Entrada al sistema | `localhost:3000` (web) y `localhost:8080` (gateway), en HTTP | Solo `https://localhost` (443) a través de `agr-proxy-rp`; el puerto 80 solo redirige |
| Puertos publicados | 3000, 8080 y 15672 | 443, 80 y `127.0.0.1:15672` |
| Llamadas del navegador al gateway | `http://localhost:8080/...` (otro origen, con CORS) | `/api/...` en el mismo origen (login, voto y SSE), con `NEXT_PUBLIC_GATEWAY_URL=/api`: la ruta relativa sirve igual con HTTP y con HTTPS |
| Componentes | 16 | 17: se agrega `agr-proxy-rp` (Nginx) |
| Redes | `agr-front`, `agr-back` y `agr-data` | Diez redes segmentadas más `agr-public-net` |
| Login | Sin límite de intentos | Límite por IP y bloqueo de cuenta (S4) |
| Auditoría | Eventos sueltos | Cadena de hashes verificable (S5) |

El servidor de Next.js y la CLI siguen llamando al gateway por la red interna (`http://agr-gateway-or:8080`), así que
no pasan por el proxy. El login sí sale del navegador (`/api/auth/login`), para que el límite de S4 vea la IP de cada
votante. La ruta `/api/health` de la web solo la usa su propio healthcheck dentro del contenedor.

## Rendimiento y escalabilidad

Pruebas de carga con **k6**. La base está en la rama `feat/agr-performance` (carpeta `lab6-performance/`, prueba de
la lista de elecciones con rampa de usuarios); aquí se pasa a `p3/perf/` y la URL deja de estar fija
(variable `BASE_URL`).

| Prueba | Qué mide |
|---|---|
| R1 · Lectura | Lista de elecciones y resultados, con rampa de usuarios virtuales |
| R2 · Escritura | Login y voto (la saga completa), con rampa hasta el punto de quiebre |
| R3 · Costo de la seguridad | R1 y R2 sin proxy ni TLS (línea base) contra R1 y R2 con proxy, TLS y límites |
| R4 · Escalabilidad | R1 con 1 y con 3 réplicas del gateway detrás del proxy (`--scale agr-gateway-or=3`) |

Métricas: latencia p50, p95 y p99, peticiones por segundo, tasa de error y uso de CPU y memoria por contenedor
(`docker stats`). Condiciones:

- La **línea base** se mide en `p3` con la segmentación pero **antes** de que entre el proxy.
- R2 necesita muchos votantes: un script `p3/perf/seed-voters.sql` inserta N votantes en `agr-auth-db` con el mismo
  hash BCrypt de los votantes de prueba (clave `123456`).
- Durante R2 el límite de S4 se sube o se desactiva con una variable, para medir el sistema y no el límite.
- Para R4 el gateway no puede tener `container_name` ni puertos publicados (el proxy lo resuelve por nombre).

## Reparto del trabajo

Cada integrante sigue siendo responsable de sus componentes y, además, entrega código, evidencias y su parte del
documento:

| Integrante | Tarea | Qué entrega | Sección del documento |
|---|---|---|---|
| David | S3 · Network Segmentation | Crear la rama `p3` con la segmentación aplicada en `docker-compose.yml` y `docker-compose.test.yml`, la matriz de flujos (`p3/s3/flujos-permitidos.md`) y el script `p3/s3/verify-segmentation.sh`. Guardar la matriz de alcanzabilidad antes y después en `p3/s3/`. Cuando entre S2, repetirla con el proxy | Escenario S3 |
| Cristian | S2 · Reverse Proxy | Carpeta `agr-proxy-rp/` (Dockerfile y `nginx.conf`) y su servicio en el compose, en `agr-public-net` y `agr-edge-net`. Rutas `/` y `/api/`, SSE sin buffer, 1 MB por petición, encabezados de seguridad. Quitar los puertos 3000 y 8080, ajustar `NEXT_PUBLIC_GATEWAY_URL` y `CORS_ORIGINS`. En la web, mensajes para 423 y 429. Evidencias en `p3/s2/` | Escenario S2 y estructura de despliegue |
| Miguel | S1 · Secure Channel | Script `p3/s1/gen-certs.sh` (openssl, certificado de `localhost`; la llave va al `.gitignore`), bloque TLS en `nginx.conf` (TLS 1.2/1.3, redirección 80 → 443, HSTS). Evidencias en `p3/s1/`: captura de tráfico antes (clave legible) y después (cifrado) y `curl -v` mostrando el certificado | Escenario S1 y estructura C&C |
| Esteban | S4 · Fuerza bruta contra el login | Bloqueo de cuenta en `agr-auth-lo` (5 intentos, respuesta 423), `limit_req` para `/api/auth/login` en el proxy (respuesta 429) y paso de 423 y 429 en `agr-gateway-or/src/clients/auth.ts`. Evidencias en `p3/s4/`: un script que lanza intentos fallidos y muestra 429 y 423 | Escenario S4, Equipo y Sistema (con la completitud funcional), consolidación y exportación a PDF |
| Camilo | S5 · Auditoría a prueba de alteraciones | Cadena de hashes en `agr-audit-lo`, `GET /audit/verify` y la ruta `GET /admin/audit/verify` en el gateway. Evidencias en `p3/s5/`: cambiar un evento en `agr-audit-db` y ver que la verificación señala el evento alterado | Escenario S5 y estructura de descomposición |
| Maicol | Rendimiento y escalabilidad | R1 a R4 en `p3/perf/` a partir de `lab6-performance/`, `seed-voters.sql`, resultados (resúmenes de k6 y `docker stats`) y gráficas | Rendimiento y escalabilidad, estructura por capas |

Las estructuras se actualizan con lo nuevo: `agr-proxy-rp`, los conectores HTTPS, las redes y las
funciones de S4 y S5. Quien hace cada estructura le pide a cada dueño de componente lo que cambió en el suyo.

## Forma de trabajo

- **Ramas.** `p3` es la rama de integración y cada integrante trabaja en `p3/<nombre>` (por ejemplo `p3/cristian`).
  Todo entra a `p3` por Pull Request; el lunes 23 `p3` pasa a `main`.
- **Orden de los cambios al compose.** `docker-compose.yml` lo tocan varias tareas, así que sus PR entran en este
  orden: S3 → S2 → S1 → límite de S4 en el proxy → ajustes de R4. Antes de abrir el PR, cada uno actualiza su rama
  con lo último de `p3` (`git pull --rebase origin p3`).
- **Evidencias.** Cada escenario guarda las suyas en `p3/sN/` (capturas, salidas de comandos y scripts para repetir la
  prueba) y el rendimiento en `p3/perf/`.
- **Un PR está listo cuando:** `docker compose up -d --build --wait` levanta todo, se puede iniciar sesión, votar y ver
  los resultados en vivo, las evidencias están en su carpeta y la sección del documento está escrita.

## Documento `p3_1b.md`

| Sección | Responsable |
|---|---|
| Equipo y sistema (incluida la completitud funcional) | Esteban |
| Estructura C&C | Miguel |
| Estructura de despliegue | Cristian |
| Estructura por capas | Maicol |
| Estructura de descomposición | Camilo |
| Seguridad: escenarios S1 a S5, con tácticas y patrones | El responsable de cada escenario |
| Rendimiento y escalabilidad: análisis y resultados | Maicol |
| Revisión final y exportación a PDF | Esteban |
