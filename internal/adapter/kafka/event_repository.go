package kafka

import (
	"context"

	dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
)

type EventRepository struct {
	dbRepo dom.Repository
}

func NewEventRepository(dbRepo dom.Repository) dom.EventRepository {
	return &EventRepository{dbRepo: dbRepo}
}

func (r *EventRepository) AddToOutbox(ctx context.Context, topic, key string, payload []byte) error {
	return r.dbRepo.AddToOutbox(ctx, topic, key, payload)
}

func (r *EventRepository) GetPendingOutbox(ctx context.Context, limit int32) ([]*dom.OutboxMessage, error) {
	return r.dbRepo.GetPendingOutbox(ctx, limit)
}

func (r *EventRepository) UpdateOutboxStatus(ctx context.Context, id, status string, retryCount int32) error {
	return r.dbRepo.UpdateOutboxStatus(ctx, id, status, retryCount)
}

