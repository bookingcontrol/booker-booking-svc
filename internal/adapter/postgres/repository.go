package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	dom "github.com/bookingcontrol/booker-booking-svc/internal/domain/booking"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) dom.Repository {
	return &Repository{db: db}
}

// Booking operations
func (r *Repository) CreateBooking(ctx context.Context, booking *dom.Booking) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO bookings (id, venue_id, table_id, date, start_time, end_time, party_size, 
		 customer_name, customer_phone, status, comment, admin_id, created_at, updated_at, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NOW(), NOW(), $13)`,
		booking.ID, booking.VenueID, booking.TableID, booking.Date, booking.StartTime, booking.EndTime,
		booking.PartySize, booking.CustomerName, booking.CustomerPhone, booking.Status,
		booking.Comment, booking.AdminID, booking.ExpiresAt)
	return err
}

func (r *Repository) GetBooking(ctx context.Context, id string) (*dom.Booking, error) {
	var b dom.Booking
	err := r.db.QueryRow(ctx,
		`SELECT id, venue_id, table_id, date::text, start_time::text, end_time::text, party_size, customer_name,
		 customer_phone, status, comment, admin_id, created_at, updated_at, expires_at
		 FROM bookings WHERE id = $1`, id).
		Scan(&b.ID, &b.VenueID, &b.TableID, &b.Date, &b.StartTime, &b.EndTime,
			&b.PartySize, &b.CustomerName, &b.CustomerPhone, &b.Status,
			&b.Comment, &b.AdminID, &b.CreatedAt, &b.UpdatedAt, &b.ExpiresAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *Repository) ListBookings(ctx context.Context, filters *dom.BookingFilters) ([]*dom.Booking, int32, error) {
	where := []string{}
	args := []interface{}{}
	argPos := 1

	if filters.VenueID != "" {
		where = append(where, fmt.Sprintf("venue_id = $%d", argPos))
		args = append(args, filters.VenueID)
		argPos++
	}
	if filters.Date != "" {
		where = append(where, fmt.Sprintf("date = $%d", argPos))
		args = append(args, filters.Date)
		argPos++
	}
	if filters.Status != "" {
		where = append(where, fmt.Sprintf("status = $%d", argPos))
		args = append(args, filters.Status)
		argPos++
	}
	if filters.TableID != "" {
		where = append(where, fmt.Sprintf("table_id = $%d", argPos))
		args = append(args, filters.TableID)
		argPos++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + fmt.Sprintf("%s", where[0])
		for i := 1; i < len(where); i++ {
			whereClause += " AND " + where[i]
		}
	}

	var total int32
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM bookings %s", whereClause)
	err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	args = append(args, filters.Limit, filters.Offset)
	query := fmt.Sprintf(
		`SELECT id, venue_id, table_id, date::text, start_time::text, end_time::text, party_size, customer_name,
		 customer_phone, status, comment, admin_id, created_at, updated_at, expires_at
		 FROM bookings %s ORDER BY date, start_time LIMIT $%d OFFSET $%d`,
		whereClause, argPos, argPos+1)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var bookings []*dom.Booking
	for rows.Next() {
		var b dom.Booking
		if err := rows.Scan(&b.ID, &b.VenueID, &b.TableID, &b.Date, &b.StartTime, &b.EndTime,
			&b.PartySize, &b.CustomerName, &b.CustomerPhone, &b.Status,
			&b.Comment, &b.AdminID, &b.CreatedAt, &b.UpdatedAt, &b.ExpiresAt); err != nil {
			return nil, 0, err
		}
		bookings = append(bookings, &b)
	}

	return bookings, total, nil
}

func (r *Repository) UpdateBookingStatus(ctx context.Context, id, status string) error {
	_, err := r.db.Exec(ctx,
		`UPDATE bookings SET status = $1, updated_at = NOW() WHERE id = $2`,
		status, id)
	return err
}

func (r *Repository) CheckTableAvailability(ctx context.Context, venueID string, tableIDs []string, date, startTime, endTime string) (map[string]bool, error) {
	if len(tableIDs) == 0 {
		return make(map[string]bool), nil
	}

	// Build query with IN clause
	placeholders := make([]string, len(tableIDs))
	args := []interface{}{venueID, date, startTime, endTime}
	argPos := 5
	for i := range tableIDs {
		placeholders[i] = fmt.Sprintf("$%d", argPos)
		argPos++
	}
	args = append(args, make([]interface{}, len(tableIDs))...)
	for i, tableID := range tableIDs {
		args[4+i] = tableID
	}

	query := fmt.Sprintf(
		`SELECT DISTINCT table_id FROM bookings 
		 WHERE venue_id = $1 
		   AND date = $2 
		   AND status IN ('held', 'confirmed', 'seated')
		   AND (
		     (start_time <= $3 AND end_time > $3) OR
		     (start_time < $4 AND end_time >= $4) OR
		     (start_time >= $3 AND end_time <= $4)
		   )
		   AND table_id IN (%s)`,
		fmt.Sprintf("%s", placeholders[0]))
	for i := 1; i < len(placeholders); i++ {
		query = query[:len(query)-1] + fmt.Sprintf(", %s)", placeholders[i])
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookedTables := make(map[string]bool)
	for rows.Next() {
		var tableID string
		if err := rows.Scan(&tableID); err != nil {
			return nil, err
		}
		bookedTables[tableID] = true
	}

	// Build result map - all requested tables, marked as available if not in bookedTables
	result := make(map[string]bool)
	for _, tableID := range tableIDs {
		result[tableID] = !bookedTables[tableID]
	}

	return result, nil
}

func (r *Repository) GetExpiredHolds(ctx context.Context) ([]*dom.Booking, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, venue_id, table_id, date::text, start_time::text, end_time::text, party_size, customer_name,
		 customer_phone, status, comment, admin_id, created_at, updated_at, expires_at
		 FROM bookings WHERE status = 'held' AND expires_at < NOW()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*dom.Booking
	for rows.Next() {
		var b dom.Booking
		if err := rows.Scan(&b.ID, &b.VenueID, &b.TableID, &b.Date, &b.StartTime, &b.EndTime,
			&b.PartySize, &b.CustomerName, &b.CustomerPhone, &b.Status,
			&b.Comment, &b.AdminID, &b.CreatedAt, &b.UpdatedAt, &b.ExpiresAt); err != nil {
			return nil, err
		}
		bookings = append(bookings, &b)
	}

	return bookings, nil
}

// Outbox operations
func (r *Repository) AddToOutbox(ctx context.Context, topic, key string, payload []byte) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO outbox (id, topic, key, payload, status, retry_count, created_at)
		 VALUES ($1, $2, $3, $4, 'pending', 0, NOW())`,
		uuid.New().String(), topic, key, payload)
	return err
}

func (r *Repository) GetPendingOutbox(ctx context.Context, limit int32) ([]*dom.OutboxMessage, error) {
	rows, err := r.db.Query(ctx,
		`SELECT id, topic, key, payload, status, retry_count, created_at
		 FROM outbox WHERE status = 'pending' ORDER BY created_at LIMIT $1`,
		limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*dom.OutboxMessage
	for rows.Next() {
		var msg dom.OutboxMessage
		if err := rows.Scan(&msg.ID, &msg.Topic, &msg.Key, &msg.Payload, &msg.Status, &msg.RetryCount, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, &msg)
	}

	return messages, nil
}

func (r *Repository) UpdateOutboxStatus(ctx context.Context, id, status string, retryCount int32) error {
	_, err := r.db.Exec(ctx,
		`UPDATE outbox SET status = $1, retry_count = $2 WHERE id = $3`,
		status, retryCount, id)
	return err
}

