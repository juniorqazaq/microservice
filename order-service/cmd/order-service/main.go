package main

import (
	"database/sql"
	"errors"
	"log"
	"net"
	nethttp "net/http"
	"os"

	"github.com/gin-gonic/gin"
	orderv1 "github.com/youruser/ap2-generated-contracts/order/v1"
	_ "github.com/lib/pq"
	"order-service/internal/repository"
	grpctransport "order-service/internal/transport/grpc"
	"order-service/internal/transport/http"
	"order-service/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	dbConnStr := mustEnv("ORDER_DATABASE_URL")
	httpAddr := mustEnv("ORDER_HTTP_ADDR")
	grpcAddr := mustEnv("ORDER_GRPC_ADDR")
	paymentTarget := mustEnv("PAYMENT_GRPC_TARGET")

	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}

	repo := repository.NewOrderRepository(db)

	paymentConn, err := grpc.Dial(
		paymentTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect to payment service: %v", err)
	}
	defer paymentConn.Close()

	paymentClient := grpctransport.NewPaymentClient(paymentConn)
	uc := usecase.NewOrderUseCase(repo, paymentClient)

	r := gin.Default()
	http.NewOrderHandler(r, uc)

	httpServer := &nethttp.Server{
		Addr:    httpAddr,
		Handler: r,
	}

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen for gRPC server: %v", err)
	}

	grpcServer := grpc.NewServer()
	orderv1.RegisterOrderServiceServer(grpcServer, grpctransport.NewOrderServer(uc))

	errCh := make(chan error, 2)

	go func() {
		log.Printf("Order HTTP service listening on %s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		log.Printf("Order gRPC service listening on %s", grpcAddr)
		errCh <- grpcServer.Serve(lis)
	}()

	if err := <-errCh; err != nil {
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
