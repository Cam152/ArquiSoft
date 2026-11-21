#!/usr/bin/env bash
# p3/perf/run.sh · ejecuta UNA prueba de k6 y guarda sus evidencias.
#
#   p3/perf/run.sh <etiqueta> <script.js> <BASE_URL> [VAR=valor ...]
#
# Ejemplos (desde la raíz del repo, con el sistema ya levantado):
#   p3/perf/run.sh r1-proxy r1-read.js  http://localhost/api
#   p3/perf/run.sh r2-proxy r2-write.js http://localhost/api LEVELS=10,50,100
#
# Guarda en p3/perf/results/<etiqueta>/:
#   summary.txt / summary.json   resumen de k6 por nivel (p50, p95, p99, req/s, errores)
#   k6.log                       salida completa de k6
#   docker-stats.csv             CPU y memoria de cada contenedor agr-*, una muestra cada ~2 s
#   docker-stats-summary.txt     promedio y máximo de CPU y memoria por contenedor
#
# k6 corre en Docker (grafana/k6), en la red del equipo, así que no hace falta
# instalarlo. Para correrlo desde otro PC (como en el Lab 6), use la IP del servidor
# en BASE_URL y lance solo el "docker run" de abajo desde ese PC.
set -euo pipefail

if [[ $# -lt 3 ]]; then
  sed -n '2,20p' "$0"; exit 1
fi
LABEL=$1 SCRIPT=$2 BASE_URL=$3; shift 3
K6_IMAGE=${K6_IMAGE:-grafana/k6:0.54.0}

PERF_DIR=$(cd "$(dirname "$0")" && pwd)
OUT="$PERF_DIR/results/$LABEL"
rm -rf "$OUT"; mkdir -p "$OUT"
chmod 777 "$OUT"   # k6 corre como usuario sin privilegios dentro del contenedor

ENV_ARGS=(-e "BASE_URL=$BASE_URL" -e "LABEL=$LABEL" -e "OUT_DIR=/out")
for kv in "$@"; do ENV_ARGS+=(-e "$kv"); done

# ---- docker stats en segundo plano -------------------------------------------
echo "timestamp,container,cpu_pct,mem_mib" >"$OUT/docker-stats.csv"
(
  while :; do
    ts=$(date +%s)
    docker stats --no-stream --format '{{.Name}},{{.CPUPerc}},{{.MemUsage}}' 2>/dev/null |
      grep 'agr-' |
      awk -F, -v ts="$ts" '{
        cpu=$2; sub(/%/,"",cpu);
        split($3, m, " / "); mem=m[1]; v=mem+0;
        if (mem ~ /GiB/) v*=1024; else if (mem ~ /KiB/) v/=1024; else if (mem ~ /[^M]B$/ && mem !~ /iB/) v/=1048576;
        printf "%s,%s,%s,%.1f\n", ts, $1, cpu, v }' >>"$OUT/docker-stats.csv"
  done
) &
STATS_PID=$!
trap 'kill $STATS_PID 2>/dev/null || true' EXIT

echo ">> $LABEL · $SCRIPT · $BASE_URL $*"
set +e
docker run --rm --network host --ulimit nofile=65535:65535 \
  -v "$PERF_DIR:/scripts:ro" -v "$OUT:/out" \
  "${ENV_ARGS[@]}" "$K6_IMAGE" run --quiet "/scripts/$SCRIPT" 2>&1 | tee "$OUT/k6.log"
K6_STATUS=${PIPESTATUS[0]}
set -e

kill $STATS_PID 2>/dev/null || true; wait $STATS_PID 2>/dev/null || true

# ---- Resumen de docker stats ------------------------------------------------
awk -F, 'NR>1 && $3!="" {
    n[$2]++; cpu[$2]+=$3; if ($3>cmax[$2]) cmax[$2]=$3; if ($4>mmax[$2]) mmax[$2]=$4
  } END {
    printf "%-34s %10s %10s %12s\n", "contenedor", "CPU prom %", "CPU máx %", "mem máx MiB"
    for (c in n) printf "%-34s %10.1f %10.1f %12.1f\n", c, cpu[c]/n[c], cmax[c], mmax[c]
  }' "$OUT/docker-stats.csv" | (read -r h; echo "$h"; sort -k3 -nr) >"$OUT/docker-stats-summary.txt"

echo
cat "$OUT/docker-stats-summary.txt"
echo ">> Evidencias en ${OUT#"$PWD"/}"
exit "$K6_STATUS"
