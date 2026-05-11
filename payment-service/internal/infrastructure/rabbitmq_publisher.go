package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"payment-service/internal/domain"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQPublisher struct {
	conn     *amqp.Connection
	ch       *amqp.Channel
	confirms <-chan amqp.Confirmation
}

type PaymentCompletedEvent struct {
	EventID       string `json:"event_id"`
	OrderID       string `json:"order_id"`
	Amount        int64  `json:"amount"`
	CustomerEmail string `json:"customer_email"`
	Status        string `json:"status"`
}

func NewRabbitMQPublisher(amqpURL string) (*RabbitMQPublisher, error) {
	conn, err := dialWithRetry(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open a channel: %w", err)
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
		return nil, fmt.Errorf("failed to declare an exchange: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		return nil, fmt.Errorf("failed to enable publisher confirms: %w", err)
	}

	return &RabbitMQPublisher{
		conn:     conn,
		ch:       ch,
		confirms: ch.NotifyPublish(make(chan amqp.Confirmation, 1)),
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

func (p *RabbitMQPublisher) PublishPaymentCompleted(ctx context.Context, payment *domain.Payment) error {
	event := PaymentCompletedEvent{
		EventID:       payment.ID,
		OrderID:       payment.OrderID,
		Amount:        payment.Amount,
		CustomerEmail: payment.CustomerEmail,
		Status:        payment.Status,
	}

	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	err = p.ch.PublishWithContext(ctx,
		"payment_events",
		"payment.completed",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
			MessageId:    payment.ID,
		})
	if err != nil {
		return fmt.Errorf("failed to publish a message: %w", err)
	}
	select {
	case confirmation := <-p.confirms:
		if !confirmation.Ack {
			return fmt.Errorf("broker did not confirm payment event for payment ID %s", payment.ID)
		}
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for broker confirmation: %w", ctx.Err())
	}

	log.Printf(" [x] Published PaymentCompletedEvent for OrderID: %s", payment.OrderID)
	return nil
}

func (p *RabbitMQPublisher) Close() {
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
}
