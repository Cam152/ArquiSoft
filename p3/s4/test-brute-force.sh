#!/usr/bin/env bash

set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
LOGIN_ENDPOINT="${BASE_URL}/auth/login"

DOCUMENT="1001"
WRONG_PASSWORD="incorrect-password"

BODY_FILE="$(mktemp)"
trap 'rm -f "$BODY_FILE"' EXIT

echo "========================================"
echo " AGORA - PRUEBA S4: BRUTE FORCE"
echo "========================================"

for attempt in 1 2 3 4 5; do

    echo ""
    echo "Intento ${attempt}/5"

    STATUS=$(curl -sS \
        -o "$BODY_FILE" \
        -w "%{http_code}" \
        -X POST "$LOGIN_ENDPOINT" \
        -H "Content-Type: application/json" \
        -d "{\"document\":\"${DOCUMENT}\",\"password\":\"${WRONG_PASSWORD}\"}")

    echo "HTTP: ${STATUS}"
    cat "$BODY_FILE"
    echo ""

    if [[ "$attempt" -lt 5 && "$STATUS" != "401" ]]; then
        echo "ERROR: Se esperaba HTTP 401."
        exit 1
    fi

    if [[ "$attempt" -eq 5 && "$STATUS" != "423" ]]; then
        echo "ERROR: Se esperaba HTTP 423."
        exit 1
    fi

done

echo ""
echo "========================================"
echo " VERIFICANDO BLOQUEO"
echo "========================================"

STATUS=$(curl -sS \
    -o "$BODY_FILE" \
    -w "%{http_code}" \
    -X POST "$LOGIN_ENDPOINT" \
    -H "Content-Type: application/json" \
    -d "{\"document\":\"${DOCUMENT}\",\"password\":\"123456\"}")

echo "Intento con contraseña correcta."
echo "HTTP: ${STATUS}"
cat "$BODY_FILE"
echo ""

if [[ "$STATUS" != "423" ]]; then
    echo "ERROR: La cuenta deberia continuar bloqueada."
    exit 1
fi

echo ""
echo "========================================"
echo " S4: PRUEBA COMPLETADA CORRECTAMENTE"
echo "========================================"
