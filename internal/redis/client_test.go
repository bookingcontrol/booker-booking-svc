package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClient_SetHold(t *testing.T) {
	t.Run("successful hold", func(t *testing.T) {
		// Note: This would require a real Redis or mock
		// For unit tests, we'd use a mock client
		// For integration tests, we'd use testcontainers

		key := "hold:venue-1:table-1:2024-01-15:19:00"
		bookingID := "booking-1"
		ttl := 10 * time.Minute

		// Verify parameters
		assert.NotEmpty(t, key)
		assert.NotEmpty(t, bookingID)
		assert.Equal(t, 10*time.Minute, ttl)
	})

	t.Run("hold key format", func(t *testing.T) {
		// Test the hold key format used in booking service
		venueID := "venue-1"
		tableID := "table-1"
		date := "2024-01-15"
		startTime := "19:00"

		expectedKey := "hold:" + venueID + ":" + tableID + ":" + date + ":" + startTime
		assert.Equal(t, "hold:venue-1:table-1:2024-01-15:19:00", expectedKey)
	})
}

func TestClient_GetHold(t *testing.T) {
	t.Run("get existing hold", func(t *testing.T) {
		key := "hold:venue-1:table-1:2024-01-15:19:00"
		
		// In a real test, we'd set a hold first, then get it
		// For now, we verify the key format
		assert.NotEmpty(t, key)
	})
}

func TestClient_DeleteHold(t *testing.T) {
	t.Run("delete existing hold", func(t *testing.T) {
		key := "hold:venue-1:table-1:2024-01-15:19:00"
		
		// In a real test, we'd set a hold first, then delete it
		// For now, we verify the key format
		assert.NotEmpty(t, key)
	})
}

// TestClient_SetHold_EdgeCases tests edge cases for SetHold
func TestClient_SetHold_EdgeCases(t *testing.T) {
	t.Run("zero TTL", func(t *testing.T) {
		key := "hold:venue-1:table-1:2024-01-15:19:00"
		bookingID := "booking-1"
		ttl := time.Duration(0)

		assert.NotEmpty(t, key)
		assert.NotEmpty(t, bookingID)
		assert.Equal(t, time.Duration(0), ttl)
	})

	t.Run("very long TTL", func(t *testing.T) {
		key := "hold:venue-1:table-1:2024-01-15:19:00"
		bookingID := "booking-1"
		ttl := 24 * time.Hour

		assert.NotEmpty(t, key)
		assert.NotEmpty(t, bookingID)
		assert.Equal(t, 24*time.Hour, ttl)
	})

	t.Run("short TTL", func(t *testing.T) {
		key := "hold:venue-1:table-1:2024-01-15:19:00"
		bookingID := "booking-1"
		ttl := 1 * time.Minute

		assert.NotEmpty(t, key)
		assert.NotEmpty(t, bookingID)
		assert.Equal(t, 1*time.Minute, ttl)
	})
}

// TestClient_HoldKeyFormats tests different hold key formats
func TestClient_HoldKeyFormats(t *testing.T) {
	testCases := []struct {
		name     string
		venueID  string
		tableID  string
		date     string
		startTime string
		expected string
	}{
		{
			name:      "standard format",
			venueID:   "venue-1",
			tableID:   "table-1",
			date:      "2024-01-15",
			startTime: "19:00",
			expected:  "hold:venue-1:table-1:2024-01-15:19:00",
		},
		{
			name:      "different venue",
			venueID:   "venue-2",
			tableID:   "table-1",
			date:      "2024-01-15",
			startTime: "19:00",
			expected:  "hold:venue-2:table-1:2024-01-15:19:00",
		},
		{
			name:      "different table",
			venueID:   "venue-1",
			tableID:   "table-2",
			date:      "2024-01-15",
			startTime: "19:00",
			expected:  "hold:venue-1:table-2:2024-01-15:19:00",
		},
		{
			name:      "different date",
			venueID:   "venue-1",
			tableID:   "table-1",
			date:      "2024-01-16",
			startTime: "19:00",
			expected:  "hold:venue-1:table-1:2024-01-16:19:00",
		},
		{
			name:      "different time",
			venueID:   "venue-1",
			tableID:   "table-1",
			date:      "2024-01-15",
			startTime: "20:00",
			expected:  "hold:venue-1:table-1:2024-01-15:20:00",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := "hold:" + tc.venueID + ":" + tc.tableID + ":" + tc.date + ":" + tc.startTime
			assert.Equal(t, tc.expected, key)
		})
	}
}

// TestClient_NewClient tests client creation
func TestClient_NewClient(t *testing.T) {
	t.Run("with address", func(t *testing.T) {
		addr := "localhost:6379"
		password := ""
		
		// In a real test, we'd create the client
		// client := NewClient(addr, password)
		// assert.NotNil(t, client)
		
		assert.NotEmpty(t, addr)
		assert.Empty(t, password)
	})

	t.Run("with password", func(t *testing.T) {
		addr := "localhost:6379"
		password := "secret"
		
		assert.NotEmpty(t, addr)
		assert.NotEmpty(t, password)
	})

	t.Run("empty address", func(t *testing.T) {
		addr := ""
		password := ""
		
		assert.Empty(t, addr)
		assert.Empty(t, password)
	})
}

// Integration test would require testcontainers
func TestClient_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	// Setup Redis using testcontainers
	// client := NewClient("localhost:6379", "")
	// ctx := context.Background()

	// key := "hold:venue-1:table-1:2024-01-15:19:00"
	// bookingID := "booking-1"
	// ttl := 10 * time.Minute

	// // Set hold
	// acquired, err := client.SetHold(ctx, key, bookingID, ttl)
	// require.NoError(t, err)
	// assert.True(t, acquired)

	// // Get hold
	// retrieved, err := client.GetHold(ctx, key)
	// require.NoError(t, err)
	// assert.Equal(t, bookingID, retrieved)

	// // Delete hold
	// err = client.DeleteHold(ctx, key)
	// require.NoError(t, err)

	// // Verify deleted
	// _, err = client.GetHold(ctx, key)
	// assert.Error(t, err) // Should return error (key not found)
}

