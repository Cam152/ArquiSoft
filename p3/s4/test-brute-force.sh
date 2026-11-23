#!/usr/bin/env bash
# p3/s4/test-brute-force.sh · S4 · Protección del login contra fuerza bruta.
#
# Prueba las dos defensas de S4 entrando por el proxy, como un atacante real:
#   A. Bloqueo de cuenta (agr-auth-lo): 5 claves erradas para el documento 1001 -> 423,
#      y la clave correcta también recibe 423 mientras dura el bloqueo (15 min).
#   B. Límite por IP (agr-proxy-rp): 5 intentos por minuto con ráfaga de 5. El 7.º
#      intento seguido desde la misma IP recibe 429 del proxy aunque cambie de documento
#      (password spraying) y no llega al gateway ni a agr-auth-lo.
# Al final muestra el estado guardado en agr-auth-db y desbloquea la cuenta 1001.
#
# Uso (Git Bash, desde la raíz del repo, con AGORA levantado):
#   bash p3/s4/test-brute-force.sh | tee p3/s4/evidencia_s4.txt
set -uo pipefail
export MSYS_NO_PATHCONV=1

BASE_URL="${BASE_URL:-https://localhost/api}"
LOGIN="${BASE_URL}/auth/login"
BODY=$(mktemp); trap 'rm -f "$BODY"' EXIT
fails=0

login() {  # login <documento> <clave> -> imprime el código HTTP; el cuerpo queda en $BODY
  curl -ksS -o "$BODY" -w "%{http_code}" -X POST "$LOGIN" \
       -H "Content-Type: application/json" -d "{\"document\":\"$1\",\"password\":\"$2\"}"
}

check() {  # check <n.º> <descripción> <esperado> <obtenido>
  local mark="OK"; [[ "$3" == "$4" ]] || { mark="FALLA"; fails=$((fails + 1)); }
  printf "%-3s %-58s esperado %s · obtenido %s  %s\n" "$1" "$2" "$3" "$4" "$mark"
  printf "    %s\n" "$(tr -d '\n' < "$BODY" | cut -c1-140)"
}

echo "AGORA · Prototipo 3 · S4 · Protección del login contra fuerza bruta"
echo "Fecha: $(date '+%Y-%m-%d %H:%M:%S %z') · Entrada: $LOGIN"
echo "=============================================================================="
echo
echo "A. Bloqueo de cuenta en agr-auth-lo (documento 1001)"
for i in 1 2 3 4; do
  s=$(login 1001 clave-incorrecta); check "$i" "Clave incorrecta, intento $i" 401 "$s"
done
s=$(login 1001 clave-incorrecta); check 5 "Clave incorrecta, intento 5: la cuenta se bloquea" 423 "$s"
s=$(login 1001 123456);           check 6 "Clave CORRECTA durante el bloqueo" 423 "$s"

echo
echo "B. Límite por IP en agr-proxy-rp (5/min, ráfaga de 5)"
s=$(login 1002 clave-incorrecta); check 7 "7.º intento seguido, otro documento (1002)" 429 "$s"
s=$(login 1003 clave-incorrecta); check 8 "8.º intento seguido, otro documento (1003)" 429 "$s"

echo
echo "C. Estado guardado en agr-auth-db"
docker compose exec -T agr-auth-db psql -U postgres -d agr_auth_db \
  -c "SELECT document, failed_attempts, locked_until FROM voters WHERE document IN ('1001','1002','1003') ORDER BY document;"

echo "D. Peticiones rechazadas por el proxy (registro de agr-proxy-rp, último minuto)"
docker compose logs --since 1m agr-proxy-rp 2>/dev/null | grep '/api/auth/login' | grep -c '" 429 ' |
  sed 's/^/    respuestas 429 en \/api\/auth\/login: /'

echo
echo "Restaurar: se desbloquea la cuenta 1001 para seguir usándola"
docker compose exec -T agr-auth-db psql -U postgres -d agr_auth_db -q \
  -c "UPDATE voters SET failed_attempts = 0, locked_until = NULL WHERE document IN ('1001','1002','1003');"

echo "=============================================================================="
if [[ $fails -eq 0 ]]; then echo "S4: todas las pruebas pasan"; else echo "S4: $fails prueba(s) fallaron"; fi
echo "(Si B falla con 401, ya pasó más de un minuto desde A o hubo pocos intentos; vuelva a correr el script.)"
exit "$fails"
