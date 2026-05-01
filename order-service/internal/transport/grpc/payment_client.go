package grpc

import (
	"context"
	"time"

	paymentv1 "github.com/youruser/ap2-generated-contracts/payment/v1"
	"google.golang.org/grpc"
)

type PaymentClient struct {
	client paymentv1.PaymentServiceClient
}

func NewPaymentClient(conn grpc.ClientConnInterface) *PaymentClient {
	return &PaymentClient{
		client: paymentv1.NewPaymentServiceClient(conn),
	}
}

	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	response, err := c.client.ProcessPayment(callCtx, &paymentv1.PaymentRequest{
		OrderId:       orderID,
		Amount:        amount,
	})
	if err != nil {
		return "", err
	}

	return response.GetStatus(), nil
}
