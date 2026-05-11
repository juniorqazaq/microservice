package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	nethttp "net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	orderv1 "github.com/youruser/ap2-generated-contracts/proto/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"order-service/internal/cache"
	"order-service/internal/middleware"
	"order-service/internal/repository"
	grpctransport "order-service/internal/transport/grpc"
	"order-service/internal/transport/http"
	"order-service/internal/usecase"
)

func main() {
	dbConnStr := mustEnv("ORDER_DATABASE_URL")
	httpAddr := mustEnv("ORDER_HTTP_ADDR")
	grpcAddr := mustEnv("ORDER_GRPC_ADDR")
	paymentTarget := mustEnv("PAYMENT_GRPC_TARGET")
	redisAddr := mustEnv("REDIS_ADDR")
	cacheTTL := mustDurationEnv("CACHE_TTL_SECONDS", 300)

	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}
	defer db.Close()

	repo := repository.NewOrderRepository(db)
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()
	orderCache := cache.NewOrderCache(redisClient, cacheTTL)

	paymentConn, err := grpc.Dial(
		paymentTarget,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect to payment service: %v", err)
	}
	defer paymentConn.Close()

	paymentClient := grpctransport.NewPaymentClient(paymentConn)
	uc := usecase.NewOrderUseCase(repo, paymentClient, orderCache)

	r := gin.Default()
	r.Use(middleware.RateLimiter(redisClient, 10, time.Minute))
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		log.Println("Shutting down Order Service gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("failed to shutdown HTTP server: %v", err)
		}
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

func mustDurationEnv(key string, defaultSeconds int) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return time.Duration(defaultSeconds) * time.Second
	}

	seconds, err := strconv.Atoi(value)
	if err != nil || seconds <= 0 {
		log.Fatalf("%s must be a positive integer number of seconds", key)
	}

	return time.Duration(seconds) * time.Second
}
