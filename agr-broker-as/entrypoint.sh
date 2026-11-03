#!/bin/sh
# Genera /etc/rabbitmq/definitions.json con el usuario y la clave de las
# variables BROKER_USER y BROKER_PASSWORD, y arranca RabbitMQ con el entrypoint
# de la imagen oficial.
#
# Hace falta porque, cuando hay definiciones que cargar al arrancar, RabbitMQ no
# crea el usuario por defecto (RABBITMQ_DEFAULT_USER se ignora): el usuario tiene
# que venir en las definiciones.
set -eu

: "${BROKER_USER:?Falta BROKER_USER}"
: "${BROKER_PASSWORD:?Falta BROKER_PASSWORD}"

# Escapa un valor en dos pasos: primero para un string JSON (\ y "), luego para
# el reemplazo de sed (\, & y el separador |).
escape() {
  printf '%s' "$1" | sed -e 's/[\\"]/\\&/g' | sed -e 's/[\\&|]/\\&/g'
}

user=$(escape "$BROKER_USER")
password=$(escape "$BROKER_PASSWORD")

sed -e "s|__BROKER_USER__|$user|g" -e "s|__BROKER_PASSWORD__|$password|g" \
  /etc/rabbitmq/definitions.template.json > /etc/rabbitmq/definitions.json
chown rabbitmq:rabbitmq /etc/rabbitmq/definitions.json
chmod 600 /etc/rabbitmq/definitions.json

exec docker-entrypoint.sh "$@"
