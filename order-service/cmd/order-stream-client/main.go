package main

import (
	"context"
	"io"
	"log"
	"os"

	orderv1 "github.com/youruser/ap2-generated-contracts/order/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	target := mustEnv("ORDER_GRPC_TARGET")
	orderID := mustEnv("ORDER_ID")

	conn, err := grpc.Dial(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to order service: %v", err)
	}
	defer conn.Close()

	client := orderv1.NewOrderServiceClient(conn)
	stream, err := client.SubscribeToOrderUpdates(context.Background(), &orderv1.OrderRequest{
		OrderId: orderID,
	})
	if err != nil {
		log.Fatalf("failed to subscribe to order updates: %v", err)
	}

	log.Printf("Subscribed to order %s on %s", orderID, target)

	for {
		update, err := stream.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Fatalf("stream receive failed: %v", err)
		}

		log.Printf(
			"order=%s status=%s source=%s changed_at=%s",
			update.GetOrderId(),
			update.GetStatus(),
			update.GetSource(),
			update.GetChangedAt().AsTime().Format("2006-01-02 15:04:05"),
		)
	}
}

func mustEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatalf("%s must be set", key)
	}
	return value
}
