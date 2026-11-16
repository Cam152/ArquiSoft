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
    && cargo clean --release --locked -p results-service \
    && rm -rf src
# "cargo clean -p" no depende del layout interno de target/ (cambió en Rust 1.100 y
# el rm por rutas dejaba el binario vacío del paso anterior). El touch evita que cargo
# tome el código copiado como viejo (COPY conserva el mtime original).
COPY src ./src
RUN touch src/main.rs src/lib.rs && cargo build --release --locked

# Etapa 2: imagen final, sin compiladores.
FROM alpine:3.20
RUN adduser -D appuser
USER appuser
COPY --from=build /src/target/release/results-service /results-service
EXPOSE 8004
CMD ["/results-service"]
