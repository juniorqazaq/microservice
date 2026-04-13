package repository

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"time"

	"order-service/internal/domain"
)

type OrderRepository struct {
	db               *sql.DB
	mu               sync.RWMutex
	nextSubscriberID int
	subscribers      map[string]map[int]chan domain.OrderStatusEvent
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{
		db:          db,
		subscribers: make(map[string]map[int]chan domain.OrderStatusEvent),
	}
}

func (r *OrderRepository) Create(ctx context.Context, o *domain.Order) error {
	query := `
		INSERT INTO orders (id, customer_id, item_name, amount, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query, o.ID, o.CustomerID, o.ItemName, o.Amount, o.Status, o.CreatedAt)
	if err != nil {
		return err
	}

	r.publish(domain.OrderStatusEvent{
		OrderID:   o.ID,
		Status:    o.Status,
		ChangedAt: o.CreatedAt,
		Source:    "repository_create",
	})
	return nil
}

func (r *OrderRepository) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	query := `
		SELECT id, customer_id, item_name, amount, status, created_at
		FROM orders
		WHERE id = $1
	`
	var o domain.Order
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&o.ID, &o.CustomerID, &o.ItemName, &o.Amount, &o.Status, &o.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `UPDATE orders SET status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}

	r.publish(domain.OrderStatusEvent{
		OrderID:   id,
		Status:    status,
		ChangedAt: time.Now(),
		Source:    "repository_update",
	})
	return nil
}

func (r *OrderRepository) SaveIdempotencyKey(ctx context.Context, key string, orderID string) error {
	query := `INSERT INTO idempotency_keys (key, order_id) VALUES ($1, $2)`
	_, err := r.db.ExecContext(ctx, query, key, orderID)
	return err
}

func (r *OrderRepository) GetOrderByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error) {
	query := `
		SELECT o.id, o.customer_id, o.item_name, o.amount, o.status, o.created_at
		FROM orders o
		JOIN idempotency_keys ik ON o.id = ik.order_id
		WHERE ik.key = $1
	`
	var o domain.Order
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&o.ID, &o.CustomerID, &o.ItemName, &o.Amount, &o.Status, &o.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &o, nil
}

func (r *OrderRepository) Subscribe(orderID string) (<-chan domain.OrderStatusEvent, func()) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.subscribers[orderID] == nil {
		r.subscribers[orderID] = make(map[int]chan domain.OrderStatusEvent)
	}

	subscriberID := r.nextSubscriberID
	r.nextSubscriberID++

	ch := make(chan domain.OrderStatusEvent, 8)
	r.subscribers[orderID][subscriberID] = ch

	unsubscribe := func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		orderSubscribers := r.subscribers[orderID]
		if orderSubscribers == nil {
			return
		}

		if existing, ok := orderSubscribers[subscriberID]; ok {
			delete(orderSubscribers, subscriberID)
			close(existing)
		}

		if len(orderSubscribers) == 0 {
			delete(r.subscribers, orderID)
		}
	}

	return ch, unsubscribe
}

func (r *OrderRepository) publish(event domain.OrderStatusEvent) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, ch := range r.subscribers[event.OrderID] {
		select {
		case ch <- event:
		default:
		}
	}
}
