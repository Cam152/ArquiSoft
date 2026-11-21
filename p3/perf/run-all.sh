#!/usr/bin/env bash
# p3/perf/run-all.sh · corre R1 a R4 de punta a punta y genera las gráficas.
#
#   p3/perf/run-all.sh            (desde la raíz del repo; ~25 min)
#
# Orden:
#   1. Línea base (R3): sin proxy, el gateway publica el 8080   -> r1-base, r2-base
#   2. Con proxy y límite de login subido (docker-compose.perf.yml)
#                                                                -> r1-proxy (= R4 con 1 réplica), r2-proxy
#   3. Con proxy y 3 réplicas del gateway (R4)                   -> r4-3x
#   4. Deja AGORA como estaba (compose normal, 1 réplica, límites de S4)
#   5. make_charts.py
#
# Variables opcionales: R1_LEVELS, R2_LEVELS, LEVEL_DURATION, VOTERS, PROXY_URL, SKIP_BUILD=1.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd); cd "$ROOT"
PERF=p3/perf
R1_LEVELS=${R1_LEVELS:-50,200,500,1000,1500}
R2_LEVELS=${R2_LEVELS:-10,25,50,100,200}
LEVEL_DURATION=${LEVEL_DURATION:-30s}
VOTERS=${VOTERS:-40000}
# Con S1 (TLS) usar PROXY_URL=https://localhost/api: el 80 redirige con 301 y un POST redirigido llega como GET.
PROXY_URL=${PROXY_URL:-http://localhost/api}
BUILD=--build; [[ "${SKIP_BUILD:-0}" == 1 ]] && BUILD=

BASE=(docker compose -f docker-compose.yml -f "$PERF/docker-compose.baseline.yml")
PROXY=(docker compose -f docker-compose.yml -f "$PERF/docker-compose.perf.yml")

seed_voters() {
  echo ">> Sembrando $VOTERS votantes de carga en agr-auth-db"
  docker exec -i agr-auth-db psql -q -U postgres -d agr_auth_db -v n="$VOTERS" -v prefix=9 <"$PERF/seed-voters.sql"
}

# R2 usa cada votante una sola vez; antes de cada R2 se borra su participación
# en agr-voters-db para poder reusarlos (los votos anónimos y los conteos quedan).
reset_ballots() {
  local first
  first=$(docker exec agr-auth-db psql -tA -U postgres -d agr_auth_db \
    -c "SELECT min(id) FROM voters WHERE document ~ '^9[0-9]{6}$'")
  docker exec agr-voters-db psql -q -U voters -d voters \
    -c "DELETE FROM ballots_cast WHERE voter_id >= $first" >/dev/null
  echo ">> Participación de los votantes de carga reiniciada (voter_id >= $first)"
}

settle() { echo ">> Pausa de 20 s para que el sistema se estabilice"; sleep 20; }

# ---- 1. Línea base ----------------------------------------------------------
echo "== Línea base: sin proxy =="
"${BASE[@]}" up -d $BUILD --wait --scale agr-proxy-rp=0
seed_voters; reset_ballots; settle
"$PERF/run.sh" r1-base r1-read.js  http://localhost:8080 LEVELS="$R1_LEVELS" LEVEL_DURATION="$LEVEL_DURATION"
settle
"$PERF/run.sh" r2-base r2-write.js http://localhost:8080 LEVELS="$R2_LEVELS" LEVEL_DURATION="$LEVEL_DURATION" VOTERS="$VOTERS"

# ---- 2. Con proxy -----------------------------------------------------------
echo "== Con proxy (1 réplica del gateway) =="
"${PROXY[@]}" up -d --wait --scale agr-proxy-rp=1 --scale agr-gateway-or=1
reset_ballots; settle
"$PERF/run.sh" r1-proxy r1-read.js  "$PROXY_URL" LEVELS="$R1_LEVELS" LEVEL_DURATION="$LEVEL_DURATION"
settle
"$PERF/run.sh" r2-proxy r2-write.js "$PROXY_URL" LEVELS="$R2_LEVELS" LEVEL_DURATION="$LEVEL_DURATION" VOTERS="$VOTERS"

# ---- 3. R4: 3 réplicas del gateway ------------------------------------------
echo "== R4: 3 réplicas del gateway =="
"${PROXY[@]}" up -d --wait --scale agr-gateway-or=3
REPLICAS=$(docker ps --filter label=com.docker.compose.service=agr-gateway-or --format '{{.Names}}\t{{.Status}}')
echo "$REPLICAS"
settle
SINCE=$(date +%s)
"$PERF/run.sh" r4-3x r1-read.js "$PROXY_URL" LEVELS="$R1_LEVELS" LEVEL_DURATION="$LEVEL_DURATION"
echo "$REPLICAS" >"$PERF/results/r4-3x/replicas.txt"
# Reparto de carga: qué réplica atendió cada petición (columna up= del log del proxy)
docker logs --since "$SINCE" agr-proxy-rp 2>/dev/null | grep -oE 'up=[0-9.:]+' | sort | uniq -c |
  tee "$PERF/results/r4-3x/reparto-replicas.txt"

# ---- 4. Volver al despliegue normal -----------------------------------------
echo "== Restaurando AGORA (1 réplica, límites de S4) =="
docker compose up -d --wait --scale agr-gateway-or=1

# ---- 5. Gráficas -------------------------------------------------------------
PY=python3; [[ -x .venv/bin/python ]] && PY=.venv/bin/python
"$PY" "$PERF/make_charts.py"
