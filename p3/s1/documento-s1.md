# Texto de S1 y de la estructura C&C para `artifacts/p3_1b.md`

> Borrador de **Miguel** para que Esteban lo integre en el documento general. No reemplaza nada: `artifacts/p3_1b.md`
> todavía no existe en el repositorio. Los números de conectores son una **propuesta** que hay que acordar con
> Cristian (S2) antes de consolidar.

---

## A. Estructura de componentes y conectores (C&C) del Prototipo 3

### A.1 Qué cambia frente al Prototipo 2

El Prototipo 3 agrega **un componente**, `agr-proxy-rp` (S2), que pasa a ser la única entrada pública. El navegador ya
no habla con la web ni con el gateway directamente: lo hace por HTTPS con el proxy (S1), y el proxy reenvía por HTTP
dentro de la red de borde. La segmentación de redes (S3), el límite de intentos (S4) y las réplicas del gateway (R4)
no agregan componentes ni conectores nuevos. El certificado de S1 es un **artefacto de despliegue** (un archivo
montado en el contenedor), no un componente en ejecución, así que no aparece en la vista C&C.

**Componentes: 17** (los 16 del Prototipo 2 más `agr-proxy-rp`).

| # | Componente | Nivel | Tecnología | Responsabilidad |
|---|---|---|---|---|
| 17 | `agr-proxy-rp` | Entrada (borde) | Nginx (Alpine) | Único punto de entrada: termina TLS (S1), redirige el 80 al 443, reparte `/` hacia la web y `/api/` hacia el gateway, limita los intentos de login por IP (S4) y reparte entre réplicas del gateway (R4). |

### A.2 Conectores

Los conectores c1–c21 del Prototipo 2 se mantienen salvo estos dos cambios:

| Conector | Cambio |
|---|---|
| c1 (REST, `agr-web-fr` → `agr-gateway-or`) | Solo lo usa el **servidor** de Next.js (SSR): `GET /results/{id}`. El login y el voto del navegador pasan ahora por c22. |
| c3 (SSE, `agr-web-fr` → `agr-gateway-or`) | **Se retira como conexión directa:** el navegador ya no llega al gateway; el flujo SSE pasa por c23 y c25. |

Conectores nuevos (c22–c26):

| Conector | Tipo | Desde → Hacia | Puertos | Uso |
|---|---|---|---|---|
| c22 | REST sobre **HTTPS** (TLS 1.2/1.3) | `agr-web-fr` (código en el navegador) → `agr-proxy-rp` | 443 | `POST /api/auth/login`, `POST /api/votes` |
| c23 | SSE sobre **HTTPS** (TLS 1.2/1.3) | `agr-web-fr` (navegador) → `agr-proxy-rp` | 443 | `GET /api/results/{id}/stream` |
| c24 | **HTTPS** (TLS 1.2/1.3) | `agr-web-fr` (navegador) → `agr-proxy-rp` | 443 | Páginas, scripts y estilos de la web. Si el navegador usa `http://` (puerto 80), el proxy responde `301` al mismo recurso por HTTPS. |
| c25 | REST / SSE sobre HTTP | `agr-proxy-rp` → `agr-gateway-or` | 8080 | `/api/*` sin el prefijo `/api`; conexiones keepalive y varias réplicas (R4). |
| c26 | HTTP | `agr-proxy-rp` → `agr-web-fr` | 3000 | `/` (páginas SSR y recursos). |

**Total: 25 conectores** (21 del Prototipo 2, menos c3, más c22–c26). *Verificar con Cristian que c24–c26 coinciden
con los flujos F1, F2, F21 y F22 de `p3/s3/flujos-permitidos.md`.*

**Relaciones nuevas**

- El navegador solo se conecta con `agr-proxy-rp` (c22–c24). Los puertos 3000 y 8080 ya no se publican (S2, S3).
- Solo c22–c24 viajan por una red pública, y solo esos van cifrados. c25 y c26 viajan por `agr-edge-net`, que es
  `internal` (sin salida al equipo anfitrión ni a internet).
- El servidor de Next.js (SSR) y la CLI siguen llamando al gateway por `agr-edge-net` (c1, c2, c4).

> **Figura pendiente:** hay que redibujar la vista C&C (`p2_cc.png` → versión del Prototipo 3) con
> `agr-proxy-rp` entre el navegador y el gateway, y marcar c22–c24 como HTTPS.

---

## B. S1 · Secure Channel

### B.1 Escenario de calidad (formato de 6 partes)

| Parte | Descripción |
|---|---|
| **Fuente** | Un atacante en la misma red que el votante (Wi-Fi compartido, equipo intermedio) que observa o altera el tráfico. |
| **Estímulo** | Intercepta la comunicación entre el navegador y AGORA mientras alguien inicia sesión, vota o consulta resultados. |
| **Artefacto** | El canal navegador ↔ `agr-proxy-rp` (puertos 80 y 443). |
| **Entorno** | Operación normal, desplegada con Docker Compose, certificado de desarrollo. |
| **Respuesta** | El proxy solo atiende la aplicación por HTTPS con TLS 1.2 o 1.3, redirige el 80 al 443 y envía HSTS. Los protocolos anteriores (TLS 1.0 y 1.1) se rechazan. |
| **Medida de la respuesta** | (1) En la captura del puerto 443, **0** apariciones del documento, la clave y el token en texto legible; en la captura previa a S1 (puerto 80), aparecen. (2) TLS 1.2 y 1.3 aceptados y TLS 1.1 rechazado. (3) El 100 % de las respuestas HTTPS lleva `Strict-Transport-Security`. (4) HTTP responde `301` a la misma ruta por HTTPS. |

*Los valores medidos salen de `p3/s1/despues/` y `p3/s1/antes/` una vez ejecutados `verificar-tls.ps1` y
`capturar-trafico.ps1`; antes de pegarlos aquí hay que confirmar que las pruebas pasan.*

### B.2 Táctica y patrón

- **Táctica:** *Encrypt data* (cifrar los datos en tránsito), de la categoría *Resist attacks*.
- **Patrón:** *Secure Channel* con **terminación TLS en el reverse proxy**.

### B.3 Decisiones de diseño

1. **Se reutiliza el único proxy** (`agr-proxy-rp`, S2): no hay un segundo componente de seguridad.
2. **TLS 1.2 y 1.3, sin versiones anteriores**; en 1.2 solo suites ECDHE con AEAD (secreto perfecto hacia adelante).
3. **HTTP solo redirige** (`301`) y no lleva HSTS; **HTTPS lleva HSTS** (`max-age=31536000`) también en los errores.
4. **Certificado de desarrollo autofirmado** generado con `p3/s1/gen-certs.sh` (SAN: `localhost`, `127.0.0.1`, `::1`).
   La **llave privada no se versiona ni entra a la imagen**: queda en `p3/s1/certs/` (ignorada por Git) y se monta en
   solo lectura.
5. **Reanudación de sesiones** para que el costo del handshake no se pague en cada conexión (relevante para R3).

### B.4 Compromisos y riesgos

| Aspecto | Efecto |
|---|---|
| Rendimiento (R3) | El handshake TLS agrega costo por conexión nueva; se mitiga con keepalive (R4) y reanudación de sesiones. La medición con TLS la hace Maicol (R3). |
| Tráfico interno | Proxy → web y proxy → gateway siguen en HTTP dentro de `agr-edge-net` (`internal`). Riesgo residual aceptado; cifrarlo exigiría mTLS entre contenedores. |
| Primera petición por HTTP | Un cliente que empieza por `http://` envía esa primera petición en claro. HSTS lo evita en visitas posteriores del navegador. |
| Operación | El certificado autofirmado exige aceptar un aviso del navegador; en producción se usaría una CA. HSTS fija `localhost` en HTTPS en el navegador del desarrollador. |

### B.5 Evidencias

Carpeta `p3/s1/`: `antes/` (captura del login en HTTP), `despues/` (captura en HTTPS, `curl -v`, versiones de TLS,
redirección, HSTS, llave fuera de Git y de la imagen, compatibilidad con S2, S3, S4 y R4) y el `README.md` con los
comandos para reproducirlas.

---

## C. Cambios que dependen de otros miembros del equipo

| Quién | Qué | Por qué |
|---|---|---|
| Esteban | Crear `artifacts/p3_1b.md` e incorporar las secciones A y B. | El archivo no existe en ninguna rama. |
| Cristian | Confirmar la numeración c22–c26 y los flujos F1, F2, F21 y F22. | Su proxy es el sujeto de esos conectores. |
| Maicol | Repetir r1/r2/r4 con `PROXY_URL=https://localhost/api` y comparar con y sin TLS en R3. | Es el pendiente que su README deja explícito para S1. |
| Esteban | Actualizar `p3/s4/test-brute-force.sh` (`BASE_URL` e `-k`). | Ya no existe `localhost:8080` y el 80 redirige. |
| Camilo | Entregar S5. | No hay código de S5 en `main`. |
