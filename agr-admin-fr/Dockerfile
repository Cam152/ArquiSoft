# agr-admin-fr: CLI "agora". Construcción en dos etapas; la imagen final solo
# lleva el binario. No es un servicio: se ejecuta bajo demanda con
#   docker compose run --rm agr-admin-fr <comando>
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath \
      -ldflags="-s -w -X agora-admin/internal/commands.Version=${VERSION}" \
      -o /agora ./cmd/agora

FROM alpine:3.20
RUN adduser -D -H agora
COPY --from=build /agora /usr/local/bin/agora
USER agora
ENTRYPOINT ["agora"]
CMD ["--help"]
