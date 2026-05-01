FROM golang:1.24-alpine AS builder

ENV GOTOOLCHAIN=local

WORKDIR /workspace

COPY go.work go.work.sum ./
COPY generated-contracts/ ./generated-contracts/
COPY proto-repository/ ./proto-repository/
COPY order-service/ ./order-service/
COPY payment-service/ ./payment-service/
COPY notification-service/ ./notification-service/

WORKDIR /workspace/notification-service
RUN go mod download
RUN go build -o /app/notification-service ./cmd/notification-service

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/notification-service /app/notification-service
CMD ["/app/notification-service"]
