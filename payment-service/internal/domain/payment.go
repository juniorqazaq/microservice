package domain

import "time"

type Payment struct {
	ID            string
	OrderID       string
	CustomerEmail string
	TransactionID string
	Amount        int64
	Status        string
	CreatedAt     time.Time
}
