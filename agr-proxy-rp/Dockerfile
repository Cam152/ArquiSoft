# agr-proxy-rp: Nginx como reverse proxy de AGORA (S2 · Reverse Proxy).
# Se usa el Nginx de Alpine porque trae como paquete el módulo headers-more, que
# es lo que permite quitar el encabezado Server (server_tokens solo quita la versión).
FROM alpine:3.22

RUN apk add --no-cache nginx nginx-mod-http-headers-more gettext-envsubst \
    && rm -f /etc/nginx/http.d/default.conf

# Límite de intentos de login (S4 · Esteban). R2 puede subirlo con estas variables.
ENV LOGIN_RATE=5r/m \
    LOGIN_BURST=5

COPY nginx.conf /etc/nginx/nginx.conf.template

EXPOSE 80

# Al arrancar: reemplaza solo LOGIN_RATE y LOGIN_BURST, valida la configuración y
# deja Nginx en primer plano (así Docker ve sus logs y su estado).
CMD ["/bin/sh", "-c", "envsubst '${LOGIN_RATE} ${LOGIN_BURST}' < /etc/nginx/nginx.conf.template > /etc/nginx/nginx.conf && nginx -t -q && exec nginx -g 'daemon off;'"]
