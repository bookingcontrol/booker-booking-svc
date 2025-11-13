package booking

import (
	"context"
)

// EventRepository defines interface for event operations (Outbox pattern)
type EventRepository interface {
	AddToOutbox(ctx context.Context, topic, key string, payload []byte) error
	GetPendingOutbox(ctx context.Context, limit int32) ([]*OutboxMessage, error)
	UpdateOutboxStatus(ctx context.Context, id, status string, retryCount int32) error
}

