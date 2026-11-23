# Texto de S5 y de la estructura de descomposición para `artifacts/p3_1b.md`

> Borrador de **Camilo** para que Esteban lo integre en el documento general. Los números de las funcionalidades
> siguen la tabla del Prototipo 2 (sección 3.4) y solo se agregan las nuevas.

---

## A. Estructura de descomposición del Prototipo 3

### A.1 Qué cambia frente al Prototipo 2

Los seis módulos del Prototipo 2 se conservan. El Prototipo 3 agrega **un módulo**, *(7) Acceso seguro*, que
implementa `agr-proxy-rp`, y **nuevas funcionalidades** en *(1) Autenticación* (S4) y *(6) Auditoría* (S5). La
segmentación de redes (S3) no es una función del sistema sino una decisión de despliegue, así que no aparece en esta
vista (se describe en la estructura de despliegue).

> **Figura pendiente:** actualizar `p2_descomposicion.png` → `p3_descomposicion.png` con el módulo (7) y las
> funcionalidades nuevas marcadas.

### A.2 Elementos y relaciones

| Elemento | Tipo | Implementado en | Descripción | Relaciones |
|---|---|---|---|---|
| (1) Autenticación | Módulo | `agr-auth-lo` | Inicio de sesión, manejo del JWT y **bloqueo de cuentas** (S4). | Votación lo usa para validar el token. (7) limita los intentos de login por IP antes de que lleguen aquí. |
| (2) Gestión de elecciones | Módulo | `agr-election-lo` | Sin cambios. | Igual que en el Prototipo 2. |
| (2.1) Elecciones | Submódulo | `agr-election-lo` | Sin cambios. | Parte de (2). |
| (2.2) Candidatos | Submódulo | `agr-election-lo` | Sin cambios. | Parte de (2). |
| (3) Participación | Módulo | `agr-voter-lo` | Sin cambios. | Igual que en el Prototipo 2. |
| (4) Votación | Módulo | `agr-gateway-or` (saga) y `agr-vote-lo` | Sin cambios. | Produce los eventos que consumen (5) y (6). Desde el navegador se llega a través de (7). |
| (5) Resultados | Módulo | `agr-results-lo` y `agr-gateway-or` | Sin cambios. | El flujo SSE llega al navegador a través de (7). |
| (6) Auditoría | Módulo | `agr-audit-lo` | Registra los eventos de voto en una **cadena de hashes** y **verifica su integridad** (S5). | Consume los eventos de (4). La CLI consulta y verifica a través del gateway. |
| (6.1) Registro | Submódulo | `agr-audit-lo` | Guarda y encadena cada evento. | Parte de (6). |
| (6.2) Verificación de integridad | Submódulo | `agr-audit-lo`, `agr-gateway-or` y `agr-admin-fr` | Recorre la cadena y señala eventos alterados, borrados o insertados. | Parte de (6). Usa lo que guarda (6.1). |
| **(7) Acceso seguro** | **Módulo nuevo** | `agr-proxy-rp` | Único punto de entrada: canal cifrado (S1), enrutamiento y encabezados de seguridad (S2), límite de login por IP (S4) y reparto entre réplicas del gateway (R4). | Está delante de (1), (4) y (5) para todo lo que viene del navegador. La CLI no pasa por él (usa la red interna). |

### A.3 Funcionalidades nuevas o modificadas

| Código | Funcionalidad | Regla de negocio |
|---|---|---|
| 1.4 | Contar intentos fallidos de login *(nueva, S4)* | Cada clave incorrecta suma uno; un login correcto reinicia la cuenta. |
| 1.5 | Bloquear la cuenta *(nueva, S4)* | Al quinto intento fallido la cuenta queda bloqueada 15 minutos; mientras tanto responde 423. |
| 6.1.1 | Registrar cada evento de voto *(antes 6.1)* | `_id` igual al `event_id`; sin votante ni candidato. |
| 6.1.2 | Encadenar el evento *(nueva, S5)* | Recibe el siguiente `seq` y `prev_hash` = hash del evento anterior (64 ceros en el primero). `hash` = HMAC-SHA256 con una llave que solo conoce `agr-audit-lo`. Un `seq` no se repite (índice único). |
| 6.1.3 | Encadenar eventos previos a S5 *(nueva, S5)* | Solo al arrancar y solo si la cadena está vacía; después, un documento sin `seq` se considera insertado a mano. |
| 6.1.4 | Consultar eventos de auditoría *(antes 6.2)* | Desde la CLI (`agora audit list`), con la llave de administración; ahora incluye `seq` y `hash`. |
| 6.2.1 | Verificar la cadena *(nueva, S5)* | `GET /admin/audit/verify` y `agora audit verify`. Señala `altered`, `missing`, `broken_link`, `out_of_order`, `unchained` y `truncated`, con el `seq` del evento. La CLI termina con código 1 si hay problemas. |
| 7.1 | Cifrar el canal *(nueva, S1)* | Solo HTTPS con TLS 1.2 o 1.3 y HSTS; el puerto 80 responde 301 a HTTPS. |
| 7.2 | Enrutar peticiones *(nueva, S2)* | `/` va a la web y `/api/` al gateway sin el prefijo; `/api/admin` no se expone; SSE sin buffer. |
| 7.3 | Proteger la entrada *(nueva, S2)* | Máximo 1 MB por petición; encabezados de seguridad; sin `Server` ni `X-Powered-By`. |
| 7.4 | Limitar el login por IP *(nueva, S4)* | `limit_req` en `/api/auth/login`; al superarlo responde 429. |
| 7.5 | Repartir entre réplicas del gateway *(nueva, R4)* | Resuelve `agr-gateway-or` por DNS de Docker y mantiene conexiones keepalive. |

### A.4 Patrones

Se mantienen los del Prototipo 2 (descomposición funcional jerárquica, por capacidad de negocio, alta cohesión y bajo
acoplamiento, responsabilidad única y separación de responsabilidades). Con el Prototipo 3:

- La seguridad del borde queda en **un módulo propio**, (7), en lugar de repartirse entre la web y el gateway
  (separación de responsabilidades).
- Integridad y registro de la auditoría quedan en el mismo módulo, (6): quien guarda los eventos es quien sabe
  verificarlos, porque es el único que tiene la llave (alta cohesión).

---

## B. S5 · Auditoría a prueba de alteraciones

### B.1 Escenario de calidad (formato de 6 partes)

| Parte | Descripción |
|---|---|
| **Fuente** | Un intruso o alguien de adentro con acceso a `agr-audit-db` (usuario y clave de la base), por ejemplo tras comprometer un contenedor de la red de datos de auditoría. |
| **Estímulo** | Cambia, borra o inserta eventos directamente en la colección `events` para ocultar o inventar votos. |
| **Artefacto** | El registro de auditoría: `agr-audit-lo` y `agr-audit-db`. |
| **Entorno** | Operación normal, con elecciones abiertas o cerradas. |
| **Respuesta** | El sistema detecta la alteración: `GET /admin/audit/verify` (CLI `agora audit verify`) recorre la cadena de hashes y señala el evento afectado y el tipo de alteración. Los datos del votante siguen sin estar en la auditoría. |
| **Medida de la respuesta** | (1) **5 de 5** tipos de ataque detectados (modificar, modificar y rehacer el hash sin la llave, borrar en medio, insertar, borrar el último), cada uno con su `seq` exacto. (2) **0 falsos positivos:** la cadena intacta y la restaurada dan `valid: true`. (3) La detección ocurre en la siguiente verificación, que la CLI hace en una sola orden y el CI corre en cada build. En el Prototipo 2, **0** de los ataques se detectaban. |

Evidencias: `p3/s5/antes/` (Prototipo 2: el evento alterado se lista como real y `/admin/audit/verify` responde 404)
y `p3/s5/despues/` (los cinco ataques y su detección), generadas con `p3/s5/verificar-auditoria.ps1`.

### B.2 Tácticas y patrón

- **Tácticas** (categoría *Detect attacks*): *Verify message integrity* (HMAC-SHA256 de cada evento) y *Detect
  intrusion* (la verificación compara la cadena con lo esperado y nombra el evento alterado). Como apoyo, *Limit
  access* (S3: `agr-audit-db` solo comparte red con `agr-audit-lo`) y *Separate entities* (la llave vive en el
  servicio, no en la base).
- **Patrón:** *Tamper-evident log* (registro encadenado por hashes, la misma idea de un *hash chain*).

### B.3 Decisiones de diseño

| Decisión | Por qué |
|---|---|
| HMAC-SHA256 con llave en lugar de SHA-256 simple | Con SHA-256 simple, quien entra a la base puede recalcular toda la cadena después de cambiarla. Con HMAC necesita además la llave, que está solo en `agr-audit-lo`. El ataque B de la evidencia lo comprueba. |
| `seq` explícito además de `prev_hash` | Permite decir exactamente qué eventos faltan ("faltan los eventos seq 2 a 3") en lugar de solo "la cadena está rota". |
| Cabeza de la cadena en memoria | Borrar los últimos eventos no deja huecos; la cabeza que conoce el servicio sí lo delata (`truncated`). |
| Campos como arreglo JSON antes del hash | Evita ambigüedades al concatenar (un `event_id` con separadores no puede hacerse pasar por otro campo). |
| Encadenar los eventos del Prototipo 2 solo si la cadena está vacía | Migra los datos existentes una vez, sin "lavar" después un documento insertado a mano. |
| Verificación bajo la llave de administración | Es una operación del administrador, como `audit list`; no se expone por el proxy (`/api/admin` es 404). |

### B.4 Límites

- Quien tenga la llave `AUDIT_CHAIN_KEY` o control del contenedor `agr-audit-lo` puede rehacer la cadena.
- Si se borran los últimos eventos y luego se reinicia el servicio, el truncamiento no se detecta (la cabeza se
  vuelve a leer de la base). El log del contenedor conserva `seq` y `hash` de cada evento; un anclaje externo de la
  cabeza lo resolvería.
- El patrón detecta, no previene: evitar el acceso a la base es tarea de S3.
