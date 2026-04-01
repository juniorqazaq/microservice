# Payment Service

## Overview
Payment microservice adhering to Clean Architecture principles. It exposes a REST API for authorizing payments.

## Architecture Decisions
- **Clean Architecture & Ports/Adapters**: The service boundaries are strictly defined. HTTP handlers (transport) depend on the UseCases (domain logic). UseCases depend on Repository interfaces.
- **Manual Dependency Injection**: Conducted entirely in `main.go`. This keeps business logic completely decoupled from framework wiring.
- **Database per Service**: It maintains its own Postgres database (`payment_db`) ensuring a bounded context.

## Bounded Context
The Payment Service operates on the `Payment` domain model. It does not know what an order is or how it functions—only that it receives an amount linked to an arbitrary `order_id` string and must authorize or decline based on the sum (`<= 100000`).

## Business Rules
- Amount must be greater than `0`
- Amounts over `100000` are stored as `Declined`
- Successful payments are stored as `Authorized` with a generated `transaction_id`

## How to Run

1. Start Postgres and configure the database name `payment_db`:
   ```bash
   CREATE DATABASE payment_db;
   ```
2. Run Database Migrations:
   ```bash
   psql -d payment_db -f migrations/001_init.sql
   ```
3. Start the service:
   ```bash
   cd payment-service
   go run cmd/payment-service/main.go
   ```

Default local database URL:

```bash
postgres://postgres:postgres@localhost:5433/payment_db?sslmode=disable
```

## API Endpoints

### 1. Authorize Payment `POST /payments`
```bash
curl -X POST http://localhost:8081/payments \
-H "Content-Type: application/json" \
-d '{"order_id": "xyz123", "amount": 15000}'
```

### 2. Get Payment by Order ID `GET /payments/:order_id`
```bash
curl http://localhost:8081/payments/xyz123
```
