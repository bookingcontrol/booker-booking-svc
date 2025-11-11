package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/bookingcontrol/booker-booking-svc/cmd/booking-svc/config"
	"github.com/bookingcontrol/booker-booking-svc/cmd/booking-svc/repository"
	"github.com/bookingcontrol/booker-booking-svc/internal/kafka"
	"github.com/bookingcontrol/booker-booking-svc/internal/redis"
	"github.com/bookingcontrol/booker-booking-svc/internal/tracing"
	commonpb "github.com/bookingcontrol/booker-contracts-go/common"
	bookingpb "github.com/bookingcontrol/booker-contracts-go/booking"
	venuepb "github.com/bookingcontrol/booker-contracts-go/venue"
)

type Service struct {
	bookingpb.UnimplementedBookingServiceServer
	repo        *repository.Repository
	producer    *kafka.Producer
	venueClient venuepb.VenueServiceClient
	redis       *redis.Client
	cfg         *config.Config
}

func New(repo *repository.Repository, producer *kafka.Producer, venueClient venuepb.VenueServiceClient, redisClient *redis.Client, cfg *config.Config) *Service {
	return &Service{
		repo:        repo,
		producer:    producer,
		venueClient: venueClient,
		redis:       redisClient,
		cfg:         cfg,
	}
}

func (s *Service) CreateBooking(ctx context.Context, req *bookingpb.CreateBookingRequest) (*bookingpb.Booking, error) {
	ctx, span := tracing.StartSpan(ctx, "CreateBooking")
	defer span.End()

	// Check idempotency
	if req.IdempotencyKey != "" {
		// TODO: Check idempotency key in Redis
	}

	// Validate slot availability with venue service
	_, err := s.venueClient.CheckAvailability(ctx, &venuepb.CheckAvailabilityRequest{
		VenueId: req.VenueId,
		Slot:    req.Slot,
		PartySize: req.PartySize,
	})
	if err != nil {
		return nil, fmt.Errorf("availability check failed: %w", err)
	}

	// Try to acquire hold in Redis
	holdKey := s.getHoldKey(req.VenueId, req.Table.TableId, req.Slot.Date, req.Slot.StartTime)
	bookingID := uuid.New().String()
	
	acquired, err := s.redis.SetHold(ctx, holdKey, bookingID, time.Duration(s.cfg.HoldTTLMinutes)*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire hold: %w", err)
	}
	if !acquired {
		return nil, fmt.Errorf("slot already held")
	}

	// Calculate end time
	endTime := s.calculateEndTime(req.Slot.StartTime, req.Slot.DurationMinutes)
	expiresAt := time.Now().Add(time.Duration(s.cfg.HoldTTLMinutes) * time.Minute)

	// Create booking in DB
	booking := &repository.Booking{
		ID:            bookingID,
		VenueID:      req.VenueId,
		TableID:      req.Table.TableId,
		Date:         req.Slot.Date,
		StartTime:    req.Slot.StartTime,
		EndTime:      endTime,
		PartySize:    req.PartySize,
		CustomerName: req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		Status:       "held",
		Comment:      req.Comment,
		AdminID:      req.AdminId,
		ExpiresAt:    &expiresAt,
	}

	if err := s.repo.CreateBooking(ctx, booking); err != nil {
		s.redis.DeleteHold(ctx, holdKey)
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	// Add event to outbox
	event := &commonpb.BookingEvent{
		BookingId: bookingID,
		Table:     req.Table,
		Slot:      req.Slot,
		PartySize: req.PartySize,
		CustomerName: req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		Payload: &commonpb.BookingEvent_Held{
			Held: &commonpb.BookingHeld{
				ExpiresAt: expiresAt.Unix(),
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.held", bookingID, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) GetBooking(ctx context.Context, req *bookingpb.GetBookingRequest) (*bookingpb.Booking, error) {
	booking, err := s.repo.GetBooking(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return s.toBookingProto(booking), nil
}

func (s *Service) ListBookings(ctx context.Context, req *bookingpb.ListBookingsRequest) (*bookingpb.ListBookingsResponse, error) {
	filters := &repository.BookingFilters{
		VenueID: req.VenueId,
		Date:    req.Date,
		Status:  req.Status,
		TableID: req.TableId,
		Limit:   req.Limit,
		Offset:  req.Offset,
	}

	bookings, total, err := s.repo.ListBookings(ctx, filters)
	if err != nil {
		return nil, err
	}

	protoBookings := make([]*bookingpb.Booking, len(bookings))
	for i, b := range bookings {
		protoBookings[i] = s.toBookingProto(b)
	}

	return &bookingpb.ListBookingsResponse{
		Bookings: protoBookings,
		Total:    total,
	}, nil
}

func (s *Service) ConfirmBooking(ctx context.Context, req *bookingpb.ConfirmBookingRequest) (*bookingpb.Booking, error) {
	ctx, span := tracing.StartSpan(ctx, "ConfirmBooking")
	defer span.End()

	booking, err := s.repo.GetBooking(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	if booking.Status != "held" {
		return nil, fmt.Errorf("booking is not in held status")
	}

	// Update status
	if err := s.repo.UpdateBookingStatus(ctx, req.Id, "confirmed"); err != nil {
		return nil, err
	}

	// Remove hold from Redis since booking is now confirmed
	holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
	s.redis.DeleteHold(ctx, holdKey)

	booking.Status = "confirmed"
	booking.ExpiresAt = nil

	// Add event to outbox
	event := &commonpb.BookingEvent{
		BookingId: req.Id,
		Payload: &commonpb.BookingEvent_Confirmed{
			Confirmed: &commonpb.BookingConfirmed{
				AdminId: req.AdminId,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.confirmed", req.Id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) CancelBooking(ctx context.Context, req *bookingpb.CancelBookingRequest) (*bookingpb.Booking, error) {
	ctx, span := tracing.StartSpan(ctx, "CancelBooking")
	defer span.End()

	booking, err := s.repo.GetBooking(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	if booking.Status == "finished" || booking.Status == "cancelled" {
		return nil, fmt.Errorf("booking cannot be cancelled")
	}

	// Update status
	if err := s.repo.UpdateBookingStatus(ctx, req.Id, "cancelled"); err != nil {
		return nil, err
	}

	// Release hold
	holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
	s.redis.DeleteHold(ctx, holdKey)

	booking.Status = "cancelled"

	// Add event to outbox
	event := &commonpb.BookingEvent{
		BookingId: req.Id,
		Payload: &commonpb.BookingEvent_Cancelled{
			Cancelled: &commonpb.BookingCancelled{
				AdminId: req.AdminId,
				Reason:  req.Reason,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.cancelled", req.Id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) MarkSeated(ctx context.Context, req *bookingpb.MarkSeatedRequest) (*bookingpb.Booking, error) {
	booking, err := s.repo.GetBooking(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateBookingStatus(ctx, req.Id, "seated"); err != nil {
		return nil, err
	}

	booking.Status = "seated"

	event := &commonpb.BookingEvent{
		BookingId: req.Id,
		Payload: &commonpb.BookingEvent_Seated{
			Seated: &commonpb.BookingSeated{
				AdminId: req.AdminId,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.seated", req.Id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) MarkFinished(ctx context.Context, req *bookingpb.MarkFinishedRequest) (*bookingpb.Booking, error) {
	booking, err := s.repo.GetBooking(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateBookingStatus(ctx, req.Id, "finished"); err != nil {
		return nil, err
	}

	// Release hold
	holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
	s.redis.DeleteHold(ctx, holdKey)

	booking.Status = "finished"

	event := &commonpb.BookingEvent{
		BookingId: req.Id,
		Payload: &commonpb.BookingEvent_Finished{
			Finished: &commonpb.BookingFinished{
				AdminId: req.AdminId,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.finished", req.Id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) MarkNoShow(ctx context.Context, req *bookingpb.MarkNoShowRequest) (*bookingpb.Booking, error) {
	booking, err := s.repo.GetBooking(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateBookingStatus(ctx, req.Id, "no_show"); err != nil {
		return nil, err
	}

	// Release hold
	holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
	s.redis.DeleteHold(ctx, holdKey)

	booking.Status = "no_show"

	event := &commonpb.BookingEvent{
		BookingId: req.Id,
		Payload: &commonpb.BookingEvent_NoShow{
			NoShow: &commonpb.BookingNoShow{
				AdminId: req.AdminId,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.no_show", req.Id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) StartOutboxWorker(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processOutbox(ctx)
		}
	}
}

func (s *Service) processOutbox(ctx context.Context) {
	messages, err := s.repo.GetPendingOutbox(ctx, 10)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get pending outbox messages")
		return
	}

	for _, msg := range messages {
		var event commonpb.BookingEvent
		// Try protojson first (new format), fallback to json (old format for backward compatibility)
		err := protojson.Unmarshal(msg.Payload, &event)
		if err != nil {
			// Try legacy json format for backward compatibility
			if jsonErr := json.Unmarshal(msg.Payload, &event); jsonErr != nil {
				log.Error().Err(err).Err(jsonErr).Str("id", msg.ID).Msg("Failed to unmarshal event (both protojson and json failed)")
				s.repo.UpdateOutboxStatus(ctx, msg.ID, "failed", msg.RetryCount+1)
				continue
			}
			// Successfully unmarshaled with json, log warning
			log.Warn().Str("id", msg.ID).Msg("Unmarshaled event using legacy json format")
		}

		if err := s.producer.PublishBookingEvent(ctx, msg.Topic, &event); err != nil {
			log.Error().Err(err).Str("id", msg.ID).Msg("Failed to publish event")
			if msg.RetryCount >= 3 {
				s.repo.UpdateOutboxStatus(ctx, msg.ID, "dlq", msg.RetryCount+1)
			} else {
				s.repo.UpdateOutboxStatus(ctx, msg.ID, "pending", msg.RetryCount+1)
			}
			continue
		}

		s.repo.UpdateOutboxStatus(ctx, msg.ID, "sent", msg.RetryCount)
	}
}

func (s *Service) StartExpiredHoldsWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processExpiredHolds(ctx)
		}
	}
}

func (s *Service) processExpiredHolds(ctx context.Context) {
	bookings, err := s.repo.GetExpiredHolds(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get expired holds")
		return
	}

	for _, booking := range bookings {
		// Update status
		if err := s.repo.UpdateBookingStatus(ctx, booking.ID, "expired"); err != nil {
			log.Error().Err(err).Str("booking_id", booking.ID).Msg("Failed to update expired booking")
			continue
		}

		// Release hold
		holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
		s.redis.DeleteHold(ctx, holdKey)

		// Add event to outbox
		event := &commonpb.BookingEvent{
			BookingId: booking.ID,
			Payload: &commonpb.BookingEvent_Expired{
				Expired: &commonpb.BookingExpired{
					Reason: "Hold expired",
				},
			},
		}

		if err := s.addToOutbox(ctx, "booking.expired", booking.ID, event); err != nil {
			log.Error().Err(err).Msg("Failed to add to outbox")
		}
	}
}

func (s *Service) addToOutbox(ctx context.Context, topic, key string, event *commonpb.BookingEvent) error {
	data, err := protojson.Marshal(event)
	if err != nil {
		return err
	}
	return s.repo.AddToOutbox(ctx, topic, key, data)
}

func (s *Service) getHoldKey(venueID, tableID, date, startTime string) string {
	return fmt.Sprintf("hold:%s:%s:%s:%s", venueID, tableID, date, startTime)
}

func (s *Service) CheckTableAvailability(ctx context.Context, req *bookingpb.CheckTableAvailabilityRequest) (*bookingpb.CheckTableAvailabilityResponse, error) {
	ctx, span := tracing.StartSpan(ctx, "CheckTableAvailability")
	defer span.End()

	// Calculate end time (default to 120 minutes if not specified)
	durationMinutes := req.Slot.DurationMinutes
	if durationMinutes == 0 {
		durationMinutes = 120
	}
	endTime := s.calculateEndTime(req.Slot.StartTime, durationMinutes)

	// Check availability
	availability, err := s.repo.CheckTableAvailability(ctx, req.VenueId, req.TableIds, req.Slot.Date, req.Slot.StartTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to check table availability: %w", err)
	}

	// Build response
	result := make([]*bookingpb.TableAvailabilityInfo, 0, len(req.TableIds))
	for _, tableID := range req.TableIds {
		available, ok := availability[tableID]
		if !ok {
			available = false
		}
		
		reason := ""
		if !available {
			reason = "Table is already booked for this time slot"
		}

		result = append(result, &bookingpb.TableAvailabilityInfo{
			TableId:   tableID,
			Available: available,
			Reason:    reason,
		})
	}

	return &bookingpb.CheckTableAvailabilityResponse{
		Tables: result,
	}, nil
}

func (s *Service) calculateEndTime(startTime string, durationMinutes int32) string {
	// Parse start time (HH:MM format)
	start, err := time.Parse("15:04", startTime)
	if err != nil {
		log.Warn().Err(err).Str("start_time", startTime).Msg("Failed to parse start time, using default")
		// Default: add 2 hours
		start, _ = time.Parse("15:04", "12:00")
		durationMinutes = 120
	}
	
	// Add duration
	end := start.Add(time.Duration(durationMinutes) * time.Minute)
	return end.Format("15:04")
}

func (s *Service) toBookingProto(b *repository.Booking) *bookingpb.Booking {
	var expiresAt int64
	if b.ExpiresAt != nil {
		expiresAt = b.ExpiresAt.Unix()
	}

	return &bookingpb.Booking{
		Id:            b.ID,
		VenueId:      b.VenueID,
		Table:        &commonpb.TableRef{TableId: b.TableID},
		Slot:         &commonpb.Slot{Date: b.Date, StartTime: b.StartTime},
		PartySize:    b.PartySize,
		CustomerName: b.CustomerName,
		CustomerPhone: b.CustomerPhone,
		Status:       b.Status,
		Comment:      b.Comment,
		AdminId:      b.AdminID,
		CreatedAt:    b.CreatedAt.Unix(),
		UpdatedAt:    b.UpdatedAt.Unix(),
		ExpiresAt:    expiresAt,
	}
}

