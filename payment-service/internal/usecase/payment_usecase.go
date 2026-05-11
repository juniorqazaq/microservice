package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"payment-service/internal/domain"
)

type PaymentRepository interface {
	Create(ctx context.Context, p *domain.Payment) error
	GetByOrderID(ctx context.Context, orderID string) (*domain.Payment, error)
}

type PaymentEventPublisher interface {
	PublishPaymentCompleted(ctx context.Context, payment *domain.Payment) error
}

type PaymentUseCase struct {
	repo      PaymentRepository
	publisher PaymentEventPublisher
}

var ErrInvalidAmount = errors.New("amount must be greater than 0")

func NewPaymentUseCase(repo PaymentRepository, publisher PaymentEventPublisher) *PaymentUseCase {
	return &PaymentUseCase{
		repo:      repo,
		publisher: publisher,
	}
}

func (uc *PaymentUseCase) ProcessPayment(ctx context.Context, orderID string, amount int64, customerEmail string) (*domain.Payment, error) {
	if amount <= 0 {
		return nil, ErrInvalidAmount
	}

	payment := &domain.Payment{
		ID:            uuid.New().String(),
		OrderID:       orderID,
		CustomerEmail: customerEmail,
		Amount:        amount,
		CreatedAt:     time.Now(),
	}

	if amount > 100000 {
		payment.Status = "Declined"
		payment.TransactionID = ""
	} else {
		payment.Status = "Authorized"
		payment.TransactionID = "txn_" + uuid.New().String()
	}

	if err := uc.repo.Create(ctx, payment); err != nil {
		return nil, err
	}

	if payment.Status == "Authorized" && uc.publisher != nil {
		if err := uc.publisher.PublishPaymentCompleted(ctx, payment); err != nil {
			return nil, err
		}
	}

	return payment, nil
}

func (uc *PaymentUseCase) GetPaymentByOrderID(ctx context.Context, orderID string) (*domain.Payment, error) {
	return uc.repo.GetByOrderID(ctx, orderID)
}
