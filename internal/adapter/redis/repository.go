package redis

import (
	"context"
	"time"

	dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
	"github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/redis"
)

type HoldRepository struct {
	client *redis.Client
}

func NewHoldRepository(client *redis.Client) dom.HoldRepository {
	return &HoldRepository{client: client}
}

func (r *HoldRepository) SetHold(ctx context.Context, key string, bookingID string, ttl time.Duration) (bool, error) {
	return r.client.SetNX(ctx, key, bookingID, ttl).Result()
}

func (r *HoldRepository) GetHold(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *HoldRepository) DeleteHold(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

