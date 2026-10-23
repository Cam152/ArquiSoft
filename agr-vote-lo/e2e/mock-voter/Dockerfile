# Se construye con contexto en vote-service/ (comparte su go.mod).
FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY e2e/mock-voter ./e2e/mock-voter
RUN CGO_ENABLED=0 go build -o /mock-voter ./e2e/mock-voter

FROM alpine:3.20
COPY --from=build /mock-voter /mock-voter
EXPOSE 8001
CMD ["/mock-voter"]
