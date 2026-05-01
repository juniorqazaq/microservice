package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"notification-service/internal/consumer"
)

func main() {
	rmqURL := mustEnv("RABBITMQ_URL")

	log.Println("Starting Notification Service...")

	cons, err := consumer.NewRabbitMQConsumer(rmqURL)
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
