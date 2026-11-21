-- p3/perf/seed-voters.sql · votantes para R2 (login y voto bajo carga)
--
-- Inserta N votantes en agr-auth-db con documentos PREFIX + 6 dígitos
-- (por defecto 9000001 ... 9040000). Todos usan el mismo hash BCrypt que el
-- votante de prueba 1001, así que su clave es 123456. Es idempotente: si el
-- documento ya existe, se deja como está y se limpia su bloqueo de S4.
--
-- Uso (desde la raíz del repo):
--   docker exec -i agr-auth-db psql -U postgres -d agr_auth_db -v n=40000 -v prefix=9 \
--     < p3/perf/seed-voters.sql
--
-- La tabla voters la crea agr-auth-lo al arrancar (JPA ddl-auto=update), así que
-- el servicio debe haber arrancado al menos una vez.

\set ON_ERROR_STOP on
\if :{?n}
\else
  \set n 40000
\endif
\if :{?prefix}
\else
  \set prefix 9
\endif

BEGIN;

-- Bases creadas antes de S4 no tienen las columnas del bloqueo de cuenta (ver
-- p3/s2/README.md). Si ya existen, no hace nada.
ALTER TABLE voters ADD COLUMN IF NOT EXISTS failed_attempts integer NOT NULL DEFAULT 0,
                   ADD COLUMN IF NOT EXISTS locked_until timestamp(6);

INSERT INTO voters (document, name, password_hash, failed_attempts, locked_until)
SELECT :'prefix' || lpad(g::text, 6, '0'),
       'Votante de carga ' || g,
       ref.password_hash,
       0,
       NULL
FROM generate_series(1, :n) AS g
CROSS JOIN (SELECT password_hash FROM voters WHERE document = '1001') AS ref
ON CONFLICT (document) DO UPDATE SET failed_attempts = 0, locked_until = NULL;

COMMIT;

SELECT count(*) AS votantes_de_carga,
       min(id)  AS primer_id,
       max(id)  AS ultimo_id
FROM voters
WHERE document LIKE :'prefix' || '______' AND length(document) = length(:'prefix') + 6;
