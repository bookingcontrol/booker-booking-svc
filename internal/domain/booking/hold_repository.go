package booking

import (
	"context"
	"time"
)

// HoldRepository defines interface for hold operations (Redis)
type HoldRepository interface {
	SetHold(ctx context.Context, key string, bookingID string, ttl time.Duration) (bool, error)
	GetHold(ctx context.Context, key string) (string, error)
	DeleteHold(ctx context.Context, key string) error
}

