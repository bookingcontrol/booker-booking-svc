package grpc

import (
	"context"

	bookingpb "github.com/bookingcontrol/booker-contracts-go/booking"
	"github.com/bookingcontrol/booker-booking-svc/internal/usecase/booking"
)

type Handler struct {
	bookingpb.UnimplementedBookingServiceServer
	bookingService *booking.Service
}

func NewHandler(bookingService *booking.Service) *Handler {
	return &Handler{
		bookingService: bookingService,
	}
}

func (h *Handler) CreateBooking(ctx context.Context, req *bookingpb.CreateBookingRequest) (*bookingpb.Booking, error) {
	return h.bookingService.CreateBooking(ctx, req)
}

func (h *Handler) GetBooking(ctx context.Context, req *bookingpb.GetBookingRequest) (*bookingpb.Booking, error) {
	return h.bookingService.GetBooking(ctx, req.Id)
}

func (h *Handler) ListBookings(ctx context.Context, req *bookingpb.ListBookingsRequest) (*bookingpb.ListBookingsResponse, error) {
	return h.bookingService.ListBookings(ctx, req)
}

func (h *Handler) ConfirmBooking(ctx context.Context, req *bookingpb.ConfirmBookingRequest) (*bookingpb.Booking, error) {
	return h.bookingService.ConfirmBooking(ctx, req.Id, req.AdminId)
}

func (h *Handler) CancelBooking(ctx context.Context, req *bookingpb.CancelBookingRequest) (*bookingpb.Booking, error) {
	return h.bookingService.CancelBooking(ctx, req.Id, req.AdminId, req.Reason)
}

func (h *Handler) MarkSeated(ctx context.Context, req *bookingpb.MarkSeatedRequest) (*bookingpb.Booking, error) {
	return h.bookingService.MarkSeated(ctx, req.Id, req.AdminId)
}

func (h *Handler) MarkFinished(ctx context.Context, req *bookingpb.MarkFinishedRequest) (*bookingpb.Booking, error) {
	return h.bookingService.MarkFinished(ctx, req.Id, req.AdminId)
}

func (h *Handler) MarkNoShow(ctx context.Context, req *bookingpb.MarkNoShowRequest) (*bookingpb.Booking, error) {
	return h.bookingService.MarkNoShow(ctx, req.Id, req.AdminId)
}

func (h *Handler) CheckTableAvailability(ctx context.Context, req *bookingpb.CheckTableAvailabilityRequest) (*bookingpb.CheckTableAvailabilityResponse, error) {
	return h.bookingService.CheckTableAvailability(ctx, req)
}

