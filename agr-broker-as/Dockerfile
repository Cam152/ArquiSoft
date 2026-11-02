# agr-broker-as: RabbitMQ con la topología de AGORA ya cargada.
FROM rabbitmq:3.13-management-alpine

COPY rabbitmq.conf /etc/rabbitmq/conf.d/20-agora.conf
COPY definitions.template.json /etc/rabbitmq/definitions.template.json
COPY --chmod=755 entrypoint.sh /usr/local/bin/agora-entrypoint.sh

# 5672: AMQP (solo dentro de Docker). 15672: consola web.
EXPOSE 5672 15672

HEALTHCHECK --interval=10s --timeout=10s --start-period=30s --retries=6 \
  CMD rabbitmq-diagnostics -q check_port_connectivity || exit 1

ENTRYPOINT ["agora-entrypoint.sh"]
CMD ["rabbitmq-server"]
