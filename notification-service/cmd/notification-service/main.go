package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"notification-service/internal/consumer"
	"notification-service/internal/provider"
)

func main() {
	rmqURL := mustEnv("RABBITMQ_URL")
	redisAddr := mustEnv("REDIS_ADDR")
	providerMode := os.Getenv("PROVIDER_MODE")

	log.Println("Starting Notification Service...")

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()

	var emailSender provider.EmailSender
	if providerMode == "REAL" {
		emailSender = provider.NewRealProvider(
			mustEnv("SMTP_HOST"),
			mustEnv("SMTP_PORT"),
			mustEnv("SMTP_USER"),
			mustEnv("SMTP_PASSWORD"),
			mustEnv("SMTP_FROM"),
		)
	} else {
		emailSender = provider.NewSimulatedProvider()
	}

	cons, err := consumer.NewRabbitMQConsumer(rmqURL, redisClient, emailSender)
	if err != nil {
		log.Fatalf("failed to initialize consumer: %v", err)
	}
	defer cons.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := cons.Start(ctx); err != nil {
			log.Fatalf("Consumer stopped with error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Notification Service gracefully...")
	cancel()
	log.Println("Notification Service stopped")
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("%s environment variable must be set", key)
	}
	return val
}
