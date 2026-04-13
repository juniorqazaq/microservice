package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"order-service/internal/domain"
)

type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) error
	GetByID(ctx context.Context, id string) (*domain.Order, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	SaveIdempotencyKey(ctx context.Context, key string, orderID string) error
	GetOrderByIdempotencyKey(ctx context.Context, key string) (*domain.Order, error)
	Subscribe(orderID string) (<-chan domain.OrderStatusEvent, func())
}

type PaymentClient interface {
	AuthorizePayment(ctx context.Context, orderID string, amount int64) (string, error)
}

type OrderUseCase struct {
	repo          OrderRepository
	paymentClient PaymentClient
}

func NewOrderUseCase(repo OrderRepository, paymentClient PaymentClient) *OrderUseCase {
	return &OrderUseCase{
		repo:          repo,
		paymentClient: paymentClient,
	}
}

var ErrPaymentServiceUnavailable = errors.New("payment service unavailable")
var ErrInvalidAmount = errors.New("amount must be greater than 0")
var ErrOrderCannotBeCancelled = errors.New("only pending orders can be cancelled")
var ErrOrderNotFound = errors.New("order not found")

func (uc *OrderUseCase) CreateOrder(ctx context.Context, customerID string, itemName string, amount int64, idempotencyKey string) (*domain.Order, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	if idempotencyKey != "" {
		existingOrder, err := uc.repo.GetOrderByIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			return nil, err
		}
		if existingOrder != nil {
			return existingOrder, nil
		}
	}

	order := &domain.Order{
		ID:         uuid.New().String(),
		CustomerID: customerID,
		ItemName:   itemName,
		Amount:     amount,
		Status:     "Pending",
		CreatedAt:  time.Now(),
	}

	if err := uc.repo.Create(ctx, order); err != nil {
		return nil, err
	}

	if idempotencyKey != "" {
		if err := uc.repo.SaveIdempotencyKey(ctx, idempotencyKey, order.ID); err != nil {
			return nil, err
		}
	}

	status, err := uc.paymentClient.AuthorizePayment(ctx, order.ID, order.Amount)

	newStatus := ""
	var returnErr error

	if err != nil {
		newStatus = "Failed"
		returnErr = ErrPaymentServiceUnavailable
	} else if status == "Authorized" {
		newStatus = "Paid"
	} else {
		newStatus = "Failed"
	}

	_ = uc.repo.UpdateStatus(ctx, order.ID, newStatus)
	order.Status = newStatus

	return order, returnErr
}

func (uc *OrderUseCase) GetOrderByID(ctx context.Context, id string) (*domain.Order, error) {
	return uc.repo.GetByID(ctx, id)
}

func (uc *OrderUseCase) CancelOrder(ctx context.Context, id string) (*domain.Order, error) {
	order, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if order == nil {
		return nil, nil
	}

	if order.Status != "Pending" {
		return nil, ErrOrderCannotBeCancelled
	}

	if err := uc.repo.UpdateStatus(ctx, id, "Cancelled"); err != nil {
		return nil, err
	}
	order.Status = "Cancelled"

	return order, nil
}

func (uc *OrderUseCase) SubscribeToOrderUpdates(
	ctx context.Context,
	orderID string,
	emit func(domain.OrderStatusEvent) error,
) error {
	order, err := uc.repo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return ErrOrderNotFound
	}

	lastStatus := order.Status
	if err := emit(domain.OrderStatusEvent{
		OrderID:   order.ID,
		Status:    order.Status,
		ChangedAt: order.CreatedAt,
		Source:    "initial_snapshot",
	}); err != nil {
		return err
	}

	updates, unsubscribe := uc.repo.Subscribe(orderID)
	defer unsubscribe()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case update := <-updates:
			if update.Status == lastStatus {
				continue
			}
			lastStatus = update.Status
			if err := emit(update); err != nil {
				return err
			}
		case <-ticker.C:
			current, err := uc.repo.GetByID(ctx, orderID)
			if err != nil {
				return err
			}
			if current == nil {
				return ErrOrderNotFound
			}
			if current.Status == lastStatus {
				continue
			}
			lastStatus = current.Status
			if err := emit(domain.OrderStatusEvent{
				OrderID:   current.ID,
				Status:    current.Status,
				ChangedAt: time.Now(),
				Source:    "database_poll",
			}); err != nil {
				return err
			}
		}
	}
}
