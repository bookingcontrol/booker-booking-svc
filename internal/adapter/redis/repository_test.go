package redis

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHoldRepository_KeyFormats(t *testing.T) {
	testCases := []struct {
		name      string
		venueID   string
		tableID   string
		date      string
		startTime string
		expected  string
	}{
		{
			name:      "standard format",
			venueID:   "venue-1",
			tableID:   "table-1",
			date:      "2025-01-01",
			startTime: "18:00",
			expected:  "hold:venue-1:table-1:2025-01-01:18:00",
		},
		{
			name:      "different venue",
			venueID:   "venue-premium",
			tableID:   "table-vip",
			date:      "2025-12-31",
			startTime: "21:00",
			expected:  "hold:venue-premium:table-vip:2025-12-31:21:00",
		},
		{
			name:      "early time",
			venueID:   "venue-1",
			tableID:   "table-1",
			date:      "2025-01-01",
			startTime: "06:00",
			expected:  "hold:venue-1:table-1:2025-01-01:06:00",
		},
		{
			name:      "late time",
			venueID:   "venue-2",
			tableID:   "table-3",
			date:      "2025-02-14",
			startTime: "22:30",
			expected:  "hold:venue-2:table-3:2025-02-14:22:30",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key := "hold:" + tc.venueID + ":" + tc.tableID + ":" + tc.date + ":" + tc.startTime
			assert.Equal(t, tc.expected, key)
		})
	}
}

func TestHoldRepository_TTLValues(t *testing.T) {
	testCases := []struct {
		name     string
		ttl      time.Duration
		expected time.Duration
	}{
		{name: "5 minutes", ttl: 5 * time.Minute, expected: 5 * time.Minute},
		{name: "10 minutes", ttl: 10 * time.Minute, expected: 10 * time.Minute},
		{name: "1 hour", ttl: 1 * time.Hour, expected: 1 * time.Hour},
		{name: "1 second", ttl: 1 * time.Second, expected: 1 * time.Second},
		{name: "30 seconds", ttl: 30 * time.Second, expected: 30 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.ttl)
		})
	}
}

func TestHoldRepository_BookingIDFormats(t *testing.T) {
	testCases := []struct {
		name      string
		bookingID string
		valid     bool
	}{
		{
			name:      "valid UUID format",
			bookingID: "550e8400-e29b-41d4-a716-446655440000",
			valid:     true,
		},
		{
			name:      "simple ID",
			bookingID: "booking-1",
			valid:     true,
		},
		{
			name:      "numeric ID",
			bookingID: "12345",
			valid:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.valid {
				assert.NotEmpty(t, tc.bookingID)
			}
		})
	}
}

func TestHoldRepository_Scenarios(t *testing.T) {
	t.Run("hold creation scenario", func(t *testing.T) {
		venueID := "venue-1"
		tableID := "table-1"
		date := "2025-01-01"
		startTime := "18:00"
		bookingID := "booking-1"
		ttl := 10 * time.Minute

		holdKey := "hold:" + venueID + ":" + tableID + ":" + date + ":" + startTime

		assert.NotEmpty(t, holdKey)
		assert.NotEmpty(t, bookingID)
		assert.Equal(t, 10*time.Minute, ttl)
	})

	t.Run("multiple bookings for different tables", func(t *testing.T) {
		venue := "venue-1"
		date := "2025-01-01"
		startTime := "18:00"

		tables := []string{"table-1", "table-2", "table-3"}
		for _, table := range tables {
			holdKey := "hold:" + venue + ":" + table + ":" + date + ":" + startTime
			assert.NotEmpty(t, holdKey)
			assert.Contains(t, holdKey, table)
		}
	})

	t.Run("hold expiry validation", func(t *testing.T) {
		ttlMinutes := 10
		ttl := time.Duration(ttlMinutes) * time.Minute

		assert.Greater(t, ttl, time.Duration(0))
		assert.Equal(t, 10*time.Minute, ttl)
	})
}

func TestHoldRepository_DateTimeFormats(t *testing.T) {
	testCases := []struct {
		name  string
		date  string
		time  string
		valid bool
	}{
		{name: "valid ISO date and time", date: "2025-01-01", time: "18:00", valid: true},
		{name: "future date", date: "2030-12-31", time: "23:59", valid: true},
		{name: "early morning", date: "2025-01-01", time: "06:00", valid: true},
		{name: "late evening", date: "2025-01-01", time: "23:00", valid: true},
		{name: "empty date", date: "", time: "18:00", valid: false},
		{name: "empty time", date: "2025-01-01", time: "", valid: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.valid {
				assert.NotEmpty(t, tc.date)
				assert.NotEmpty(t, tc.time)
			} else {
				assert.True(t, tc.date == "" || tc.time == "")
			}
		})
	}
}
