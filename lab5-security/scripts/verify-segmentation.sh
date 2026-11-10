#!/usr/bin/env bash
# verify-segmentation.sh — Matriz de alcanzabilidad origen x destino (Lab 5, equipo 1b)
#
# Para cada contenedor agr-* en ejecución (origen) intenta abrir una conexión TCP
# hacia el puerto de servicio de cada otro contenedor (destino), desde el propio
# espacio de red del origen (busybox con --network container:<origen>):
#   1) por nombre (DNS interno de Docker);
#   2) si el nombre no abre, por cada IP real del destino (para no confundir
#      "no resuelve" con "no hay ruta").
# También prueba la salida a internet de cada origen.
#
# Cada camino abierto se clasifica contra design/flujos-permitidos.md:
#   F   flujo permitido (F3-F19)
#   R   sentido inverso de un flujo permitido (mismo segmento; un bridge no es direccional)
#   B8  servicio <-> servicio dentro de agr-back-net (riesgo aceptado, §3.6)
#   X   camino NO autorizado  <- la medida del escenario es cuántos hay
#
# Uso:  ./verify-segmentation.sh [archivo-salida]
#   Antes:   en la rama main            -> evidence/before/matriz.txt
#   Después: en la rama lab5-security   -> evidence/after/matriz.txt
set -uo pipefail

OUT="${1:-}"
IMG="busybox:1.36"
docker image inspect "$IMG" >/dev/null 2>&1 || docker pull -q "$IMG" >/dev/null

# destino -> puerto de servicio
declare -A PORT=(
  [agr-web-fr]=3000 [agr-gateway-or]=8080
  [agr-auth-lo]=8081 [agr-voter-lo]=8001 [agr-election-lo]=8000
  [agr-vote-lo]=8002 [agr-results-lo]=8004 [agr-audit-lo]=8005
  [agr-auth-db]=5432 [agr-voters-db]=5432 [agr-elections-db]=5432
  [agr-votes-db]=27017 [agr-results-db]=6379 [agr-audit-db]=27017
  [agr-broker-as]=5672
)
ORDER=(agr-web-fr agr-gateway-or agr-auth-lo agr-voter-lo agr-election-lo agr-vote-lo
       agr-results-lo agr-audit-lo agr-auth-db agr-voters-db agr-elections-db
       agr-votes-db agr-results-db agr-audit-db agr-broker-as)
SHORT=(web gw auth voter elec vote res audit authDB votrsDB elecDB votesDB resDB audDB broker)

# Flujos permitidos (origen>destino) de design/flujos-permitidos.md §3
declare -A FLOW=(
  [agr-web-fr\>agr-gateway-or]=F3
  [agr-gateway-or\>agr-auth-lo]=F5 [agr-gateway-or\>agr-voter-lo]=F6
  [agr-gateway-or\>agr-election-lo]=F7 [agr-gateway-or\>agr-vote-lo]=F8
  [agr-gateway-or\>agr-results-lo]=F9 [agr-gateway-or\>agr-audit-lo]=F10
  [agr-auth-lo\>agr-auth-db]=F11 [agr-election-lo\>agr-elections-db]=F12
  [agr-voter-lo\>agr-voters-db]=F13 [agr-vote-lo\>agr-votes-db]=F14
  [agr-results-lo\>agr-results-db]=F15 [agr-audit-lo\>agr-audit-db]=F16
  [agr-vote-lo\>agr-broker-as]=F17 [agr-results-lo\>agr-broker-as]=F18
  [agr-audit-lo\>agr-broker-as]=F19
)
LOGIC=" agr-auth-lo agr-voter-lo agr-election-lo agr-vote-lo agr-results-lo agr-audit-lo agr-gateway-or "

classify() { # $1 origen  $2 destino
  local k="$1>$2" r="$2>$1"
  if [[ -n "${FLOW[$k]:-}" ]]; then echo "${FLOW[$k]}"
  elif [[ -n "${FLOW[$r]:-}" ]]; then echo R
  elif [[ "$LOGIC" == *" $1 "* && "$LOGIC" == *" $2 "* ]]; then echo B8
  else echo X; fi
}

running=$(docker ps --format '{{.Names}}' | grep '^agr-' | sort)
present=(); for c in "${ORDER[@]}"; do grep -qx "$c" <<<"$running" && present+=("$c"); done

# lista "nombre puerto ip1,ip2" de los destinos
targets=""
for d in "${present[@]}"; do
  ips=$(docker inspect "$d" --format '{{range .NetworkSettings.Networks}}{{.IPAddress}},{{end}}')
  targets+="$d ${PORT[$d]} ${ips%,}"$'\n'
done

TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT
REP="$TMP/report.txt"

probe() { # $1 origen -> $TMP/<origen>
  local src="$1"
  docker run --rm --network "container:$src" -e SRC="$src" -e TARGETS="$targets" "$IMG" sh -c '
    echo "$TARGETS" | while read -r name port ips; do
      [ -z "$name" ] && continue
      [ "$name" = "$SRC" ] && { echo "$name SELF"; continue; }
      out=$(nc -zv -w 2 "$name" "$port" 2>&1)
      case "$out" in
        *open*) echo "$name NAME"; continue;;
        *"bad address"*) dns=0;; *) dns=1;;
      esac
      res="CLOSED"
      for ip in $(echo "$ips" | tr , " "); do
        if nc -z -w 1 "$ip" "$port" 2>/dev/null; then res="IP"; break; fi
      done
      [ "$res" = CLOSED ] && [ $dns = 1 ] && res="DNSONLY"
      echo "$name $res"
    done
    if wget -q -T 4 -O /dev/null http://example.com 2>/dev/null; then echo "@internet YES"; else echo "@internet NO"; fi
  ' >"$TMP/$src" 2>/dev/null
}

for s in "${present[@]}"; do probe "$s" & done; wait

{
  echo "Matriz de alcanzabilidad · AGORA"
  echo "$(date '+%Y-%m-%d %H:%M:%S %z') · rama $(git rev-parse --abbrev-ref HEAD 2>/dev/null) ($(git rev-parse --short HEAD 2>/dev/null)) · Docker $(docker version --format '{{.Server.Version}}')"
  echo
  echo "Redes de cada contenedor:"
  for c in "${present[@]}"; do
    printf '  %-17s %s\n' "$c" "$(docker inspect "$c" --format '{{range $k,$v := .NetworkSettings.Networks}}{{$k}} {{end}}')"
  done
  echo
  echo "Leyenda de celda: F#=flujo permitido abierto · R=inverso de flujo permitido · B8=servicio<->servicio (riesgo aceptado)"
  echo "                  X=camino NO autorizado abierto (por nombre) · X*=abierto solo por IP · .=bloqueado · -=mismo contenedor"
  echo
  printf '%-18s' "origen \\ destino"
  for i in "${!ORDER[@]}"; do grep -qx "${ORDER[$i]}" <<<"$running" && printf '%-8s' "${SHORT[$i]}"; done
  printf '%-8s\n' "internet"
} >"$REP"

declare -i nF=0 nR=0 nB8=0 nX=0 nNet=0 nBlocked=0
xlist=""
for s in "${present[@]}"; do
  line=$(printf '%-18s' "$s")
  for d in "${present[@]}"; do
    r=$(awk -v d="$d" '$1==d{print $2}' "$TMP/$s")
    case "$r" in
      SELF) cell="-";;
      NAME|IP)
        c=$(classify "$s" "$d"); cell="$c"
        case "$c" in F*) nF+=1;; R) nR+=1;; B8) nB8+=1;; X) nX+=1; xlist+="  $s -> $d:${PORT[$d]} ($r)"$'\n'; [[ $r == IP ]] && cell="X*";; esac;;
      *) cell="."; nBlocked+=1;;
    esac
    line+=$(printf '%-8s' "$cell")
  done
  inet=$(awk '$1=="@internet"{print $2}' "$TMP/$s")
  if [[ "$inet" == YES ]]; then line+="SÍ"; nNet+=1; else line+="no"; fi
  echo "$line" >>"$REP"
done

{
  echo
  echo "Resumen"
  echo "  Flujos permitidos abiertos (F):            $nF"
  echo "  Inversos de flujos permitidos (R):         $nR"
  echo "  Servicio<->servicio en back-net (B8):      $nB8"
  echo "  Caminos NO autorizados abiertos (X):       $nX"
  echo "  Pares bloqueados:                          $nBlocked"
  echo "  Contenedores con salida a internet:        $nNet de ${#present[@]}"
  if [[ -n "$xlist" ]]; then echo; echo "Caminos no autorizados:"; printf '%s' "$xlist"; fi
} >>"$REP"

cat "$REP"; [[ -n "$OUT" ]] && cp "$REP" "$OUT"
exit 0
