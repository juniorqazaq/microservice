package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"order-service/internal/repository"
	"order-service/internal/transport/http"
	"order-service/internal/usecase"
)

func main() {
	dbConnStr := os.Getenv("DATABASE_URL")
	if dbConnStr == "" {
		dbConnStr = "postgres://postgres:postgres@localhost:5433/order_db?sslmode=disable"
	}
	paymentServiceURL := os.Getenv("PAYMENT_SERVICE_URL")
	if paymentServiceURL == "" {
		paymentServiceURL = "http://localhost:8081"
	}

	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	repo := repository.NewOrderRepository(db)
	paymentClient := http.NewPaymentClient(paymentServiceURL)
	uc := usecase.NewOrderUseCase(repo, paymentClient)

	r := gin.Default()
	http.NewOrderHandler(r, uc)

	log.Println("Order service starting on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
