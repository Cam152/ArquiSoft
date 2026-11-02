# Construcción en dos etapas: la imagen final solo lleva el binario.
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /vote-service .

FROM alpine:3.20
RUN adduser -D -H appuser
COPY --from=build /vote-service /usr/local/bin/vote-service
USER appuser
EXPOSE 8002
HEALTHCHECK --interval=10s --timeout=5s --start-period=15s --retries=6 \
  CMD wget -qO- http://localhost:8002/health || exit 1
ENTRYPOINT ["/usr/local/bin/vote-service"]
