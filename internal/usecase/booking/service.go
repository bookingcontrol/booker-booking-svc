package booking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/bookingcontrol/booker-booking-svc/internal/config"
	dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
	"github.com/bookingcontrol/booker-booking-svc/internal/infrastructure/tracing"
	bookingpb "github.com/bookingcontrol/booker-contracts-go/booking"
	commonpb "github.com/bookingcontrol/booker-contracts-go/common"
	venuepb "github.com/bookingcontrol/booker-contracts-go/venue"
)

// EventPublisher abstracts publishing booking events to Kafka (or any other transport).
type EventPublisher interface {
	PublishBookingEvent(ctx context.Context, topic string, event *commonpb.BookingEvent) error
}

// VenueAvailabilityClient represents the subset of venue client functionality used by the service.
type VenueAvailabilityClient interface {
	CheckAvailability(ctx context.Context, in *venuepb.CheckAvailabilityRequest, opts ...grpc.CallOption) (*venuepb.CheckAvailabilityResponse, error)
}

type Service struct {
	repo          dom.Repository
	holdRepo      dom.HoldRepository
	eventRepo     dom.EventRepository
	venueClient   VenueAvailabilityClient
	kafkaProducer EventPublisher
	cfg           *config.Config
}

func NewService(
	repo dom.Repository,
	holdRepo dom.HoldRepository,
	eventRepo dom.EventRepository,
	venueClient VenueAvailabilityClient,
	kafkaProducer EventPublisher,
	cfg *config.Config,
) *Service {
	return &Service{
		repo:          repo,
		holdRepo:      holdRepo,
		eventRepo:     eventRepo,
		venueClient:   venueClient,
		kafkaProducer: kafkaProducer,
		cfg:           cfg,
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
		VenueId:   req.VenueId,
		Slot:      req.Slot,
		PartySize: req.PartySize,
	})
	if err != nil {
		return nil, fmt.Errorf("availability check failed: %w", err)
	}

	// Try to acquire hold in Redis
	holdKey := s.getHoldKey(req.VenueId, req.Table.TableId, req.Slot.Date, req.Slot.StartTime)
	bookingID := uuid.New().String()

	acquired, err := s.holdRepo.SetHold(ctx, holdKey, bookingID, time.Duration(s.cfg.HoldTTLMinutes)*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire hold: %w", err)
	}
	if !acquired {
		// Hold exists - check if there's an active booking for this slot
		existingBookingID, err := s.holdRepo.GetHold(ctx, holdKey)
		if err != nil {
			// Can't get hold info - might be a race condition or key expired
			// Try to acquire again in case it was deleted
			acquired, err = s.holdRepo.SetHold(ctx, holdKey, bookingID, time.Duration(s.cfg.HoldTTLMinutes)*time.Minute)
			if err != nil {
				return nil, fmt.Errorf("failed to acquire hold: %w", err)
			}
			if !acquired {
				return nil, fmt.Errorf("slot already held")
			}
		} else if existingBookingID != "" {
			// Check if the existing booking is still active
			existingBooking, err := s.repo.GetBooking(ctx, existingBookingID)
			if err == nil && existingBooking != nil {
				// Only block if booking is in an active state
				activeStatuses := map[string]bool{
					"held":      true,
					"confirmed": true,
					"seated":    true,
				}
				if activeStatuses[existingBooking.Status] {
					return nil, fmt.Errorf("slot already held")
				}
				// Booking is cancelled/expired/finished - clear the stale hold
				log.Warn().
					Str("booking_id", existingBookingID).
					Str("status", existingBooking.Status).
					Msg("Clearing stale hold for inactive booking")
				s.holdRepo.DeleteHold(ctx, holdKey)
				// Retry acquiring hold
				acquired, err = s.holdRepo.SetHold(ctx, holdKey, bookingID, time.Duration(s.cfg.HoldTTLMinutes)*time.Minute)
				if err != nil {
					return nil, fmt.Errorf("failed to acquire hold after clearing stale hold: %w", err)
				}
				if !acquired {
					return nil, fmt.Errorf("slot already held")
				}
			} else {
				// Booking not found or error - clear the stale hold
				log.Warn().
					Str("booking_id", existingBookingID).
					Msg("Clearing stale hold - booking not found")
				s.holdRepo.DeleteHold(ctx, holdKey)
				// Retry acquiring hold
				acquired, err = s.holdRepo.SetHold(ctx, holdKey, bookingID, time.Duration(s.cfg.HoldTTLMinutes)*time.Minute)
				if err != nil {
					return nil, fmt.Errorf("failed to acquire hold after clearing stale hold: %w", err)
				}
				if !acquired {
					return nil, fmt.Errorf("slot already held")
				}
			}
		} else {
			// Can't get hold info - might be a race condition, return error
			return nil, fmt.Errorf("slot already held")
		}
	}

	// Calculate end time
	endTime := s.calculateEndTime(req.Slot.StartTime, req.Slot.DurationMinutes)
	expiresAt := time.Now().Add(time.Duration(s.cfg.HoldTTLMinutes) * time.Minute)

	// Create booking in DB
	booking := &dom.Booking{
		ID:            bookingID,
		VenueID:       req.VenueId,
		TableID:       req.Table.TableId,
		Date:          req.Slot.Date,
		StartTime:     req.Slot.StartTime,
		EndTime:       endTime,
		PartySize:     req.PartySize,
		CustomerName:  req.CustomerName,
		CustomerPhone: req.CustomerPhone,
		Status:        "held",
		Comment:       req.Comment,
		AdminID:       req.AdminId,
		ExpiresAt:     &expiresAt,
	}

	if err := s.repo.CreateBooking(ctx, booking); err != nil {
		s.holdRepo.DeleteHold(ctx, holdKey)
		return nil, fmt.Errorf("failed to create booking: %w", err)
	}

	// Add event to outbox
	event := &commonpb.BookingEvent{
		BookingId:     bookingID,
		Table:         req.Table,
		Slot:          req.Slot,
		PartySize:     req.PartySize,
		CustomerName:  req.CustomerName,
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

func (s *Service) GetBooking(ctx context.Context, id string) (*bookingpb.Booking, error) {
	booking, err := s.repo.GetBooking(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.toBookingProto(booking), nil
}

func (s *Service) ListBookings(ctx context.Context, req *bookingpb.ListBookingsRequest) (*bookingpb.ListBookingsResponse, error) {
	filters := &dom.BookingFilters{
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

func (s *Service) ConfirmBooking(ctx context.Context, id, adminID string) (*bookingpb.Booking, error) {
	ctx, span := tracing.StartSpan(ctx, "ConfirmBooking")
	defer span.End()

	booking, err := s.repo.GetBooking(ctx, id)
	if err != nil {
		return nil, err
	}

	if booking.Status != "held" {
		return nil, fmt.Errorf("booking is not in held status")
	}

	// Update status
	if err := s.repo.UpdateBookingStatus(ctx, id, "confirmed"); err != nil {
		return nil, err
	}

	// Remove hold from Redis since booking is now confirmed
	holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
	s.holdRepo.DeleteHold(ctx, holdKey)

	booking.Status = "confirmed"
	booking.ExpiresAt = nil

	// Add event to outbox
	event := &commonpb.BookingEvent{
		BookingId: id,
		Payload: &commonpb.BookingEvent_Confirmed{
			Confirmed: &commonpb.BookingConfirmed{
				AdminId: adminID,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.confirmed", id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) CancelBooking(ctx context.Context, id, adminID, reason string) (*bookingpb.Booking, error) {
	ctx, span := tracing.StartSpan(ctx, "CancelBooking")
	defer span.End()

	booking, err := s.repo.GetBooking(ctx, id)
	if err != nil {
		return nil, err
	}

	if booking.Status == "finished" || booking.Status == "cancelled" {
		return nil, fmt.Errorf("booking cannot be cancelled")
	}

	// Update status
	if err := s.repo.UpdateBookingStatus(ctx, id, "cancelled"); err != nil {
		return nil, err
	}

	// Release hold
	holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
	s.holdRepo.DeleteHold(ctx, holdKey)

	booking.Status = "cancelled"

	// Add event to outbox
	event := &commonpb.BookingEvent{
		BookingId: id,
		Payload: &commonpb.BookingEvent_Cancelled{
			Cancelled: &commonpb.BookingCancelled{
				AdminId: adminID,
				Reason:  reason,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.cancelled", id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) MarkSeated(ctx context.Context, id, adminID string) (*bookingpb.Booking, error) {
	booking, err := s.repo.GetBooking(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateBookingStatus(ctx, id, "seated"); err != nil {
		return nil, err
	}

	booking.Status = "seated"

	event := &commonpb.BookingEvent{
		BookingId: id,
		Payload: &commonpb.BookingEvent_Seated{
			Seated: &commonpb.BookingSeated{
				AdminId: adminID,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.seated", id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) MarkFinished(ctx context.Context, id, adminID string) (*bookingpb.Booking, error) {
	booking, err := s.repo.GetBooking(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateBookingStatus(ctx, id, "finished"); err != nil {
		return nil, err
	}

	// Release hold
	holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
	s.holdRepo.DeleteHold(ctx, holdKey)

	booking.Status = "finished"

	event := &commonpb.BookingEvent{
		BookingId: id,
		Payload: &commonpb.BookingEvent_Finished{
			Finished: &commonpb.BookingFinished{
				AdminId: adminID,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.finished", id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
}

func (s *Service) MarkNoShow(ctx context.Context, id, adminID string) (*bookingpb.Booking, error) {
	booking, err := s.repo.GetBooking(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateBookingStatus(ctx, id, "no_show"); err != nil {
		return nil, err
	}

	// Release hold
	holdKey := s.getHoldKey(booking.VenueID, booking.TableID, booking.Date, booking.StartTime)
	s.holdRepo.DeleteHold(ctx, holdKey)

	booking.Status = "no_show"

	event := &commonpb.BookingEvent{
		BookingId: id,
		Payload: &commonpb.BookingEvent_NoShow{
			NoShow: &commonpb.BookingNoShow{
				AdminId: adminID,
			},
		},
	}

	if err := s.addToOutbox(ctx, "booking.no_show", id, event); err != nil {
		log.Error().Err(err).Msg("Failed to add to outbox")
	}

	return s.toBookingProto(booking), nil
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
	messages, err := s.eventRepo.GetPendingOutbox(ctx, 10)
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
				s.eventRepo.UpdateOutboxStatus(ctx, msg.ID, "failed", msg.RetryCount+1)
				continue
			}
			// Successfully unmarshaled with json, log warning
			log.Warn().Str("id", msg.ID).Msg("Unmarshaled event using legacy json format")
		}

		if err := s.kafkaProducer.PublishBookingEvent(ctx, msg.Topic, &event); err != nil {
			log.Error().Err(err).Str("id", msg.ID).Msg("Failed to publish event")
			if msg.RetryCount >= 3 {
				s.eventRepo.UpdateOutboxStatus(ctx, msg.ID, "dlq", msg.RetryCount+1)
			} else {
				s.eventRepo.UpdateOutboxStatus(ctx, msg.ID, "pending", msg.RetryCount+1)
			}
			continue
		}

		s.eventRepo.UpdateOutboxStatus(ctx, msg.ID, "sent", msg.RetryCount)
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
		s.holdRepo.DeleteHold(ctx, holdKey)

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
	return s.eventRepo.AddToOutbox(ctx, topic, key, data)
}

func (s *Service) getHoldKey(venueID, tableID, date, startTime string) string {
	return fmt.Sprintf("hold:%s:%s:%s:%s", venueID, tableID, date, startTime)
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

func (s *Service) toBookingProto(b *dom.Booking) *bookingpb.Booking {
	var expiresAt int64
	if b.ExpiresAt != nil {
		expiresAt = b.ExpiresAt.Unix()
	}

	return &bookingpb.Booking{
		Id:            b.ID,
		VenueId:       b.VenueID,
		Table:         &commonpb.TableRef{TableId: b.TableID},
		Slot:          &commonpb.Slot{Date: b.Date, StartTime: b.StartTime},
		PartySize:     b.PartySize,
		CustomerName:  b.CustomerName,
		CustomerPhone: b.CustomerPhone,
		Status:        b.Status,
		Comment:       b.Comment,
		AdminId:       b.AdminID,
		CreatedAt:     b.CreatedAt.Unix(),
		UpdatedAt:     b.UpdatedAt.Unix(),
		ExpiresAt:     expiresAt,
	}
}
