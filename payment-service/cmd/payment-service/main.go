package main

import (
	"database/sql"
	"log"
	"net"
	"os"

	paymentv1 "github.com/youruser/ap2-generated-contracts/payment/v1"
	_ "github.com/lib/pq"
	"payment-service/internal/repository"
	grpctransport "payment-service/internal/transport/grpc"
	"payment-service/internal/usecase"
	"google.golang.org/grpc"
)

func main() {
	dbConnStr := mustEnv("PAYMENT_DATABASE_URL")
	grpcAddr := mustEnv("PAYMENT_GRPC_ADDR")

	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	repo := repository.NewPaymentRepository(db)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen for gRPC server: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(grpctransport.LoggingInterceptor))
	paymentv1.RegisterPaymentServiceServer(grpcServer, grpctransport.NewPaymentServer(uc))

	log.Printf("Payment gRPC service listening on %s", grpcAddr)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}

func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s must be set", key)
	}
	return value
}
