# S5 · Auditoría a prueba de alteraciones (`agr-audit-lo`)

**Responsable:** Camilo. **Patrón:** Tamper-evident log (registro con evidencia de alteración), con las tácticas
*Verify message integrity* y *Detect intrusion*. **Rama:** `p3-camilo` → PR hacia `p3`.

En el Prototipo 2, quien entraba a `agr-audit-db` podía cambiar, borrar o insertar eventos y nadie se enteraba: la
CLI mostraba el registro alterado como si fuera real. S5 encadena cada evento con el anterior mediante un
**HMAC-SHA256** y agrega una verificación que recorre la cadena y señala **qué evento** fue tocado y **cómo**.

## Qué cambió

| Archivo | Cambio |
|---|---|
| `agr-audit-lo/Services/HashChain.cs` | **Nuevo.** `hash = HMAC-SHA256(AUDIT_CHAIN_KEY, ["agora-audit-v1", seq, event_id, type, election_id, occurred_at, recorded_at, prev_hash])`. El primer evento usa `prev_hash` = 64 ceros. Comparación en tiempo constante. |
| `agr-audit-lo/Models/AuditEvent.cs` | Campos nuevos `seq`, `prev_hash` y `hash`. Siguen sin guardarse datos del votante ni del candidato. |
| `agr-audit-lo/Services/AuditService.cs` | Al registrar: toma la cabeza de la cadena, calcula el hash y guarda (una escritura a la vez). `VerifyAsync` recorre toda la cadena. Al arrancar, si la cadena está vacía, encadena una sola vez los eventos de antes de S5. |
| `agr-audit-lo/Repositories/*` | Índice único `seq_unique` (parcial). Consulta de la cabeza, recorrido completo en orden de `seq` y encadenado de eventos previos. |
| `agr-audit-lo/Controllers/AuditController.cs` | **`GET /audit/verify`**. `GET /audit/events` devuelve también `seq`, `prev_hash` y `hash`. |
| `agr-audit-lo/Consumers/VoteRegisteredConsumer.cs` | El log de cada evento incluye `seq` y `hash` (una copia de la cabeza fuera de la base). |
| `agr-gateway-or/src/routes/index.ts` | **`GET /admin/audit/verify`** (con `X-API-Key`) → `agr-audit-lo /audit/verify`. |
| `agr-admin-fr` | **`agora audit verify`**: muestra el resultado y termina con código 1 si la auditoría fue alterada. Acepta `--json`. |
| `docker-compose.yml`, `.env.example` | `AUDIT_CHAIN_KEY` (mínimo 32 caracteres) solo en `agr-audit-lo`. Sin ella el servicio no arranca. |
| `.github/workflows/ci.yml` | El job `system` corre `agora audit verify`. |
| Pruebas | `agr-audit-lo`: 52 pruebas (nuevas en `HashChainTests.cs`: cada tipo de alteración, llave falsa, reinicio, carrera por el `seq`, eventos previos). Gateway: 2 pruebas nuevas. CLI: 2 pruebas nuevas. |

## Qué detecta `GET /audit/verify`

| `kind` | Qué pasó en la base | Cómo se detecta |
|---|---|---|
| `altered` | Se cambió un campo de un evento | El hash recalculado con la llave no coincide con el guardado |
| `altered` | Se cambió un campo **y** se recalculó el hash sin la llave (SHA-256 simple o una llave adivinada) | Ídem: sin `AUDIT_CHAIN_KEY` no se puede producir un HMAC válido |
| `missing` | Se borró uno o varios eventos de en medio | Hay un salto en `seq` |
| `broken_link` | Se cambió `prev_hash` | `prev_hash` no es el hash del evento anterior |
| `out_of_order` | `seq` repetido o hacia atrás (por ejemplo, tras borrar el índice) | El recorrido en orden de `seq` no avanza |
| `unchained` | Se insertó un evento a mano | El documento no tiene `seq` ni `hash` |
| `truncated` | Se borraron los **últimos** eventos | La base termina antes que la cabeza que el servicio tiene en memoria |

Respuesta (resumida):

```json
{
  "valid": false, "total_events": 6, "chained_events": 6, "head_seq": 6, "head_hash": "b6f5…b3e2",
  "known_head_seq": 6, "checked_at": "2026-11-23T01:00:58Z", "problem_count": 1,
  "problems": [{ "kind": "altered", "seq": 3, "event_id": "…", "detail": "El hash no coincide con el contenido: el evento fue modificado" }]
}
```

## Cómo ejecutarlo

Desde la raíz del repositorio, en `p3-camilo` (con el certificado de S1 ya generado):

```powershell
docker compose up -d --build --wait
docker compose --profile tools run --rm agr-admin-fr audit verify
```

### Evidencias

```powershell
# Después (esta rama): 5 ataques contra agr-audit-db y su detección
powershell -ExecutionPolicy Bypass -File .\p3\s5\verificar-auditoria.ps1

# Antes (Prototipo 2): con el sistema de main corriendo, desde su checkout
powershell -ExecutionPolicy Bypass -File .\p3\s5\verificar-auditoria.ps1 -Fase antes -ComposeDir <carpeta de main>
```

El script publica 5 eventos de prueba **solo** en la cola `audit.vote-registered` (exchange por defecto de RabbitMQ:
no llegan a `agr-results-lo` ni cambian resultados), ataca la base con `mongosh` dentro de `agr-audit-db` (con el
usuario de la base, como lo haría un intruso), verifica con la CLI y **restaura** cada documento, así que se puede
repetir. Resultados en `p3/s5/antes/` y `p3/s5/despues/` (`00_resumen.txt` + un archivo por prueba).

| Fase | Resultado |
|---|---|
| Antes (`main`) | El evento movido a la elección 999 aparece en `GET /admin/audit?election_id=999` como un voto normal; `GET /admin/audit/verify` → 404 |
| Después (`p3-camilo`) | 5/5 ataques detectados con el `seq` exacto; la cadena vuelve a `valid: true` al restaurar; 0 falsos positivos |

## Límites conocidos

- **La llave es el secreto.** Quien tenga `AUDIT_CHAIN_KEY` (o entre al contenedor `agr-audit-lo`) puede rehacer la
  cadena. La defensa es que la llave no está en la base ni en la red de datos; en producción iría en un gestor de
  secretos. Si se cambia la llave, los eventos ya guardados dejan de verificar.
- **Truncamiento tras un reinicio.** La cabeza en memoria se carga de la base al arrancar: si alguien borra los
  últimos eventos y luego se reinicia el servicio, `truncated` no se detecta. El `seq` y el `hash` de cada evento
  quedan en el log del contenedor (`docker compose logs agr-audit-lo`), que sirve para compararlo a mano. Publicar
  la cabeza periódicamente en otro sistema (anclaje externo) cerraría este hueco.
- **Detección, no prevención.** El patrón no impide cambiar la base (eso lo hace S3, que la deja en su propia red
  `internal`); hace que el cambio no pase desapercibido.
- Una sola réplica de `agr-audit-lo` escribe a la vez; si hubiera varias, el índice único de `seq` hace que la
  segunda reintente con la cabeza nueva.
