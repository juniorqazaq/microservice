# Assignment 4: Performance Optimization & External Integrations

This project extends the Order, Payment, and Notification services with Redis caching, Redis-backed idempotency, provider adapters, and retry logic for background notifications.

## Services

- `order-service`
  - REST API for creating and reading orders
  - Redis cache-aside for `GET /orders/:id`
  - Redis rate limiter for HTTP requests
  - gRPC client for `payment-service`
  - gRPC server for order status streaming
- `payment-service`
  - gRPC server for payment processing
  - Publishes `payment.completed` events to RabbitMQ after a successful DB write
- `notification-service`
  - RabbitMQ consumer for `payment_completed_queue`
  - Sends email through a provider adapter
  - Supports simulated and SMTP providers
  - Uses Redis for notification idempotency
- `postgres`
  - Stores order and payment data
- `rabbitmq`
  - Message broker with management UI on `http://localhost:15672`
- `redis`
  - Shared cache, idempotency store, and rate limiter state

## Architecture Diagram

```mermaid
flowchart LR
    Client["Client"] -->|HTTP POST /orders| OrderHTTP["Order Service"]
    Client -->|HTTP GET /orders/:id| OrderHTTP
    OrderHTTP -->|cache get/set/delete| Redis[("Redis")]
    OrderHTTP --> OrderDB[("order_db")]
    OrderHTTP -->|gRPC ProcessPayment| PaymentGRPC["Payment Service"]
    PaymentGRPC --> PaymentDB[("payment_db")]
    PaymentGRPC -->|publish payment.completed| Exchange["RabbitMQ exchange: payment_events"]
    Exchange --> Queue["Durable queue: payment_completed_queue"]
    Queue -->|manual ACK| Notification["Notification Service"]
    Notification -->|idempotency check| Redis
    Notification --> Provider["EmailSender provider"]
    Provider --> Simulated["Simulated provider"]
    Provider --> SMTP["SMTP provider"]
    Queue --> DLX["Dead letter exchange"]
    DLX --> DLQ["payment_events_dlq"]
```

## Cache Strategy

The Order Service uses the cache-aside pattern. `GetOrderByID` checks Redis first using the key `order:<id>`. On a miss, it loads the order from Postgres and stores the JSON value in Redis with `CACHE_TTL_SECONDS` TTL.

Cache invalidation happens immediately after order status changes. When payment processing updates an order to `Paid` or `Failed`, and when `CancelOrder` updates an order to `Cancelled`, the service deletes `order:<id>` from Redis so stale statuses are not served.

The optional bonus rate limiter also uses Redis. It increments `rate:<client-ip>` for each request and expires the key after one minute.

## Background Jobs and Retry

- RabbitMQ queue and exchanges are durable.
- Published messages use `DeliveryMode: Persistent`.
- The payment producer uses RabbitMQ publisher confirms, so publish only succeeds after broker confirmation.
- The notification consumer disables auto-ACK and calls `Ack(false)` only after the provider sends successfully and Redis is marked as done.
- Provider failures are retried with exponential backoff: `2s`, `4s`, then `8s`.
- If all retry attempts fail, the message is rejected with `Nack(false, false)` and routed to the DLQ.

## Idempotency

The producer sets the RabbitMQ `MessageId` to the payment ID and includes the same value as `event_id` in the JSON body. The notification service uses Redis key `notif:<payment-id>` with a 24-hour TTL.

Before sending an email, the worker calls `SetNX` with status `processing`. If the key already exists, the event is treated as a duplicate and ACKed. After a successful send, the worker stores `done`. If sending fails after all retries, the worker deletes the key before NACKing so a future redelivery can be processed cleanly.

## Provider Adapter

Notification delivery depends on the `EmailSender` interface. `PROVIDER_MODE=SIMULATED` uses a simulated provider that waits 200ms and randomly fails around 30% of the time. `PROVIDER_MODE=REAL` uses the SMTP provider configured by environment variables.

## Environment

Order Service:

```text
ORDER_DATABASE_URL=postgres://postgres:postgres@postgres:5432/order_db?sslmode=disable
ORDER_HTTP_ADDR=:8080
ORDER_GRPC_ADDR=:9090
PAYMENT_GRPC_TARGET=payment-service:9091
REDIS_ADDR=redis:6379
CACHE_TTL_SECONDS=300
```

Notification Service:

```text
RABBITMQ_URL=amqp://guest:guest@rabbitmq:5672/
REDIS_ADDR=redis:6379
PROVIDER_MODE=SIMULATED
SMTP_HOST=
SMTP_PORT=
SMTP_USER=
SMTP_PASSWORD=
SMTP_FROM=
```

## Run

```bash
docker compose up --build
```

RabbitMQ UI:

```text
http://localhost:15672
username: guest
password: guest
```

Create an order:

```bash
curl -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-1' \
  -d '{
    "customer_id": "customer-1",
    "customer_email": "user@example.com",
    "item_name": "Keyboard",
    "amount": 9999
  }'
```

Expected notification log:

```text
[Notification] Simulated email sent to user@example.com with subject "Payment completed for order <order-id>"
```
