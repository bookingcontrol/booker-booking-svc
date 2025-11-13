package booking

import (
	"context"
	"time"
)

// Booking represents a booking entity
type Booking struct {
	ID            string
	VenueID       string
	TableID       string
	Date          string
	StartTime     string
	EndTime       string
	PartySize     int32
	CustomerName  string
	CustomerPhone string
	Status        string
	Comment       string
	AdminID       string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ExpiresAt     *time.Time
}

// BookingFilters represents filters for listing bookings
type BookingFilters struct {
	VenueID string
	Date    string
	Status  string
	TableID string
	Limit   int32
	Offset  int32
}

// Repository defines interface for booking repository operations
type Repository interface {
	CreateBooking(ctx context.Context, booking *Booking) error
	GetBooking(ctx context.Context, id string) (*Booking, error)
	ListBookings(ctx context.Context, filters *BookingFilters) ([]*Booking, int32, error)
	UpdateBookingStatus(ctx context.Context, id, status string) error
	CheckTableAvailability(ctx context.Context,
		venueID string, tableIDs []string, date, startTime, endTime string) (map[string]bool, error)
	GetExpiredHolds(ctx context.Context) ([]*Booking, error)
	AddToOutbox(ctx context.Context, topic, key string, payload []byte) error
	GetPendingOutbox(ctx context.Context, limit int32) ([]*OutboxMessage, error)
	UpdateOutboxStatus(ctx context.Context, id, status string, retryCount int32) error
}

// OutboxMessage represents a message in the outbox table
type OutboxMessage struct {
	ID         string
	Topic      string
	Key        string
	Payload    []byte
	Status     string
	RetryCount int32
	CreatedAt  time.Time
}

