#!/usr/bin/env bash
# =============================================================================
# B5 · agr-web-fr solo alcanza a agr-gateway-or
# Laboratorio 5 (Network Segmentation) · Equipo 1b · Responsable: Cristian
#
# Comprueba, desde el espacio de red de agr-web-fr, que:
#   1. agr-web-fr está solo en agr-edge-net, publica solo el 3000 y no guarda
#      secretos en su entorno;
#   2. el flujo permitido F3 (agr-web-fr -> agr-gateway-or:8080) funciona;
#   3. ningún *-lo, *-db ni el broker resuelve ni conecta por nombre;
#   4. tampoco conectan por IP: el aislamiento no depende solo del DNS;
#   5. ningún puerto publicado en el host abre un camino que salte la
#      segmentación;
#   6. (informativo) si agr-web-fr tiene salida a internet.
#
# Las pruebas corren en un contenedor busybox que comparte la red de agr-web-fr
# (--network container:agr-web-fr). No hace falta instalar nada en la imagen
# del front-end.
#
# Uso, desde la raíz del repositorio y con el sistema arriba:
#   docker compose up --build -d --wait
#   bash lab5-security/scripts/verify-b5-web-fr.sh [archivo-de-salida]
#
# Por defecto guarda una copia sin colores en
# lab5-security/evidence/after/B5_web-fr.txt.
# Código de salida: 0 si todo salió como se esperaba; 1 si hubo fallas;
# 2 si no se pudo ejecutar.
# =============================================================================

set -uo pipefail
# En Git Bash (Windows) evita que rutas como /dev/null se conviertan a rutas de Windows.
export MSYS_NO_PATHCONV=1

ORIGEN="agr-web-fr"
GATEWAY="agr-gateway-or"
GATEWAY_PUERTO=8080
RED_ESPERADA="agr-edge-net"
IMAGEN="${BUSYBOX_IMAGE:-busybox}"

# Destinos que agr-web-fr no debe alcanzar (design/flujos-permitidos.md, B5).
DESTINOS=(
  agr-auth-lo:8081 agr-voter-lo:8001 agr-election-lo:8000
  agr-vote-lo:8002 agr-results-lo:8004 agr-audit-lo:8005
  agr-auth-db:5432 agr-voters-db:5432 agr-elections-db:5432
  agr-votes-db:27017 agr-results-db:6379 agr-audit-db:27017
  agr-broker-as:5672
)

# ---------------------------------------------------------------- salida ----
DIR_LAB="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ $# -ge 1 ]]; then
  SALIDA="$1"
elif [[ -d "$DIR_LAB/evidence" ]]; then
  SALIDA="$DIR_LAB/evidence/after/B5_web-fr.txt"
else
  SALIDA="./B5_web-fr.txt"
fi
mkdir -p "$(dirname "$SALIDA")"
: > "$SALIDA"

if [[ -t 1 ]]; then
  VERDE=$'\e[32m'; ROJO=$'\e[31m'; AMARILLO=$'\e[33m'; GRIS=$'\e[90m'
  NEGRITA=$'\e[1m'; NORMAL=$'\e[0m'
else
  VERDE=""; ROJO=""; AMARILLO=""; GRIS=""; NEGRITA=""; NORMAL=""
fi

# Escribe en pantalla (con color) y en el archivo (sin color).
say() {
  printf '%s\n' "$*"
  printf '%s\n' "$*" | sed $'s/\e\\[[0-9;]*m//g' >> "$SALIDA"
}

TOTAL=0
FALLAS=0
AVISOS=0
ok()      { TOTAL=$((TOTAL + 1)); say "  ${VERDE}OK${NORMAL}       $*"; }
falla()   { TOTAL=$((TOTAL + 1)); FALLAS=$((FALLAS + 1)); say "  ${ROJO}FALLA${NORMAL}    $*"; }
atencion(){ AVISOS=$((AVISOS + 1)); say "  ${AMARILLO}ATENCIÓN${NORMAL} $*"; }
info()    { say "  ${GRIS}INFO${NORMAL}     $*"; }
titulo()  { say ""; say "${NEGRITA}$*${NORMAL}"; }

# ------------------------------------------------------------- sonda ----
SONDA="b5-sonda-$$"
limpiar() { docker rm -f "$SONDA" >/dev/null 2>&1 || true; }
trap limpiar EXIT

# resolver NOMBRE: pregunta al DNS de Docker desde agr-web-fr.
# Deja la primera IPv4 en RES_IP (vacía si no resuelve) y, si no resuelve,
# la causa en DNS_MSG (NXDOMAIN o sin respuesta).
RES_IP=""
DNS_MSG=""
resolver() {
  local salida
  salida=$(docker exec "$SONDA" nslookup -type=a "$1" 2>&1)
  RES_IP=$(printf '%s\n' "$salida" | awk '/^Name:/ {n = 1; next} n && /^Address/ {print $NF; exit}')
  if printf '%s\n' "$salida" | grep -q NXDOMAIN; then DNS_MSG="NXDOMAIN"; else DNS_MSG="sin respuesta"; fi
}

# tcp HOST PUERTO [SEGUNDOS]: 0 si la conexión se abre. Deja el mensaje en TCP_MSG.
TCP_MSG=""
tcp() {
  local salida rc
  salida=$(docker exec "$SONDA" nc -zv -w "${3:-2}" "$1" "$2" 2>&1)
  rc=$?
  TCP_MSG=$(printf '%s\n' "$salida" | tail -n 1)
  return "$rc"
}

# Resume el mensaje de nc: abierto, bad address, timeout, sin ruta o cerrado.
motivo() {
  case "$1" in
    *open*)                        echo "abierto" ;;
    *"bad address"*)               echo "bad address" ;;
    *"timed out"*)                 echo "timeout" ;;
    *"unreachable"*|*"No route"*)  echo "sin ruta" ;;
    *)                             echo "cerrado" ;;
  esac
}

# ---------------------------------------------------- precondiciones ----
if ! command -v docker >/dev/null 2>&1; then
  echo "No se encontró el comando docker." >&2; exit 2
fi
estado=$(docker inspect -f '{{.State.Status}}' "$ORIGEN" 2>/dev/null) || {
  echo "No existe el contenedor $ORIGEN. Levanten el sistema: docker compose up --build -d --wait" >&2
  exit 2
}
if [[ "$estado" != "running" ]]; then
  echo "$ORIGEN está en estado '$estado'. Revisen: docker compose logs $ORIGEN --tail 50" >&2
  exit 2
fi
if ! docker image inspect "$IMAGEN" >/dev/null 2>&1; then
  docker pull -q "$IMAGEN" >/dev/null || { echo "No se pudo descargar la imagen $IMAGEN." >&2; exit 2; }
fi
if ! docker run -d --rm --name "$SONDA" --network "container:$ORIGEN" "$IMAGEN" sleep 900 >/dev/null; then
  echo "No se pudo crear el contenedor auxiliar en la red de $ORIGEN." >&2; exit 2
fi

rama=$(git -C "$DIR_LAB" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "?")
commit=$(git -C "$DIR_LAB" rev-parse --short HEAD 2>/dev/null || echo "?")
version=$(docker version -f '{{.Server.Version}}' 2>/dev/null || echo "?")

say "${NEGRITA}B5 · ${ORIGEN} solo alcanza a ${GATEWAY}${NORMAL}"
say "$(date '+%Y-%m-%d %H:%M:%S %z') · rama ${rama} (${commit}) · Docker ${version}"

# ------------------------------------------------ 1. configuración ----
titulo "1. Configuración de ${ORIGEN}"

redes=$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}} {{end}}' "$ORIGEN" | xargs)
if [[ "$redes" == "$RED_ESPERADA" ]]; then
  ok "redes: ${redes}"
else
  falla "redes: ${redes:-ninguna} (se esperaba solo ${RED_ESPERADA})"
fi

puertos=$(docker port "$ORIGEN" 2>/dev/null | awk -F' -> ' '{print $2}' | xargs)
otros=$(docker port "$ORIGEN" 2>/dev/null | awk -F' -> ' '{n = split($2, a, ":"); if (a[n] != "3000") print $2}' | xargs)
if [[ -n "$puertos" && -z "$otros" ]]; then
  ok "puertos publicados: ${puertos}"
else
  falla "puertos publicados: ${puertos:-ninguno} (se esperaba solo el 3000)"
fi

for red in $redes; do
  interna=$(docker network inspect -f '{{.Internal}}' "$red" 2>/dev/null || echo "?")
  miembros=$(docker network inspect -f '{{range .Containers}}{{.Name}} {{end}}' "$red" 2>/dev/null | tr ' ' '\n' | sed '/^$/d' | sort | xargs)
  inesperados=""
  for m in $miembros; do
    case "$m" in
      "$ORIGEN"|"$GATEWAY"|*admin-fr*|"$SONDA") ;;
      *) inesperados="$inesperados $m" ;;
    esac
  done
  if [[ -z "$inesperados" ]]; then
    ok "${red}: internal=${interna}; miembros: ${miembros}"
  else
    falla "${red}: también están${inesperados}"
  fi
done
if [[ "$redes" == "$RED_ESPERADA" ]]; then
  info "${RED_ESPERADA} no puede ser internal: publica 3000 y 8080 en el host"
fi

entorno=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$ORIGEN")
secretos=$(printf '%s\n' "$entorno" | cut -d= -f1 | grep -E 'KEY|SECRET|PASS|TOKEN|JWT|CRED' | xargs)
if [[ -z "$secretos" ]]; then
  ok "sin secretos en el entorno (ninguna variable *KEY*, *SECRET*, *PASS*, *TOKEN*)"
else
  falla "variables con posibles secretos: ${secretos}"
fi
urls=$(printf '%s\n' "$entorno" | grep -E '^[A-Z_]*GATEWAY[A-Z_]*=' | xargs)
[[ -n "$urls" ]] && info "${urls}"

# ---------------------------------------------------- 2. permitido ----
titulo "2. Flujo permitido F3: ${ORIGEN} -> ${GATEWAY}:${GATEWAY_PUERTO}"

resolver "$GATEWAY"
if [[ -n "$RES_IP" ]]; then ok "DNS   ${GATEWAY} -> ${RES_IP}"; else falla "DNS   ${GATEWAY}: ${DNS_MSG}"; fi
if tcp "$GATEWAY" "$GATEWAY_PUERTO"; then
  ok "TCP   ${GATEWAY}:${GATEWAY_PUERTO} abierto"
else
  falla "TCP   ${GATEWAY}:${GATEWAY_PUERTO}: $(motivo "$TCP_MSG")"
fi
if cuerpo=$(docker exec "$SONDA" wget -q -T 5 -O - "http://${GATEWAY}:${GATEWAY_PUERTO}/health" 2>&1); then
  ok "HTTP  GET /health -> $(printf '%s' "$cuerpo" | tr -d '\r\n' | cut -c1-60)"
else
  falla "HTTP  GET /health: $(printf '%s' "$cuerpo" | tail -n 1)"
fi

# -------------------------------------------- 3. bloqueados por nombre ----
titulo "3. B5 por nombre: ningún *-lo, *-db ni broker debe resolver ni conectar"
say "$(printf '           %-18s %-7s %-16s %s' destino puerto 'DNS (nslookup)' 'TCP (nc)')"

for d in "${DESTINOS[@]}"; do
  nombre="${d%%:*}"; puerto="${d##*:}"
  if [[ "$(docker inspect -f '{{.State.Status}}' "$nombre" 2>/dev/null)" != "running" ]]; then
    falla "$(printf '%-18s %-7s %s' "$nombre" "$puerto" "no está corriendo: la prueba no vale")"
    continue
  fi
  resolver "$nombre"
  ip="$RES_IP"
  dns="${ip:-$DNS_MSG}"
  if tcp "$nombre" "$puerto"; then tcp_ok=0; else tcp_ok=1; fi
  linea=$(printf '%-18s %-7s %-16s %s' "$nombre" "$puerto" "$dns" "$(motivo "$TCP_MSG")")
  if [[ -z "$ip" && $tcp_ok -ne 0 ]]; then ok "$linea"; else falla "$linea"; fi
done

# ----------------------------------------------- 4. bloqueados por IP ----
titulo "4. B5 por IP: aun conociendo la IP del destino, no hay conexión"

for d in "${DESTINOS[@]}"; do
  nombre="${d%%:*}"; puerto="${d##*:}"
  ips=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}} {{end}}' "$nombre" 2>/dev/null | xargs)
  if [[ -z "$ips" ]]; then
    falla "$(printf '%-18s %s' "$nombre" "sin IP (¿está corriendo?)")"
    continue
  fi
  # Prueba todas las IP del destino en paralelo, con 1 s de espera cada una.
  # shellcheck disable=SC2086
  abiertas=$(docker exec "$SONDA" sh -c '
    p=$1; shift
    for ip in "$@"; do
      ( nc -z -w 1 "$ip" "$p" >/dev/null 2>&1 && echo "$ip" ) &
    done
    wait' sh "$puerto" $ips | xargs)
  destino_txt=$(for i in $ips; do printf '%s:%s ' "$i" "$puerto"; done | xargs)
  if [[ -z "$abiertas" ]]; then
    ok "$(printf '%-18s %-14s %s' "$nombre" "sin conexión" "$destino_txt")"
  else
    falla "$(printf '%-18s %-14s %s' "$nombre" "ABIERTO" "$abiertas (de: $destino_txt)")"
  fi
done

# ------------------------------------------ 5. puertos del host ----
titulo "5. Puertos publicados en el host: ninguno debe abrir un camino alterno"

extra=()       # "contenedor ip:puerto" que no deberían estar publicados
consola=()     # direcciones donde se publica la consola 15672 fuera de loopback
publicados=""
while read -r c; do
  while read -r linea; do
    [[ -z "$linea" ]] && continue
    host="${linea##* -> }"
    hport="${host##*:}"
    hip="${host%:*}"
    publicados="${publicados}${publicados:+; }${c} ${host}"
    case "$c:$hport" in
      "$ORIGEN:3000"|"$GATEWAY:$GATEWAY_PUERTO") ;;
      agr-broker-as:15672)
        [[ "$hip" == "127.0.0.1" || "$hip" == "[::1]" ]] || consola+=("$host") ;;
      *) extra+=("$c $host") ;;
    esac
  done < <(docker port "$c" 2>/dev/null)
done < <(docker ps --format '{{.Names}}' | grep '^agr-' | sort)

info "publicados: ${publicados:-ninguno}"
if [[ ${#extra[@]} -eq 0 && ${#consola[@]} -eq 0 ]]; then
  ok "solo se publican 3000 (web), 8080 (gateway) y 127.0.0.1:15672 (consola)"
fi
for h in ${consola[@]+"${consola[@]}"}; do
  falla "agr-broker-as publica la consola en ${h}; debe ser solo 127.0.0.1:15672"
done
for e in ${extra[@]+"${extra[@]}"}; do
  falla "puerto publicado de más: ${e} (revisen ports: y si hay un docker-compose.override.yml)"
done

host_gw=$(docker exec "$SONDA" ip route 2>/dev/null | awk '/^default/ {print $3; exit}')
if [[ -z "$host_gw" ]]; then
  info "agr-web-fr no tiene ruta por defecto; no se prueba el acceso por el host"
else
  if tcp "$host_gw" "$GATEWAY_PUERTO" 2; then
    info "${host_gw}:${GATEWAY_PUERTO} abierto: es la API pública del gateway, el mismo camino de F2"
  fi
  if tcp "$host_gw" 15672 2; then
    falla "${host_gw}:15672 abierto: la consola de RabbitMQ se alcanza a través del host"
  else
    ok "${host_gw}:15672 $(motivo "$TCP_MSG"): la consola de RabbitMQ no se alcanza por el host"
  fi
  for e in ${extra[@]+"${extra[@]}"}; do
    p="${e##*:}"
    if tcp "$host_gw" "$p" 2; then
      info "${host_gw}:${p} abierto: ${e%% *} se alcanza a través del host"
    fi
  done
fi

# Solo en Docker Desktop: host.docker.internal lleva a los puertos del equipo,
# incluidos los publicados en 127.0.0.1.
resolver host.docker.internal
if [[ -n "$RES_IP" ]]; then
  if tcp host.docker.internal 15672 2; then
    atencion "host.docker.internal:15672 abierto (Docker Desktop): la consola se alcanza desde los contenedores"
  else
    ok "host.docker.internal:15672 $(motivo "$TCP_MSG")"
  fi
fi

# ------------------------------------------------ 6. internet ----
titulo "6. Salida a internet (informativo; B4 solo exige el bloqueo a los *-lo)"

if docker exec "$SONDA" wget -q -T 4 -O /dev/null http://example.com >/dev/null 2>&1; then
  atencion "${ORIGEN} sí sale a internet: ${RED_ESPERADA} no es internal (riesgo residual, ver §3.6)"
else
  info "${ORIGEN} no llegó a http://example.com"
fi

# ---------------------------------------------------- resumen ----
say ""
if [[ $FALLAS -eq 0 ]]; then
  say "${NEGRITA}${VERDE}Resultado: ${TOTAL}/${TOTAL} comprobaciones como se esperaba. B5 se cumple.${NORMAL}"
else
  say "${NEGRITA}${ROJO}Resultado: ${FALLAS} de ${TOTAL} comprobaciones fallaron.${NORMAL}"
fi
[[ $AVISOS -gt 0 ]] && say "Avisos para documentar: ${AVISOS}."
say "Copia en: ${SALIDA}"

[[ $FALLAS -eq 0 ]]
