package main

import (
	"context"
	"database/sql"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	paymentv1 "github.com/youruser/ap2-generated-contracts/proto/payment/v1"
	"google.golang.org/grpc"
	"payment-service/internal/infrastructure"
	"payment-service/internal/repository"
	grpctransport "payment-service/internal/transport/grpc"
	"payment-service/internal/usecase"
)

func main() {
	dbConnStr := mustEnv("PAYMENT_DATABASE_URL")
	grpcAddr := mustEnv("PAYMENT_GRPC_ADDR")
	rabbitURL := mustEnv("RABBITMQ_URL")

	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}
	defer db.Close()

	repo := repository.NewPaymentRepository(db)
	publisher, err := infrastructure.NewRabbitMQPublisher(rabbitURL)
	if err != nil {
		log.Fatalf("failed to initialize RabbitMQ publisher: %v", err)
	}
	defer publisher.Close()
	uc := usecase.NewPaymentUseCase(repo, publisher)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen for gRPC server: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(grpctransport.LoggingInterceptor))
	paymentv1.RegisterPaymentServiceServer(grpcServer, grpctransport.NewPaymentServer(uc))

	log.Printf("Payment gRPC service listening on %s", grpcAddr)
	errCh := make(chan error, 1)
	go func() {
		errCh <- grpcServer.Serve(lis)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Println("Shutting down Payment Service gracefully...")
		grpcServer.GracefulStop()
	case err := <-errCh:
		if err != nil {
			log.Fatalf("failed to run server: %v", err)
		}
	}
}

func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s must be set", key)
	}
	return value
}
