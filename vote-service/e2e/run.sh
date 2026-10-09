#!/bin/sh
# Levanta el sistema en un proyecto de Compose aparte (no toca los contenedores ni
# los datos del sistema normal), corre las pruebas de punta a punta y lo borra.
set -u
cd "$(dirname "$0")/../.."

compose() {
  docker compose -p arquisoft-e2e -f docker-compose.yml -f docker-compose.test.yml "$@"
}

compose run --rm --build e2e
status=$?

if [ "$status" -ne 0 ]; then
  echo "--- Las pruebas fallaron. Logs de los servicios:"
  compose logs --no-color --tail 50 vote-service election-service mock-voter-service
fi
compose down --volumes --remove-orphans
exit "$status"
