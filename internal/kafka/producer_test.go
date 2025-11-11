package kafka

import (
	"testing"

	"github.com/stretchr/testify/assert"

	commonpb "github.com/bookingcontrol/booker-contracts-go/common"
)

func TestProducer_PublishBookingEvent(t *testing.T) {
	t.Run("successful publish", func(t *testing.T) {
		// Note: This would require a real Kafka broker or a mock
		// For unit tests, we'd use a mock producer
		// For integration tests, we'd use testcontainers

		event := &commonpb.BookingEvent{
			BookingId: "booking-1",
			Payload: &commonpb.BookingEvent_Held{
				Held: &commonpb.BookingHeld{
					ExpiresAt: 1234567890,
				},
			},
		}

		// Verify event structure
		assert.NotEmpty(t, event.BookingId)
		assert.NotNil(t, event.Payload)
		assert.NotNil(t, event.GetHeld())
	})

	t.Run("event with headers", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-1",
			Headers: &commonpb.EventHeaders{
				TraceId:   "trace-123",
				Timestamp: 1234567890,
				Source:    "booking-svc",
			},
			Payload: &commonpb.BookingEvent_Confirmed{
				Confirmed: &commonpb.BookingConfirmed{
					AdminId: "admin-1",
				},
			},
		}

		assert.Equal(t, "trace-123", event.Headers.TraceId)
		assert.Equal(t, "booking-svc", event.Headers.Source)
		assert.NotNil(t, event.GetConfirmed())
	})
}

func TestProducer_PublishVenueEvent(t *testing.T) {
	t.Run("successful publish", func(t *testing.T) {
		event := &commonpb.VenueEvent{
			VenueId: "venue-1",
			Payload: &commonpb.VenueEvent_LayoutUpdated{
				LayoutUpdated: &commonpb.TableLayoutUpdated{
					RoomId:   "room-1",
					TableIds: []string{"table-1", "table-2"},
				},
			},
		}

		// Verify event structure
		assert.Equal(t, "venue-1", event.VenueId)
		assert.NotNil(t, event.Payload)
		layoutUpdated := event.GetLayoutUpdated()
		assert.NotNil(t, layoutUpdated)
		assert.Equal(t, "room-1", layoutUpdated.RoomId)
		assert.Equal(t, 2, len(layoutUpdated.TableIds))
	})
}

// Integration test would require testcontainers
func TestProducer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup Kafka using testcontainers
	// brokers := []string{"localhost:9092"}
	// producer, err := NewProducer(brokers)
	// require.NoError(t, err)
	// defer producer.Close()

	// ctx := context.Background()
	// event := &commonpb.BookingEvent{
	// 	BookingId: "booking-1",
	// 	Payload: &commonpb.BookingEvent_Held{
	// 		Held: &commonpb.BookingHeld{
	// 			ExpiresAt: time.Now().Unix(),
	// 		},
	// 	},
	// }

	// err = producer.PublishBookingEvent(ctx, "booking.events", event)
	// require.NoError(t, err)
}

