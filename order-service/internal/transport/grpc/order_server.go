package grpc

import (
	"context"
	"errors"

	orderv1 "github.com/youruser/ap2-generated-contracts/order/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"order-service/internal/domain"
	"order-service/internal/usecase"
)

type OrderServer struct {
	orderv1.UnimplementedOrderServiceServer
	usecase *usecase.OrderUseCase
}

func NewOrderServer(uc *usecase.OrderUseCase) *OrderServer {
	return &OrderServer{usecase: uc}
}

func (s *OrderServer) SubscribeToOrderUpdates(
	req *orderv1.OrderRequest,
	stream orderv1.OrderService_SubscribeToOrderUpdatesServer,
) error {
	if req.GetOrderId() == "" {
		return status.Error(codes.InvalidArgument, "order_id is required")
	}

	err := s.usecase.SubscribeToOrderUpdates(
		stream.Context(),
		req.GetOrderId(),
		func(event domain.OrderStatusEvent) error {
			return stream.Send(&orderv1.OrderStatusUpdate{
				OrderId:   event.OrderID,
				Status:    event.Status,
				ChangedAt: timestamppb.New(event.ChangedAt),
				Source:    event.Source,
			})
		},
	)
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	if errors.Is(err, usecase.ErrOrderNotFound) {
		return status.Error(codes.NotFound, err.Error())
	}
	return status.Error(codes.Internal, err.Error())
}
