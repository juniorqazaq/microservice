package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"order-service/internal/domain"
)

type OrderCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewOrderCache(client *redis.Client, ttl time.Duration) *OrderCache {
	return &OrderCache{
		client: client,
		ttl:    ttl,
	}
}

func (c *OrderCache) Get(ctx context.Context, id string) (*domain.Order, error) {
	data, err := c.client.Get(ctx, key(id)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var order domain.Order
	if err := json.Unmarshal([]byte(data), &order); err != nil {
		return nil, err
	}

	return &order, nil
}

func (c *OrderCache) Set(ctx context.Context, order *domain.Order) error {
	data, err := json.Marshal(order)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, key(order.ID), data, c.ttl).Err()
}

func (c *OrderCache) Delete(ctx context.Context, id string) error {
	return c.client.Del(ctx, key(id)).Err()
}

func key(id string) string {
	return fmt.Sprintf("order:%s", id)
}
