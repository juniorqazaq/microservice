package domain

import "time"

type Order struct {
	ID         string
	CustomerID string
	ItemName   string
	Amount     int64
	Status     string
	CreatedAt  time.Time
}

type OrderStatusEvent struct {
	OrderID   string
	Status    string
	ChangedAt time.Time
	Source    string
}
