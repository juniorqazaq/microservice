package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"notification-service/internal/provider"
)

type RabbitMQConsumer struct {
	conn        *amqp.Connection
	ch          *amqp.Channel
	redisClient *redis.Client
	emailSender provider.EmailSender
}

type PaymentCompletedEvent struct {
	EventID       string `json:"event_id"`
	OrderID       string `json:"order_id"`
	Amount        int64  `json:"amount"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
}

func NewRabbitMQConsumer(amqpURL string, redisClient *redis.Client, emailSender provider.EmailSender) (*RabbitMQConsumer, error) {
	conn, err := dialWithRetry(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
	}

	err = ch.ExchangeDeclare(
		"payment_events_dlx",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare DLX: %w", err)
	}

	dlq, err := ch.QueueDeclare(
		"payment_events_dlq",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare DLQ: %w", err)
	}

	err = ch.QueueBind(
		dlq.Name,
		"payment.completed",
		"payment_events_dlx",
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to bind DLQ: %w", err)
	}

	err = ch.ExchangeDeclare(
		"payment_events",
		"topic",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	args := amqp.Table{
		"x-dead-letter-exchange":    "payment_events_dlx",
		"x-dead-letter-routing-key": "payment.completed",
		"x-message-ttl":             int32(300000),
	}
	q, err := ch.QueueDeclare(
		"payment_completed_queue",
		true,
		false,
		false,
		false,
		args,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to declare a queue: %w", err)
	}

	err = ch.QueueBind(
		q.Name,
		"payment.completed",
		"payment_events",
		false,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to bind the queue: %w", err)
	}

	return &RabbitMQConsumer{
		conn:        conn,
		ch:          ch,
		redisClient: redisClient,
		emailSender: emailSender,
	}, nil
}

func dialWithRetry(amqpURL string) (*amqp.Connection, error) {
	var lastErr error
	for attempt := 1; attempt <= 30; attempt++ {
		conn, err := amqp.Dial(amqpURL)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		log.Printf("RabbitMQ is not ready yet, retrying connection attempt %d/30: %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	return nil, lastErr
}

func (c *RabbitMQConsumer) Start(ctx context.Context) error {
	msgs, err := c.ch.Consume(
		"payment_completed_queue",
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register a consumer: %w", err)
	}

	log.Println(" [*] Waiting for payment completion events. To exit press CTRL+C")

	for {
		select {
		case <-ctx.Done():
			log.Println("Stopping consumer loop...")
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("consumer channel closed")
			}
			c.handleMessage(ctx, msg)
		}
	}
}

func (c *RabbitMQConsumer) handleMessage(ctx context.Context, msg amqp.Delivery) {
	var event PaymentCompletedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("[Error] Failed to unmarshal message: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	paymentID := msg.MessageId
	if paymentID == "" {
		paymentID = event.EventID
	}
	if paymentID == "" {
		log.Println("[Error] Message received without payment ID, cannot ensure idempotency.")
		_ = msg.Nack(false, false)
		return
	}

	idempotencyKey := "notif:" + paymentID
	locked, err := c.redisClient.SetNX(ctx, idempotencyKey, "processing", 24*time.Hour).Result()
	if err != nil {
		log.Printf("[Error] Failed to check idempotency in Redis: %v", err)
		_ = msg.Nack(false, true)
		return
	}
	if !locked {
		log.Printf("[Idempotency] Skipping already processed payment ID: %s", paymentID)
		_ = msg.Ack(false)
		return
	}

	if err := sendWithRetry(ctx, c.emailSender, event); err != nil {
		log.Printf("[Error] Failed to send notification after retries: %v", err)
		_ = c.redisClient.Del(ctx, idempotencyKey).Err()
		_ = msg.Nack(false, false)
		return
	}

	if err := c.redisClient.Set(ctx, idempotencyKey, "done", 24*time.Hour).Err(); err != nil {
		log.Printf("[Error] Failed to mark notification as done in Redis: %v", err)
		_ = c.redisClient.Del(ctx, idempotencyKey).Err()
		_ = msg.Nack(false, true)
		return
	}
	if err := msg.Ack(false); err != nil {
		log.Printf("[Error] Failed to ACK message: %v", err)
	}
}

func sendWithRetry(ctx context.Context, sender provider.EmailSender, event PaymentCompletedEvent) error {
	subject := fmt.Sprintf("Payment completed for order %s", event.OrderID)
	body := fmt.Sprintf("Your payment for order %s was completed. Amount: $%.2f", event.OrderID, float64(event.Amount)/100)
	delays := []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second}

	var lastErr error
	for attempt := 0; attempt <= len(delays); attempt++ {
		if err := sender.Send(ctx, event.CustomerEmail, subject, body); err == nil {
			return nil
		} else {
			lastErr = err
			log.Printf("[Retry] Notification attempt %d failed: %v", attempt+1, err)
		}

		if attempt == len(delays) {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delays[attempt]):
		}
	}

	return lastErr
}

func (c *RabbitMQConsumer) Close() {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
