FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /vote-service .

FROM alpine:3.20
RUN adduser -D appuser
USER appuser
COPY --from=build /vote-service /vote-service
EXPOSE 8002
CMD ["/vote-service"]
