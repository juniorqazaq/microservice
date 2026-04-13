# Assignment 2: gRPC Migration

This project migrates internal communication between `order-service` and `payment-service` from REST to gRPC.

## Services

- `order-service`
  - REST API for end users
  - gRPC client for `payment-service`
  - gRPC server for order status streaming
- `payment-service`
  - gRPC server for payment processing

## Architecture Diagram

```mermaid
flowchart LR
    Client["Client"] -->|HTTP REST| OrderHTTP["Order Service HTTP API"]
    OrderHTTP --> OrderUC["Order Use Case"]
    OrderUC --> OrderRepo["Order Repository"]
    OrderRepo --> OrderDB[("order_db")]
    OrderUC -->|gRPC ProcessPayment| PaymentGRPC["Payment Service gRPC Server"]
    PaymentGRPC --> PaymentUC["Payment Use Case"]
    PaymentUC --> PaymentRepo["Payment Repository"]
    PaymentRepo --> PaymentDB[("payment_db")]
    StreamClient["Stream Client"] -->|gRPC SubscribeToOrderUpdates| OrderGRPC["Order Service gRPC Server"]
    OrderGRPC --> OrderUC
```

## Contracts

- Proto source: `proto-repository/`
- Generated Go contracts: `generated-contracts/`

Replace `github.com/youruser/ap2-generated-contracts` in:

- `proto-repository/proto/order/v1/order.proto`
- `proto-repository/proto/payment/v1/payment.proto`
- `generated-contracts/go.mod`
- `order-service/go.mod`
- `payment-service/go.mod`

with your real generated-contract repository path before submission.

## Environment

Use `.env.example`:

```env
ORDER_DATABASE_URL=postgres://postgres:postgres@localhost:5433/order_db?sslmode=disable
PAYMENT_DATABASE_URL=postgres://postgres:postgres@localhost:5433/payment_db?sslmode=disable
ORDER_HTTP_ADDR=:8080
ORDER_GRPC_ADDR=:9090
ORDER_GRPC_TARGET=localhost:9090
PAYMENT_GRPC_ADDR=:9091
PAYMENT_GRPC_TARGET=localhost:9091
```

## Run

1. Start PostgreSQL with `docker compose up -d`.
2. Apply migrations:

```bash
psql "$ORDER_DATABASE_URL" -f order-service/migrations/001_init.sql
psql "$PAYMENT_DATABASE_URL" -f payment-service/migrations/001_init.sql
```

3. Start Payment Service:

```bash
cd payment-service
go run ./cmd/payment-service
```

4. Start Order Service:

```bash
cd order-service
go run ./cmd/order-service
```

5. Optional stream client:

```bash
cd order-service
ORDER_ID=<order-id> ORDER_GRPC_TARGET=localhost:9090 go run ./cmd/order-stream-client
```

## Notes

- Payment processing now uses gRPC `ProcessPayment`.
- Order tracking uses server-side streaming `SubscribeToOrderUpdates`.
- Payment service has a unary interceptor for request logging.
