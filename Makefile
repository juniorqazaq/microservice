.PHONY: db-up db-down wait-db migrate-order migrate-payment run-order run-payment

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

wait-db:
	until docker compose exec -T postgres pg_isready -U postgres >/dev/null 2>&1; do sleep 1; done

migrate-order: wait-db
	docker compose exec -T postgres psql -U postgres -d order_db < order-service/migrations/001_init.sql

migrate-payment: wait-db
	docker compose exec -T postgres psql -U postgres -d payment_db < payment-service/migrations/001_init.sql

run-order:
	cd /Users/admin/Assignment1GO_Sanat/order-service && DEBUG=true DATABASE_URL="$${ORDER_DATABASE_URL:-postgres://postgres:postgres@localhost:5433/order_db?sslmode=disable}" PAYMENT_SERVICE_URL="$${PAYMENT_SERVICE_URL:-http://localhost:8081}" go run ./cmd/order-service/main.go

run-payment:
	cd /Users/admin/Assignment1GO_Sanat/payment-service && DEBUG=true DATABASE_URL="$${PAYMENT_DATABASE_URL:-postgres://postgres:postgres@localhost:5433/payment_db?sslmode=disable}" go run ./cmd/payment-service/main.go
