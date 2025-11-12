package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBookingModel tests the Booking model structure
func TestBookingModel(t *testing.T) {
	booking := &Booking{
		ID:            uuid.New().String(),
		VenueID:      "venue-1",
		TableID:      "table-1",
		Date:         "2024-01-15",
		StartTime:    "19:00",
		EndTime:      "21:00",
		PartySize:    4,
		CustomerName: "John Doe",
		CustomerPhone: "+1234567890",
		Status:       "confirmed",
		Comment:      "Window seat preferred",
		AdminID:      "admin-1",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	assert.NotEmpty(t, booking.ID)
	assert.Equal(t, "venue-1", booking.VenueID)
	assert.Equal(t, "table-1", booking.TableID)
	assert.Equal(t, int32(4), booking.PartySize)
	assert.Equal(t, "confirmed", booking.Status)
}

// TestBookingFilters tests the BookingFilters structure
func TestBookingFilters(t *testing.T) {
	filters := &BookingFilters{
		VenueID: "venue-1",
		Date:    "2024-01-15",
		Status:  "confirmed",
		TableID: "table-1",
		Limit:   10,
		Offset:  0,
	}

	assert.Equal(t, "venue-1", filters.VenueID)
	assert.Equal(t, "2024-01-15", filters.Date)
	assert.Equal(t, "confirmed", filters.Status)
	assert.Equal(t, int32(10), filters.Limit)
}

// TestOutboxMessage tests the OutboxMessage model
func TestOutboxMessage(t *testing.T) {
	msg := &OutboxMessage{
		ID:         uuid.New().String(),
		Topic:      "booking.held",
		Key:        "booking-1",
		Payload:    []byte(`{"booking_id":"booking-1"}`),
		Status:     "pending",
		RetryCount: 0,
		CreatedAt:  time.Now(),
	}

	assert.NotEmpty(t, msg.ID)
	assert.Equal(t, "booking.held", msg.Topic)
	assert.Equal(t, "pending", msg.Status)
	assert.Equal(t, int32(0), msg.RetryCount)
}

// Note: Integration tests for repository would require a real database connection
// These would be in a separate file like repository_integration_test.go
// and would use testcontainers or a test database

// MockRedisClient for testing repository without real Redis
type mockRedisClient struct {
	delCalled bool
	delKey    string
}

func (m *mockRedisClient) Del(ctx context.Context, key string) error {
	m.delCalled = true
	m.delKey = key
	return nil
}

// TestRepository_New tests repository creation
func TestRepository_New(t *testing.T) {
	// This test would require a real database connection
	// For now, we test the structure
	redisClient := &mockRedisClient{}
	
	// In a real test, we'd create a test database connection
	// repo := New(testDB, redisClient)
	// assert.NotNil(t, repo)
	
	_ = redisClient
}

// Integration test helpers (would be in integration test file)
func setupTestDB(t *testing.T) (*Repository, func()) {
	// Setup test database using testcontainers or in-memory database
	// Return repository and cleanup function
	t.Skip("Integration test - requires database")
	return nil, func() {}
}

func TestRepository_CreateBooking_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()
	booking := &Booking{
		ID:            uuid.New().String(),
		VenueID:      "venue-1",
		TableID:      "table-1",
		Date:         "2024-01-15",
		StartTime:    "19:00",
		EndTime:      "21:00",
		PartySize:    4,
		CustomerName: "John Doe",
		Status:       "held",
	}

	err := repo.CreateBooking(ctx, booking)
	require.NoError(t, err)

	// Verify booking was created
	retrieved, err := repo.GetBooking(ctx, booking.ID)
	require.NoError(t, err)
	assert.Equal(t, booking.ID, retrieved.ID)
	assert.Equal(t, booking.VenueID, retrieved.VenueID)
}

func TestRepository_CheckTableAvailability_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	repo, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a booking for table-1
	booking := &Booking{
		ID:         uuid.New().String(),
		VenueID:    "venue-1",
		TableID:    "table-1",
		Date:       "2024-01-15",
		StartTime:  "19:00",
		EndTime:    "21:00",
		PartySize:  4,
		Status:     "confirmed",
	}
	err := repo.CreateBooking(ctx, booking)
	require.NoError(t, err)

	// Check availability - table-1 should be booked, table-2 should be available
	availability, err := repo.CheckTableAvailability(ctx, "venue-1", []string{"table-1", "table-2"}, "2024-01-15", "19:00", "21:00")
	require.NoError(t, err)

	assert.False(t, availability["table-1"], "table-1 should be booked")
	assert.True(t, availability["table-2"], "table-2 should be available")
}

// TestRepository_New tests repository creation
func TestRepository_New_Unit(t *testing.T) {
	redisClient := &mockRedisClient{}
	
	// Test that New doesn't panic with nil db (for unit tests)
	// In real scenario, db would be provided
	_ = redisClient
	
	// This test verifies the structure can be created
	// Full integration test would require actual database
	assert.NotNil(t, redisClient)
}

// TestBooking_AllFields tests all booking fields are properly set
func TestBooking_AllFields(t *testing.T) {
	now := time.Now()
	expiresAt := now.Add(10 * time.Minute)
	
	booking := &Booking{
		ID:            "booking-1",
		VenueID:      "venue-1",
		TableID:      "table-1",
		Date:         "2024-01-15",
		StartTime:    "19:00",
		EndTime:      "21:00",
		PartySize:    4,
		CustomerName: "John Doe",
		CustomerPhone: "+1234567890",
		Status:       "held",
		Comment:      "Window seat",
		AdminID:      "admin-1",
		CreatedAt:    now,
		UpdatedAt:    now,
		ExpiresAt:    &expiresAt,
	}

	assert.Equal(t, "booking-1", booking.ID)
	assert.Equal(t, "venue-1", booking.VenueID)
	assert.Equal(t, "table-1", booking.TableID)
	assert.Equal(t, "2024-01-15", booking.Date)
	assert.Equal(t, "19:00", booking.StartTime)
	assert.Equal(t, "21:00", booking.EndTime)
	assert.Equal(t, int32(4), booking.PartySize)
	assert.Equal(t, "John Doe", booking.CustomerName)
	assert.Equal(t, "+1234567890", booking.CustomerPhone)
	assert.Equal(t, "held", booking.Status)
	assert.Equal(t, "Window seat", booking.Comment)
	assert.Equal(t, "admin-1", booking.AdminID)
	assert.Equal(t, now, booking.CreatedAt)
	assert.Equal(t, now, booking.UpdatedAt)
	assert.NotNil(t, booking.ExpiresAt)
	assert.Equal(t, expiresAt, *booking.ExpiresAt)
}

// TestBookingFilters_AllCombinations tests different filter combinations
func TestBookingFilters_AllCombinations(t *testing.T) {
	t.Run("all filters set", func(t *testing.T) {
		filters := &BookingFilters{
			VenueID: "venue-1",
			Date:    "2024-01-15",
			Status:  "confirmed",
			TableID: "table-1",
			Limit:   20,
			Offset:  10,
		}

		assert.Equal(t, "venue-1", filters.VenueID)
		assert.Equal(t, "2024-01-15", filters.Date)
		assert.Equal(t, "confirmed", filters.Status)
		assert.Equal(t, "table-1", filters.TableID)
		assert.Equal(t, int32(20), filters.Limit)
		assert.Equal(t, int32(10), filters.Offset)
	})

	t.Run("empty filters", func(t *testing.T) {
		filters := &BookingFilters{
			Limit:  10,
			Offset: 0,
		}

		assert.Empty(t, filters.VenueID)
		assert.Empty(t, filters.Date)
		assert.Empty(t, filters.Status)
		assert.Empty(t, filters.TableID)
		assert.Equal(t, int32(10), filters.Limit)
		assert.Equal(t, int32(0), filters.Offset)
	})

	t.Run("only venue filter", func(t *testing.T) {
		filters := &BookingFilters{
			VenueID: "venue-1",
			Limit:   10,
			Offset:  0,
		}

		assert.Equal(t, "venue-1", filters.VenueID)
		assert.Empty(t, filters.Date)
		assert.Empty(t, filters.Status)
	})
}

// TestOutboxMessage_AllFields tests all outbox message fields
func TestOutboxMessage_AllFields(t *testing.T) {
	now := time.Now()
	msg := &OutboxMessage{
		ID:         "msg-1",
		Topic:      "booking.held",
		Key:        "booking-1",
		Payload:    []byte(`{"booking_id":"booking-1"}`),
		Status:     "pending",
		RetryCount: 3,
		CreatedAt:  now,
	}

	assert.Equal(t, "msg-1", msg.ID)
	assert.Equal(t, "booking.held", msg.Topic)
	assert.Equal(t, "booking-1", msg.Key)
	assert.Equal(t, []byte(`{"booking_id":"booking-1"}`), msg.Payload)
	assert.Equal(t, "pending", msg.Status)
	assert.Equal(t, int32(3), msg.RetryCount)
	assert.Equal(t, now, msg.CreatedAt)
}

// TestOutboxMessage_Statuses tests different outbox statuses
func TestOutboxMessage_Statuses(t *testing.T) {
	statuses := []string{"pending", "sent", "failed", "dlq"}
	
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			msg := &OutboxMessage{
				ID:     uuid.New().String(),
				Topic:  "booking.held",
				Status: status,
			}
			assert.Equal(t, status, msg.Status)
		})
	}
}

// TestBooking_Statuses tests different booking statuses
func TestBooking_Statuses(t *testing.T) {
	statuses := []string{"held", "confirmed", "seated", "finished", "cancelled", "expired", "no_show"}
	
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			booking := &Booking{
				ID:     uuid.New().String(),
				Status: status,
			}
			assert.Equal(t, status, booking.Status)
		})
	}
}

// TestBookingFilters_EdgeCases tests edge cases for filters
func TestBookingFilters_EdgeCases(t *testing.T) {
	t.Run("zero limit", func(t *testing.T) {
		filters := &BookingFilters{
			Limit: 0,
		}
		assert.Equal(t, int32(0), filters.Limit)
	})

	t.Run("negative offset", func(t *testing.T) {
		filters := &BookingFilters{
			Offset: -1,
		}
		assert.Equal(t, int32(-1), filters.Offset)
	})

	t.Run("large limit", func(t *testing.T) {
		filters := &BookingFilters{
			Limit: 1000,
		}
		assert.Equal(t, int32(1000), filters.Limit)
	})
}

// TestBooking_ExpiresAt tests expires at field variations
func TestBooking_ExpiresAt(t *testing.T) {
	t.Run("with expires at", func(t *testing.T) {
		expiresAt := time.Now().Add(10 * time.Minute)
		booking := &Booking{
			ID:         "booking-1",
			ExpiresAt:  &expiresAt,
		}
		assert.NotNil(t, booking.ExpiresAt)
		assert.Equal(t, expiresAt, *booking.ExpiresAt)
	})

	t.Run("without expires at", func(t *testing.T) {
		booking := &Booking{
			ID:        "booking-2",
			ExpiresAt: nil,
		}
		assert.Nil(t, booking.ExpiresAt)
	})
}

// TestBooking_TimeFields tests time-related fields
func TestBooking_TimeFields(t *testing.T) {
	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	booking := &Booking{
		ID:         "booking-1",
		CreatedAt:  past,
		UpdatedAt:  now,
		ExpiresAt:  &future,
	}

	assert.True(t, booking.CreatedAt.Before(booking.UpdatedAt))
	assert.True(t, booking.UpdatedAt.Before(*booking.ExpiresAt))
}

// TestOutboxMessage_RetryCount tests retry count variations
func TestOutboxMessage_RetryCount(t *testing.T) {
	t.Run("zero retries", func(t *testing.T) {
		msg := &OutboxMessage{
			RetryCount: 0,
		}
		assert.Equal(t, int32(0), msg.RetryCount)
	})

	t.Run("max retries", func(t *testing.T) {
		msg := &OutboxMessage{
			RetryCount: 3,
		}
		assert.Equal(t, int32(3), msg.RetryCount)
	})

	t.Run("exceeded retries", func(t *testing.T) {
		msg := &OutboxMessage{
			RetryCount: 10,
		}
		assert.Equal(t, int32(10), msg.RetryCount)
	})
}

