#!/usr/bin/env bash
# verify-b1-b2.sh — Casos B1 y B2 (anonimato del voto), Lab 5, equipo 1b
#   B1: agr-vote-lo  -> agr-voters-db (quién votó)   debe quedar bloqueado
#   B2: agr-voter-lo -> agr-votes-db  (los votos)    debe quedar bloqueado
#   Controles: F13 (voter-lo -> voters-db) y F14 (vote-lo -> votes-db) deben seguir abiertos
# Cada caso se prueba por nombre (DNS de Docker) y por cada IP real del destino.
# Uso: ./verify-b1-b2.sh [archivo-salida]   (main -> evidence/before/B1_B2.txt, lab5-security -> evidence/after/B1_B2.txt)
set -uo pipefail
OUT="${1:-}"
IMG="busybox:1.36"
docker image inspect "$IMG" >/dev/null 2>&1 || docker pull -q "$IMG" >/dev/null

ips() { docker inspect "$1" --format '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}'; }

check() { # caso origen destino puerto esperado(open|closed)
  local case=$1 src=$2 dst=$3 port=$4 exp=$5 dns tcp ipres="sin conexión" got ok
  dns=$(docker run --rm --network "container:$src" "$IMG" nslookup "$dst" 2>&1 | awk '/^Address/ && !/:53/{print $2}' | tr '\n' ' ')
  [[ -z "$dns" ]] && dns="no resuelve"
  tcp=$(docker run --rm --network "container:$src" "$IMG" nc -zv -w 2 "$dst" "$port" 2>&1 | tail -1)
  for ip in $(ips "$dst"); do
    if docker run --rm --network "container:$src" "$IMG" nc -z -w 2 "$ip" "$port" 2>/dev/null; then ipres="ABIERTO ($ip)"; break; fi
  done
  if [[ "$tcp" == *open* || "$ipres" == ABIERTO* ]]; then got=open; else got=closed; fi
  [[ "$got" == "$exp" ]] && ok="OK  " || ok="FALLA"
  printf '%s %-4s %-13s -> %-14s :%-5s DNS: %-16s TCP nombre: %-40s TCP IP: %s\n' "$ok" "$case" "$src" "$dst" "$port" "$dns" "$tcp" "$ipres"
}

{
  echo "B1/B2 · anonimato del voto"
  echo "$(date '+%Y-%m-%d %H:%M:%S %z') · rama $(git rev-parse --abbrev-ref HEAD 2>/dev/null) ($(git rev-parse --short HEAD 2>/dev/null))"
  echo
  for c in agr-vote-lo agr-voter-lo agr-votes-db agr-voters-db; do
    printf '  %-14s %s\n' "$c" "$(docker inspect "$c" --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}')"
  done
  echo
  exp=closed; [[ "${EXPECT_OPEN:-}" == 1 ]] && exp=open
  check B1  agr-vote-lo  agr-voters-db 5432  "$exp"
  check B2  agr-voter-lo agr-votes-db  27017 "$exp"
  check F13 agr-voter-lo agr-voters-db 5432  open
  check F14 agr-vote-lo  agr-votes-db  27017 open
} > >(if [[ -n "$OUT" ]]; then tee "$OUT"; else cat; fi)
wait
