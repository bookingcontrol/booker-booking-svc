package booking

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/bookingcontrol/booker-booking-svc/internal/config"
	dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
	bookingpb "github.com/bookingcontrol/booker-contracts-go/booking"
	commonpb "github.com/bookingcontrol/booker-contracts-go/common"
	venuepb "github.com/bookingcontrol/booker-contracts-go/venue"
)

type mockRepository struct {
	mock.Mock
}

func (m *mockRepository) CreateBooking(ctx context.Context, booking *dom.Booking) error {
	args := m.Called(ctx, booking)
	return args.Error(0)
}

func (m *mockRepository) GetBooking(ctx context.Context, id string) (*dom.Booking, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dom.Booking), args.Error(1)
}

func (m *mockRepository) ListBookings(ctx context.Context, filters *dom.BookingFilters) ([]*dom.Booking, int32, error) {
	args := m.Called(mock.Anything, filters)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	total := args.Get(1).(int32)
	return args.Get(0).([]*dom.Booking), total, args.Error(2)
}

func (m *mockRepository) UpdateBookingStatus(ctx context.Context, id, status string) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *mockRepository) CheckTableAvailability(ctx context.Context, venueID string, tableIDs []string, date, startTime, endTime string) (map[string]bool, error) {
	args := m.Called(ctx, venueID, tableIDs, date, startTime, endTime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]bool), args.Error(1)
}

func (m *mockRepository) GetExpiredHolds(ctx context.Context) ([]*dom.Booking, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dom.Booking), args.Error(1)
}

func (m *mockRepository) AddToOutbox(ctx context.Context, topic, key string, payload []byte) error {
	args := m.Called(ctx, topic, key, payload)
	return args.Error(0)
}

func (m *mockRepository) GetPendingOutbox(ctx context.Context, limit int32) ([]*dom.OutboxMessage, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dom.OutboxMessage), args.Error(1)
}

func (m *mockRepository) UpdateOutboxStatus(ctx context.Context, id, status string, retryCount int32) error {
	args := m.Called(ctx, id, status, retryCount)
	return args.Error(0)
}

type mockHoldRepository struct {
	mock.Mock
}

func (m *mockHoldRepository) SetHold(ctx context.Context, key string, bookingID string, ttl time.Duration) (bool, error) {
	args := m.Called(ctx, key, bookingID, ttl)
	return args.Bool(0), args.Error(1)
}

func (m *mockHoldRepository) GetHold(ctx context.Context, key string) (string, error) {
	args := m.Called(ctx, key)
	return args.String(0), args.Error(1)
}

func (m *mockHoldRepository) DeleteHold(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

type mockEventRepository struct {
	mock.Mock
}

func (m *mockEventRepository) AddToOutbox(ctx context.Context, topic, key string, payload []byte) error {
	args := m.Called(ctx, topic, key, payload)
	return args.Error(0)
}

func (m *mockEventRepository) GetPendingOutbox(ctx context.Context, limit int32) ([]*dom.OutboxMessage, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dom.OutboxMessage), args.Error(1)
}

func (m *mockEventRepository) UpdateOutboxStatus(ctx context.Context, id, status string, retryCount int32) error {
	args := m.Called(ctx, id, status, retryCount)
	return args.Error(0)
}

type mockVenueClient struct {
	mock.Mock
}

func (m *mockVenueClient) CheckAvailability(ctx context.Context, in *venuepb.CheckAvailabilityRequest, opts ...grpc.CallOption) (*venuepb.CheckAvailabilityResponse, error) {
	args := m.Called(ctx, in)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*venuepb.CheckAvailabilityResponse), args.Error(1)
}

type mockEventPublisher struct {
	mock.Mock
}

func (m *mockEventPublisher) PublishBookingEvent(ctx context.Context, topic string, event *commonpb.BookingEvent) error {
	args := m.Called(ctx, topic, event)
	return args.Error(0)
}

func TestService_CreateBooking_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{HoldTTLMinutes: 5}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	req := &bookingpb.CreateBookingRequest{
		VenueId: "venue-1",
		Table: &commonpb.TableRef{
			TableId: "table-1",
		},
		Slot: &commonpb.Slot{
			Date:      "2025-01-01",
			StartTime: "18:00",
		},
		PartySize:     2,
		CustomerName:  "John Doe",
		CustomerPhone: "+79990000000",
		AdminId:       "admin-1",
	}

	venueClient.
		On("CheckAvailability", mock.Anything, mock.AnythingOfType("*venue.CheckAvailabilityRequest")).
		Return(&venuepb.CheckAvailabilityResponse{}, nil)

	holdRepo.
		On("SetHold", mock.Anything, "hold:venue-1:table-1:2025-01-01:18:00", mock.AnythingOfType("string"),
			mock.MatchedBy(func(d time.Duration) bool { return d == 5*time.Minute })).
		Return(true, nil)

	var createdBookingID string
	repo.
		On("CreateBooking", mock.Anything, mock.MatchedBy(func(b *dom.Booking) bool {
			createdBookingID = b.ID
			return b.VenueID == "venue-1" && b.TableID == "table-1" && b.Status == "held"
		})).
		Return(nil)

	eventRepo.
		On("AddToOutbox", mock.Anything, "booking.held", mock.AnythingOfType("string"), mock.AnythingOfType("[]uint8")).
		Return(nil).
		Run(func(args mock.Arguments) {
			eventData := args.Get(3).([]byte)
			var event commonpb.BookingEvent
			err := protojson.Unmarshal(eventData, &event)
			require.NoError(t, err)
			assert.Equal(t, createdBookingID, event.BookingId)
			require.NotNil(t, event.GetHeld())
			assert.Greater(t, event.GetHeld().GetExpiresAt(), int64(0))
		})

	resp, err := svc.CreateBooking(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "venue-1", resp.VenueId)
	assert.Equal(t, "held", resp.Status)
	assert.NotEmpty(t, resp.Id)

	venueClient.AssertExpectations(t)
	holdRepo.AssertExpectations(t)
	repo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
	publisher.AssertNotCalled(t, "PublishBookingEvent", mock.Anything, mock.Anything, mock.Anything)
}

func TestService_CreateBooking_SlotAlreadyHeld(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{HoldTTLMinutes: 5}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	req := &bookingpb.CreateBookingRequest{
		VenueId:   "venue-1",
		Table:     &commonpb.TableRef{TableId: "table-1"},
		Slot:      &commonpb.Slot{Date: "2025-01-01", StartTime: "18:00"},
		PartySize: 2,
		AdminId:   "admin-1",
	}

	venueClient.
		On("CheckAvailability", mock.Anything, mock.AnythingOfType("*venue.CheckAvailabilityRequest")).
		Return(&venuepb.CheckAvailabilityResponse{}, nil)

	holdRepo.
		On("SetHold", mock.Anything, "hold:venue-1:table-1:2025-01-01:18:00", mock.AnythingOfType("string"),
			mock.Anything).
		Return(false, nil)

	holdRepo.
		On("GetHold", mock.Anything, "hold:venue-1:table-1:2025-01-01:18:00").
		Return("existing-booking", nil)

	repo.
		On("GetBooking", mock.Anything, "existing-booking").
		Return(&dom.Booking{ID: "existing-booking", Status: "confirmed"}, nil)

	resp, err := svc.CreateBooking(ctx, req)

	require.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "slot already held")

	eventRepo.AssertNotCalled(t, "AddToOutbox", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestService_CheckTableAvailability(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	repo.
		On("CheckTableAvailability", mock.Anything, "venue-1", []string{"table-1", "table-2"}, "2025-01-01", "18:00", "20:00").
		Return(map[string]bool{"table-1": true, "table-2": false}, nil)

	req := &bookingpb.CheckTableAvailabilityRequest{
		VenueId:  "venue-1",
		TableIds: []string{"table-1", "table-2"},
		Slot: &commonpb.Slot{
			Date:            "2025-01-01",
			StartTime:       "18:00",
			DurationMinutes: 120,
		},
	}

	resp, err := svc.CheckTableAvailability(ctx, req)

	require.NoError(t, err)
	require.Len(t, resp.Tables, 2)
	assert.True(t, resp.Tables[0].Available)
	assert.False(t, resp.Tables[1].Available)
	assert.Equal(t, "Table is already booked for this time slot", resp.Tables[1].Reason)
}

func TestService_ProcessOutbox_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	eventPayload, err := protojson.Marshal(&commonpb.BookingEvent{
		BookingId: "booking-1",
		Payload: &commonpb.BookingEvent_Confirmed{
			Confirmed: &commonpb.BookingConfirmed{AdminId: "admin-1"},
		},
	})
	require.NoError(t, err)

	msg := &dom.OutboxMessage{
		ID:         "msg-1",
		Topic:      "booking.confirmed",
		Key:        "booking-1",
		Payload:    eventPayload,
		RetryCount: 0,
	}

	eventRepo.
		On("GetPendingOutbox", mock.Anything, int32(10)).
		Return([]*dom.OutboxMessage{msg}, nil)

	publisher.
		On("PublishBookingEvent", mock.Anything, "booking.confirmed", mock.AnythingOfType("*common.BookingEvent")).
		Return(nil).
		Run(func(args mock.Arguments) {
			event := args.Get(2).(*commonpb.BookingEvent)
			assert.Equal(t, "booking-1", event.BookingId)
		})

	eventRepo.
		On("UpdateOutboxStatus", mock.Anything, "msg-1", "sent", int32(0)).
		Return(nil)

	svc.processOutbox(ctx)

	eventRepo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestService_ProcessOutbox_PublishError(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	eventPayload, err := protojson.Marshal(&commonpb.BookingEvent{
		BookingId: "booking-1",
	})
	require.NoError(t, err)

	msg := &dom.OutboxMessage{
		ID:         "msg-1",
		Topic:      "booking.confirmed",
		Key:        "booking-1",
		Payload:    eventPayload,
		RetryCount: 1,
	}

	eventRepo.
		On("GetPendingOutbox", mock.Anything, int32(10)).
		Return([]*dom.OutboxMessage{msg}, nil)

	publisher.
		On("PublishBookingEvent", mock.Anything, "booking.confirmed", mock.AnythingOfType("*common.BookingEvent")).
		Return(errors.New("kafka down"))

	eventRepo.
		On("UpdateOutboxStatus", mock.Anything, "msg-1", "pending", int32(2)).
		Return(nil)

	svc.processOutbox(ctx)

	eventRepo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestService_ProcessExpiredHolds(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	expiredAt := time.Now().Add(-time.Minute)
	bookingRecord := &dom.Booking{
		ID:        "booking-1",
		VenueID:   "venue-1",
		TableID:   "table-1",
		Date:      "2025-01-01",
		StartTime: "18:00",
		Status:    "held",
		ExpiresAt: &expiredAt,
	}

	repo.
		On("GetExpiredHolds", mock.Anything).
		Return([]*dom.Booking{bookingRecord}, nil)

	repo.
		On("UpdateBookingStatus", mock.Anything, "booking-1", "expired").
		Return(nil)

	holdRepo.
		On("DeleteHold", mock.Anything, "hold:venue-1:table-1:2025-01-01:18:00").
		Return(nil)

	eventRepo.
		On("AddToOutbox", mock.Anything, "booking.expired", "booking-1", mock.AnythingOfType("[]uint8")).
		Return(nil).
		Run(func(args mock.Arguments) {
			eventBytes := args.Get(3).([]byte)
			var event commonpb.BookingEvent
			err := protojson.Unmarshal(eventBytes, &event)
			require.NoError(t, err)
			assert.Equal(t, "booking-1", event.BookingId)
			assert.NotNil(t, event.GetExpired())
		})

	svc.processExpiredHolds(ctx)

	repo.AssertExpectations(t)
	holdRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

func TestService_ConfirmBooking_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	booking := &dom.Booking{
		ID:        "booking-1",
		VenueID:   "venue-1",
		TableID:   "table-1",
		Date:      "2025-01-01",
		StartTime: "18:00",
		Status:    "held",
	}

	repo.
		On("GetBooking", mock.Anything, "booking-1").
		Return(booking, nil)

	repo.
		On("UpdateBookingStatus", mock.Anything, "booking-1", "confirmed").
		Return(nil)

	holdRepo.
		On("DeleteHold", mock.Anything, "hold:venue-1:table-1:2025-01-01:18:00").
		Return(nil)

	eventRepo.
		On("AddToOutbox", mock.Anything, "booking.confirmed", "booking-1", mock.AnythingOfType("[]uint8")).
		Return(nil)

	resp, err := svc.ConfirmBooking(ctx, "booking-1", "admin-1")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "confirmed", resp.Status)
	repo.AssertExpectations(t)
	holdRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

func TestService_ConfirmBooking_NotHeldStatus(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	booking := &dom.Booking{
		ID:      "booking-1",
		Status:  "confirmed",
		VenueID: "venue-1",
	}

	repo.
		On("GetBooking", mock.Anything, "booking-1").
		Return(booking, nil)

	resp, err := svc.ConfirmBooking(ctx, "booking-1", "admin-1")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "not in held status")
}

func TestService_CancelBooking_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	booking := &dom.Booking{
		ID:        "booking-1",
		VenueID:   "venue-1",
		TableID:   "table-1",
		Date:      "2025-01-01",
		StartTime: "18:00",
		Status:    "held",
	}

	repo.
		On("GetBooking", mock.Anything, "booking-1").
		Return(booking, nil)

	repo.
		On("UpdateBookingStatus", mock.Anything, "booking-1", "cancelled").
		Return(nil)

	holdRepo.
		On("DeleteHold", mock.Anything, "hold:venue-1:table-1:2025-01-01:18:00").
		Return(nil)

	eventRepo.
		On("AddToOutbox", mock.Anything, "booking.cancelled", "booking-1", mock.AnythingOfType("[]uint8")).
		Return(nil)

	resp, err := svc.CancelBooking(ctx, "booking-1", "admin-1", "Customer request")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "cancelled", resp.Status)
	repo.AssertExpectations(t)
	holdRepo.AssertExpectations(t)
	eventRepo.AssertExpectations(t)
}

func TestService_CancelBooking_CannotCancelFinished(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	booking := &dom.Booking{
		ID:     "booking-1",
		Status: "finished",
	}

	repo.
		On("GetBooking", mock.Anything, "booking-1").
		Return(booking, nil)

	resp, err := svc.CancelBooking(ctx, "booking-1", "admin-1", "Cannot cancel")

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "cannot be cancelled")
}

func TestService_MarkSeated_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	booking := &dom.Booking{
		ID:      "booking-1",
		VenueID: "venue-1",
		Status:  "confirmed",
	}

	repo.
		On("GetBooking", mock.Anything, "booking-1").
		Return(booking, nil)

	repo.
		On("UpdateBookingStatus", mock.Anything, "booking-1", "seated").
		Return(nil)

	eventRepo.
		On("AddToOutbox", mock.Anything, "booking.seated", "booking-1", mock.AnythingOfType("[]uint8")).
		Return(nil)

	resp, err := svc.MarkSeated(ctx, "booking-1", "admin-1")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "seated", resp.Status)
}

func TestService_MarkFinished_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	booking := &dom.Booking{
		ID:        "booking-1",
		VenueID:   "venue-1",
		TableID:   "table-1",
		Date:      "2025-01-01",
		StartTime: "18:00",
		Status:    "seated",
	}

	repo.
		On("GetBooking", mock.Anything, "booking-1").
		Return(booking, nil)

	repo.
		On("UpdateBookingStatus", mock.Anything, "booking-1", "finished").
		Return(nil)

	holdRepo.
		On("DeleteHold", mock.Anything, "hold:venue-1:table-1:2025-01-01:18:00").
		Return(nil)

	eventRepo.
		On("AddToOutbox", mock.Anything, "booking.finished", "booking-1", mock.AnythingOfType("[]uint8")).
		Return(nil)

	resp, err := svc.MarkFinished(ctx, "booking-1", "admin-1")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "finished", resp.Status)
}

func TestService_MarkNoShow_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	booking := &dom.Booking{
		ID:        "booking-1",
		VenueID:   "venue-1",
		TableID:   "table-1",
		Date:      "2025-01-01",
		StartTime: "18:00",
		Status:    "confirmed",
	}

	repo.
		On("GetBooking", mock.Anything, "booking-1").
		Return(booking, nil)

	repo.
		On("UpdateBookingStatus", mock.Anything, "booking-1", "no_show").
		Return(nil)

	holdRepo.
		On("DeleteHold", mock.Anything, "hold:venue-1:table-1:2025-01-01:18:00").
		Return(nil)

	eventRepo.
		On("AddToOutbox", mock.Anything, "booking.no_show", "booking-1", mock.AnythingOfType("[]uint8")).
		Return(nil)

	resp, err := svc.MarkNoShow(ctx, "booking-1", "admin-1")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "no_show", resp.Status)
}

func TestService_ProcessOutbox_RetryLogic(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	eventPayload, _ := protojson.Marshal(&commonpb.BookingEvent{
		BookingId: "booking-1",
	})

	msg := &dom.OutboxMessage{
		ID:         "msg-1",
		Topic:      "booking.confirmed",
		Payload:    eventPayload,
		RetryCount: 3, // max retries reached
	}

	eventRepo.
		On("GetPendingOutbox", mock.Anything, int32(10)).
		Return([]*dom.OutboxMessage{msg}, nil)

	publisher.
		On("PublishBookingEvent", mock.Anything, "booking.confirmed", mock.AnythingOfType("*common.BookingEvent")).
		Return(errors.New("kafka down"))

	eventRepo.
		On("UpdateOutboxStatus", mock.Anything, "msg-1", "dlq", int32(4)).
		Return(nil)

	svc.processOutbox(ctx)

	eventRepo.AssertExpectations(t)
	publisher.AssertExpectations(t)
}

func TestService_GetBooking_Success(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	booking := &dom.Booking{
		ID:      "booking-1",
		VenueID: "venue-1",
		Status:  "confirmed",
	}

	repo.
		On("GetBooking", mock.Anything, "booking-1").
		Return(booking, nil)

	resp, err := svc.GetBooking(ctx, "booking-1")

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "booking-1", resp.Id)
	assert.Equal(t, "confirmed", resp.Status)
}

func TestService_ListBookings_WithFilters(t *testing.T) {
	ctx := context.Background()
	cfg := &config.Config{}

	repo := new(mockRepository)
	holdRepo := new(mockHoldRepository)
	eventRepo := new(mockEventRepository)
	venueClient := new(mockVenueClient)
	publisher := new(mockEventPublisher)

	svc := NewService(repo, holdRepo, eventRepo, venueClient, publisher, cfg)

	bookings := []*dom.Booking{
		{ID: "booking-1", VenueID: "venue-1", Status: "confirmed"},
		{ID: "booking-2", VenueID: "venue-1", Status: "confirmed"},
	}

	repo.
		On("ListBookings", mock.Anything, mock.MatchedBy(func(f *dom.BookingFilters) bool {
			return f.VenueID == "venue-1" && f.Status == "confirmed"
		})).
		Return(bookings, int32(2), nil)

	req := &bookingpb.ListBookingsRequest{
		VenueId: "venue-1",
		Status:  "confirmed",
		Limit:   10,
		Offset:  0,
	}

	resp, err := svc.ListBookings(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(2), resp.Total)
	assert.Len(t, resp.Bookings, 2)
}
