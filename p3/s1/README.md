# S1 · Secure Channel (TLS en `agr-proxy-rp`)

**Responsable:** Miguel Angel Zamudio Villa. **Patrón:** Secure Channel (táctica *Encrypt data*).
**Rama:** `p3-miguel` → PR hacia `p3`.

S1 cifra con TLS el único tramo de AGORA que sale del equipo anfitrión: **navegador ↔ `agr-proxy-rp`**. No crea otro
proxy: reutiliza el de S2 (Cristian), conserva la segmentación de S3 (David), el límite de S4 (Esteban) y el balanceo
y keepalive de R4 (Maicol).

## Qué cambió

| Archivo | Cambio |
|---|---|
| `p3/s1/gen-certs.sh` | **Nuevo.** Genera con OpenSSL la llave y el certificado autofirmado de desarrollo (SAN: `localhost`, `127.0.0.1`, `::1`). |
| `agr-proxy-rp/nginx.conf` | El `server` pasa a `listen 443 ssl`; TLS 1.2 y 1.3; suites ECDHE con AEAD; caché de sesiones; HSTS. Nuevo `server` en el 80 que solo hace `return 301` a HTTPS. Todo lo de S2, S4 y R4 queda igual. |
| `agr-proxy-rp/Dockerfile` | `EXPOSE 80 443` y, al arrancar, un aviso claro si faltan los certificados. **No copia** certificados a la imagen. |
| `docker-compose.yml` | `agr-proxy-rp` publica también `443:443` y monta `./p3/s1/certs` en `/etc/nginx/certs` como **solo lectura**. Redes, `ports` del gateway y de la web: sin cambios. |
| `.gitignore` | Ignora `p3/s1/certs/`, `*.key` y `*.crt`. |
| `.env.example` | Comentarios actualizados (HTTPS). |
| `.github/workflows/ci.yml` | El job `system` genera el certificado antes del `up` y prueba por `https://localhost` (`--insecure`); agrega pruebas de redirección, HSTS, TLS 1.2/1.3 y de que la llave no está en la imagen. |
| `p3/s1/verificar-tls.ps1` | **Nuevo.** Verifica S1 y la compatibilidad con S2, S3, S4 y R4; guarda la evidencia en `p3/s1/despues/`. |
| `p3/s1/capturar-trafico.ps1` | **Nuevo.** Captura el login en el 80 (antes) y en el 443 (después). |
| `p3/s1/documento-s1.md` | **Nuevo.** Texto de S1 y de la estructura C&C listo para pasar a `artifacts/p3_1b.md`. |

## Cómo ejecutarlo (paso a paso)

Desde la raíz del repositorio, en `p3-miguel`:

```powershell
# 1. Certificado de desarrollo (una sola vez por equipo). Necesita Git Bash.
& "C:\Program Files\Git\bin\bash.exe" p3/s1/gen-certs.sh

# 2. Levantar el sistema
docker compose up -d --build --wait

# 3. Abrir https://localhost y aceptar el aviso del navegador (certificado autofirmado)
```

Si no tiene Git Bash, el mismo script corre dentro de Docker:

```powershell
docker run --rm -v "${PWD}:/repo" -w /repo alpine sh -c "apk add --no-cache openssl bash >/dev/null && bash p3/s1/gen-certs.sh"
```

Si el proxy no arranca con el mensaje *Faltan los certificados de S1*, falta el paso 1.

> **HSTS y `localhost`.** Después de entrar por HTTPS, el navegador recordará durante un año que `localhost` solo
> se visita por HTTPS. Si otro proyecto suyo usa `http://localhost`, en Chrome/Edge se borra esa regla en
> `chrome://net-internals/#hsts` (*Delete domain security policies* → `localhost`).

## Verificación

```powershell
powershell -ExecutionPolicy Bypass -File .\p3\s1\verificar-tls.ps1
powershell -ExecutionPolicy Bypass -File .\p3\s1\verificar-tls.ps1 -ProbarEscalado   # además, R4 con 3 réplicas
```

Resultado en `p3/s1/despues/00_resumen.txt` (OK / FALLA / INFO) y un archivo por prueba. Las pruebas de TLS usan la
imagen `alpine/openssl`, así que no hace falta instalar OpenSSL en Windows.

## Evidencias

| Evidencia | Cómo se obtiene | Dónde queda |
|---|---|---|
| Tráfico **antes** de S1 (HTTP, credenciales legibles) | En `main`: `capturar-trafico.ps1 -Fase antes -Puerto 80` | `p3/s1/antes/` |
| Tráfico **después** de S1 (HTTPS, cifrado) | En `p3-miguel`: `capturar-trafico.ps1 -Fase despues -Puerto 443` | `p3/s1/despues/` |
| `curl -v` con el certificado | `verificar-tls.ps1` | `08_curl_verbose.txt` |
| TLS 1.2 y 1.3 sí; TLS 1.1 no | `verificar-tls.ps1` | `04_versiones_tls.txt` |
| Redirección HTTP → HTTPS | `verificar-tls.ps1` | `05_redireccion_http_a_https.txt` |
| HSTS | `verificar-tls.ps1` | `06_hsts.txt` |
| Llave no versionada ni en la imagen | `verificar-tls.ps1` | `01_llave_fuera_de_git_y_de_la_imagen.txt` |

Para la captura *antes* hay que estar en `main` (volver con `git switch p3-miguel` al terminar). Las carpetas
`antes/` y `despues/` sí se versionan; `certs/` no.

## Decisiones

1. **TLS se termina en el proxy.** Detrás, `agr-edge-net` es `internal` (S3): sin salida al host ni a internet. El
   tráfico proxy → web y proxy → gateway sigue en HTTP. Cifrarlo de extremo a extremo (mTLS) queda fuera del alcance
   y se registra como riesgo residual.
2. **Solo TLS 1.2 y 1.3.** En 1.2, únicamente ECDHE con AES-GCM o ChaCha20-Poly1305 (secreto perfecto hacia adelante).
3. **Certificado RSA 2048 autofirmado, 365 días.** Es de desarrollo: no es de confianza pública. En producción lo
   emitiría una CA.
4. **Llave y certificado fuera de Git y de la imagen.** Se generan en cada equipo, se montan en solo lectura y el
   Dockerfile no los copia. El contexto de build (`agr-proxy-rp/`) ni siquiera contiene `p3/s1/certs/`.
5. **Redirección `301`.** El `server` del 80 no tiene `proxy_pass` ni HSTS: no llega nada a la web ni al gateway.
6. **HSTS `max-age=31536000`**, sin `includeSubDomains` ni `preload` (es `localhost`).
7. **Reanudación de sesiones TLS** (`ssl_session_cache`) para abaratar el handshake en R3; sin tickets para no
   debilitar el secreto perfecto.
8. **El 443 es fijo.** `PROXY_HTTP_PORT` solo cambia el puerto del host para el 80; la redirección siempre apunta al 443.

## Compatibilidad con los demás escenarios

| Con | Qué se conserva |
|---|---|
| **S2** (Cristian) | Un solo `agr-proxy-rp`; `/` → web, `/api/` → gateway sin prefijo; SSE sin buffering; límite de 1 MB; encabezados de seguridad; sin `Server` ni `X-Powered-By`; `/api/admin/` sin exponer. |
| **S3** (David) | Mismas redes y flujos. Solo se agrega el puerto 443 al proxy. `agr-edge-net` sigue `internal`; el gateway y la web siguen sin publicar puertos. |
| **S4** (Esteban) | `limit_req_zone`, `limit_req zone=login_limit` y `limit_req_status 429` intactos. |
| **S5** (Camilo) | No se tocó auditoría. *Nota:* en `main` aún no hay código de S5. |
| **R1–R4** (Maicol) | `upstream agr_gateway` con `resolve` y `keepalive 64`, sin `container_name` ni puertos en el gateway: intactos. `--scale agr-gateway-or=3` sigue funcionando detrás del mismo Nginx. |

### Cómo correr las pruebas de Maicol con S1

No se modificó ningún archivo suyo. El proxy ya no sirve la aplicación por HTTP, así que sus scripts deben apuntar a
HTTPS (aceptan certificado autofirmado):

```bash
PROXY_URL=https://localhost/api p3/perf/run-all.sh
```

Si se usara `http://localhost/api`, el POST de R2 (login y voto) recibiría un `301` y se convertiría en GET.
Pendiente de Maicol (no ejecutado por S1): repetir `r1-proxy`, `r2-proxy` y `r4-3x` con TLS y agregar la comparación
con y sin TLS a R3, tal como lo deja anotado su README.

### Scripts de otros escenarios que ahora necesitan HTTPS

- `p3/s2/verificar-proxy.ps1`: ejecutarlo con `-Base https://localhost -Inseguro`.
- `p3/s4/test-brute-force.sh`: su `BASE_URL` por defecto (`localhost:8080`) ya no existe desde S2 y, con S1, `curl`
  necesita `-k`. No se modificó; se lo señalo a Esteban.

## Límites conocidos

- Un cliente que empiece por `http://` envía su **primera** petición en claro antes de recibir la redirección. HSTS
  evita que el navegador repita ese error después de la primera visita. `capturar-trafico.ps1 -Fase despues -Puerto 80`
  lo documenta.
- El certificado autofirmado hace que el navegador muestre un aviso hasta que se acepta o se importa `localhost.crt`.
- Las pruebas del job `system` de CI comprueban la configuración de TLS 1.1 (solo 1.2 y 1.3) pero no ofrecen un
  handshake 1.1, porque el `curl` de Ubuntu lo bloquea del lado cliente. El rechazo real se prueba en
  `verificar-tls.ps1` (punto 04).
