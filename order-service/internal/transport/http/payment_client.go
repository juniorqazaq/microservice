package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type PaymentClient struct {
	baseURL string
	client  *http.Client
}

func NewPaymentClient(baseURL string) *PaymentClient {
	return &PaymentClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

type authRequest struct {
	OrderID string `json:"order_id"`
	Amount  int64  `json:"amount"`
}

type authResponse struct {
	Status string `json:"status"`
}

func (c *PaymentClient) AuthorizePayment(ctx context.Context, orderID string, amount int64) (string, error) {
	reqBody := authRequest{
		OrderID: orderID,
		Amount:  amount,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payment request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/payments", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create payment request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("payment service returned non-200 status: %d", resp.StatusCode)
	}

	var resData authResponse
	if err := json.NewDecoder(resp.Body).Decode(&resData); err != nil {
		return "", fmt.Errorf("failed to decode payment response: %w", err)
	}

	return resData.Status, nil
}
