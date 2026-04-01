# Order Service

## Overview
Order microservice built with Go and Gin following Clean Architecture. It creates orders and leverages the `payment-service` to process their status.

## Architecture Decisions
- **Clean Architecture**: Thin Gin handlers map requests directly to UseCase methods. 
- **Timeouts**: The core HTTP Client communicating with the Payment Service uses a strict 2-second timeout as required by the parameters.
- **Idempotency (Bonus)**: Implemented using a custom Postgres table (`idempotency_keys`) interacting with the OrderRepository. Passing the same `Idempotency-Key` header will return the cached original order, guarding against duplicate execution.
- **Database Ownership**: This service writes only to `order_db` and never touches payment persistence directly.

## Failure Handling
**Why "Failed" vs "Pending"?**
If the Payment service times out (takes >2 seconds) or is entirely unreachable, the system actively transitions the Order status from `Pending` directly to `Failed` and returns a `503 Service Unavailable`. 
This is a conscious architectural choice to provide **immediate eventual consistency**. Leaving it as `Pending` would require a cron job (or background worker) actively polling or retrying failed payment attempts. Transitioning it to `Failed` immediately closes the loop for the customer while signaling the true state of the internal system.

## How to Run

1. Start Postgres and configure the database name `order_db`:
   ```bash
   CREATE DATABASE order_db;
   ```
2. Run Database Migrations:
   ```bash
   psql -d order_db -f migrations/001_init.sql
   ```
3. Start the service:
   ```bash
   cd order-service
   # Setup PAYMENT_SERVICE_URL appropriately if testing custom domains
   go run cmd/order-service/main.go
   ```

Default local database URL:

```bash
postgres://postgres:postgres@localhost:5433/order_db?sslmode=disable
```

## API Endpoints

### 1. Create Order `POST /orders`
Include the `Idempotency-Key` to safely prevent duplicate orders.
```bash
curl -X POST http://localhost:8080/orders \
-H "Content-Type: application/json" \
-H "Idempotency-Key: test-key-123" \
-d '{"customer_id": "abc", "item_name": "Book", "amount": 15000}'
```

### 2. Get Order by ID `GET /orders/:id`
```bash
curl http://localhost:8080/orders/<order_id_from_creation>
```

### 3. Cancel Order `PATCH /orders/:id/cancel`
```bash
curl -X PATCH http://localhost:8080/orders/<order_id_from_creation>/cancel
```

## Architecture Diagram
```mermaid
graph TD
    Client[Client] -->|POST /orders| O_Handler(Order Handler)
    O_Handler --> O_UseCase(Order UseCase)
    O_UseCase --> O_Repo[(Order DB)]
    O_UseCase -->|Auth HTTP Client| P_Handler(Payment Handler)
    
    P_Handler --> P_UseCase(Payment UseCase)
    P_UseCase --> P_Repo[(Payment DB)]
```
