package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQConsumer struct {
	conn            *amqp.Connection
	ch              *amqp.Channel
	processedEvents sync.Map
}

type PaymentCompletedEvent struct {
	OrderID       string `json:"order_id"`
	Amount        int64  `json:"amount"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
}

func NewRabbitMQConsumer(amqpURL string) (*RabbitMQConsumer, error) {
	conn, err := amqp.Dial(amqpURL)
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
		conn: conn,
		ch:   ch,
	}, nil
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
			c.handleMessage(msg)
		}
	}
}

func (c *RabbitMQConsumer) handleMessage(msg amqp.Delivery) {
	paymentID := msg.MessageId
	if paymentID == "" {
		log.Println("[Warning] Message received without MessageId, processing anyway but cannot ensure idempotency.")
	} else {
		if _, exists := c.processedEvents.Load(paymentID); exists {
			log.Printf("[Idempotency] Skipping already processed message ID: %s", paymentID)
			_ = msg.Ack(false)
			return
		}
	}

	var event PaymentCompletedEvent
	if err := json.Unmarshal(msg.Body, &event); err != nil {
		log.Printf("[Error] Failed to unmarshal message: %v", err)
		_ = msg.Nack(false, false)
		return
	}

	if strings.Contains(event.CustomerEmail, "fail@") || strings.Contains(event.CustomerEmail, "simulate_dlq") {
		log.Printf("[Error] Simulating permanent failure for email: %s", event.CustomerEmail)
		_ = msg.Nack(false, false)
		return
	}

	log.Printf("[Notification] Sent email to %s for Order #%s. Amount: $%d", event.CustomerEmail, event.OrderID, event.Amount/100)
	
	if paymentID != "" {
		c.processedEvents.Store(paymentID, true)
	}

	if err := msg.Ack(false); err != nil {
		log.Printf("[Error] Failed to ACK message: %v", err)
	}
}

func (c *RabbitMQConsumer) Close() {
	if c.ch != nil {
		_ = c.ch.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
}
