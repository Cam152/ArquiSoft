# Etapa 1: compila el binario (musl, sin dependencias de sistema en la imagen final).
FROM rust:1-alpine AS build
RUN apk add --no-cache musl-dev
WORKDIR /src
# Primero solo los manifiestos: las dependencias quedan en caché si el código cambia.
COPY Cargo.toml Cargo.lock ./
RUN mkdir src \
    && echo "fn main() {}" > src/main.rs \
    && touch src/lib.rs \
    && cargo build --release --locked \
    && rm -rf src target/release/results-service* target/release/deps/results_service* target/release/.fingerprint/results-service*
COPY src ./src
RUN cargo build --release --locked

# Etapa 2: imagen final, sin compiladores.
FROM alpine:3.20
RUN adduser -D appuser
USER appuser
COPY --from=build /src/target/release/results-service /results-service
EXPOSE 8004
CMD ["/results-service"]
