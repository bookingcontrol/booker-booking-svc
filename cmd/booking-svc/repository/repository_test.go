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

