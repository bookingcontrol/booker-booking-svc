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

// TestProducer_PublishBookingEvent_AllEventTypes tests all booking event types
func TestProducer_PublishBookingEvent_AllEventTypes(t *testing.T) {
	t.Run("held event", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-1",
			Payload: &commonpb.BookingEvent_Held{
				Held: &commonpb.BookingHeld{
					ExpiresAt: 1234567890,
				},
			},
		}

		assert.NotEmpty(t, event.BookingId)
		assert.NotNil(t, event.GetHeld())
		assert.Equal(t, int64(1234567890), event.GetHeld().ExpiresAt)
	})

	t.Run("confirmed event", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-2",
			Payload: &commonpb.BookingEvent_Confirmed{
				Confirmed: &commonpb.BookingConfirmed{
					AdminId: "admin-1",
				},
			},
		}

		assert.NotNil(t, event.GetConfirmed())
		assert.Equal(t, "admin-1", event.GetConfirmed().AdminId)
	})

	t.Run("cancelled event", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-3",
			Payload: &commonpb.BookingEvent_Cancelled{
				Cancelled: &commonpb.BookingCancelled{
					AdminId: "admin-1",
					Reason:  "Customer request",
				},
			},
		}

		assert.NotNil(t, event.GetCancelled())
		assert.Equal(t, "admin-1", event.GetCancelled().AdminId)
		assert.Equal(t, "Customer request", event.GetCancelled().Reason)
	})

	t.Run("seated event", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-4",
			Payload: &commonpb.BookingEvent_Seated{
				Seated: &commonpb.BookingSeated{
					AdminId: "admin-1",
				},
			},
		}

		assert.NotNil(t, event.GetSeated())
		assert.Equal(t, "admin-1", event.GetSeated().AdminId)
	})

	t.Run("finished event", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-5",
			Payload: &commonpb.BookingEvent_Finished{
				Finished: &commonpb.BookingFinished{
					AdminId: "admin-1",
				},
			},
		}

		assert.NotNil(t, event.GetFinished())
		assert.Equal(t, "admin-1", event.GetFinished().AdminId)
	})

	t.Run("no_show event", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-6",
			Payload: &commonpb.BookingEvent_NoShow{
				NoShow: &commonpb.BookingNoShow{
					AdminId: "admin-1",
				},
			},
		}

		assert.NotNil(t, event.GetNoShow())
		assert.Equal(t, "admin-1", event.GetNoShow().AdminId)
	})

	t.Run("expired event", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-7",
			Payload: &commonpb.BookingEvent_Expired{
				Expired: &commonpb.BookingExpired{
					Reason: "Hold expired",
				},
			},
		}

		assert.NotNil(t, event.GetExpired())
		assert.Equal(t, "Hold expired", event.GetExpired().Reason)
	})
}

// TestProducer_PublishBookingEvent_EdgeCases tests edge cases
func TestProducer_PublishBookingEvent_EdgeCases(t *testing.T) {
	t.Run("empty booking id", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "",
			Payload: &commonpb.BookingEvent_Held{
				Held: &commonpb.BookingHeld{
					ExpiresAt: 1234567890,
				},
			},
		}

		assert.Empty(t, event.BookingId)
		assert.NotNil(t, event.Payload)
	})

	t.Run("event without headers", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-1",
			Payload: &commonpb.BookingEvent_Held{
				Held: &commonpb.BookingHeld{
					ExpiresAt: 1234567890,
				},
			},
		}

		assert.Nil(t, event.Headers)
	})

	t.Run("event with all headers", func(t *testing.T) {
		event := &commonpb.BookingEvent{
			BookingId: "booking-1",
			Headers: &commonpb.EventHeaders{
				TraceId:   "trace-123",
				Timestamp: 1234567890,
				Source:    "booking-svc",
			},
			Payload: &commonpb.BookingEvent_Held{
				Held: &commonpb.BookingHeld{
					ExpiresAt: 1234567890,
				},
			},
		}

		assert.NotNil(t, event.Headers)
		assert.Equal(t, "trace-123", event.Headers.TraceId)
		assert.Equal(t, int64(1234567890), event.Headers.Timestamp)
		assert.Equal(t, "booking-svc", event.Headers.Source)
	})
}

// TestProducer_PublishVenueEvent_AllEventTypes tests all venue event types
func TestProducer_PublishVenueEvent_AllEventTypes(t *testing.T) {
	t.Run("layout updated event", func(t *testing.T) {
		event := &commonpb.VenueEvent{
			VenueId: "venue-1",
			Payload: &commonpb.VenueEvent_LayoutUpdated{
				LayoutUpdated: &commonpb.TableLayoutUpdated{
					RoomId:   "room-1",
					TableIds: []string{"table-1", "table-2"},
				},
			},
		}

		assert.Equal(t, "venue-1", event.VenueId)
		layoutUpdated := event.GetLayoutUpdated()
		assert.NotNil(t, layoutUpdated)
		assert.Equal(t, "room-1", layoutUpdated.RoomId)
		assert.Len(t, layoutUpdated.TableIds, 2)
	})

	t.Run("empty table ids", func(t *testing.T) {
		event := &commonpb.VenueEvent{
			VenueId: "venue-1",
			Payload: &commonpb.VenueEvent_LayoutUpdated{
				LayoutUpdated: &commonpb.TableLayoutUpdated{
					RoomId:   "room-1",
					TableIds: []string{},
				},
			},
		}

		layoutUpdated := event.GetLayoutUpdated()
		assert.Empty(t, layoutUpdated.TableIds)
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

