# Assignment 1: Clean Architecture Microservices in Go

This repository implements the required two-service platform:

- `order-service` on port `8080`
- `payment-service` on port `8081`

Both services use Gin for REST, PostgreSQL for persistence, and Clean Architecture layers with manual dependency injection in `main.go`.

## What Is Already Covered

- Separate bounded contexts for Orders and Payments
- Separate databases: `order_db` and `payment_db`
- No shared entity/model package between services
- Thin HTTP handlers
- Business rules in use cases
- PostgreSQL repositories
- REST-only communication from Order Service to Payment Service
- Shared `http.Client` instance with `2s` timeout in the Order Service payment adapter
- Failure handling with `503 Service Unavailable` when Payment Service is unreachable
- Idempotency support via `Idempotency-Key` header for `POST /orders`

## Project Structure

```text
order-service/
payment-service/
docker-compose.yml
infra/postgres/init/
```

Each service follows:

```text
service/
├── cmd/service-name/main.go
├── internal/domain
├── internal/usecase
├── internal/repository
├── internal/transport/http
└── migrations
```

## Architecture Decisions

- `order-service` owns order creation, cancellation, and state transitions.
- `payment-service` owns payment authorization and payment limits.
- `order-service` never reads or writes the payment database directly.
- Money is stored as `int64` cents everywhere.
- If payment authorization fails because the service is unavailable, the order is marked as `Failed`.

That last choice is intentional for defense: it gives a clear final state without background retries and satisfies the required `503` behavior.

## Architecture Diagram

```mermaid
flowchart LR
    Client["Client"] -->|POST /orders| OrderHandler["Order HTTP Handler"]
    Client -->|GET /orders/:id| OrderHandler
    Client -->|PATCH /orders/:id/cancel| OrderHandler
    OrderHandler --> OrderUseCase["Order Use Case"]
    OrderUseCase --> OrderRepo["Order Repository"]
    OrderRepo --> OrderDB[("order_db")]
    OrderUseCase -->|POST /payments via REST, timeout 2s| PaymentHandler["Payment HTTP Handler"]
    PaymentHandler --> PaymentUseCase["Payment Use Case"]
    PaymentUseCase --> PaymentRepo["Payment Repository"]
    PaymentRepo --> PaymentDB[("payment_db")]
```

## PostgreSQL Setup

### Option 1: Docker Compose

```bash
docker compose up -d postgres
make migrate-order
make migrate-payment
```

`docker-compose.yml` starts one PostgreSQL server and creates two isolated databases:

- `order_db`
- `payment_db`

### Option 2: Manual Postgres

```sql
CREATE DATABASE order_db;
CREATE DATABASE payment_db;
```

Then run:

```bash
psql "postgres://postgres:postgres@localhost:5433/order_db?sslmode=disable" -f order-service/migrations/001_init.sql
psql "postgres://postgres:postgres@localhost:5433/payment_db?sslmode=disable" -f payment-service/migrations/001_init.sql
```

## Running the Services

Terminal 1:

```bash
make run-payment
```

Terminal 2:

```bash
make run-order
```

Default connection values come from [`.env.example`](/Users/admin/Assignment1GO_Sanat/.env.example).

## API Examples

Create order:

```bash
curl -X POST http://localhost:8080/orders \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: order-001" \
  -d '{"customer_id":"cust-1","item_name":"Keyboard","amount":15000}'
```

Get order:

```bash
curl http://localhost:8080/orders/<order_id>
```

Cancel order:

```bash
curl -X PATCH http://localhost:8080/orders/<order_id>/cancel
```

Authorize payment directly:

```bash
curl -X POST http://localhost:8081/payments \
  -H "Content-Type: application/json" \
  -d '{"order_id":"order-123","amount":15000}'
```

Get payment by order id:

```bash
curl http://localhost:8081/payments/order-123
```

Declined payment example:

```bash
curl -X POST http://localhost:8081/payments \
  -H "Content-Type: application/json" \
  -d '{"order_id":"order-999","amount":150001}'
```

## Defense Notes

- `Paid` orders cannot be cancelled because cancellation is allowed only for `Pending`.
- Amount must be greater than zero in both services.
- Payment amounts greater than `100000` are declined.
- Order Service uses a custom outbound HTTP client with a `2s` timeout.
- When Payment Service is down, Order Service returns `503` and sets order status to `Failed`.

## Verification

`go test ./...` builds both services successfully, but in this environment the Go tool exits with a cache-permission warning when trimming the global build cache. The packages themselves compile.
