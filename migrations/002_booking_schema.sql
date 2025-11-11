-- Booking service migrations

-- Bookings
CREATE TABLE IF NOT EXISTS bookings (
    id VARCHAR(36) PRIMARY KEY,
    venue_id VARCHAR(36) NOT NULL,
    table_id VARCHAR(36) NOT NULL,
    date DATE NOT NULL,
    start_time TIME NOT NULL,
    end_time TIME NOT NULL,
    party_size INTEGER NOT NULL,
    customer_name VARCHAR(255) NOT NULL,
    customer_phone VARCHAR(50),
    status VARCHAR(50) NOT NULL,
    comment TEXT,
    admin_id VARCHAR(36),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_bookings_venue_date ON bookings(venue_id, date, start_time);
CREATE INDEX IF NOT EXISTS idx_bookings_table_date ON bookings(table_id, date, start_time);
CREATE INDEX IF NOT EXISTS idx_bookings_status ON bookings(status) WHERE status IN ('held', 'confirmed', 'seated');
CREATE INDEX IF NOT EXISTS idx_bookings_expires_at ON bookings(expires_at) WHERE status = 'held';

-- Booking events (audit log)
CREATE TABLE IF NOT EXISTS booking_events (
    id VARCHAR(36) PRIMARY KEY,
    booking_id VARCHAR(36) NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    payload_json JSONB,
    ts TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_booking_events_booking_id ON booking_events(booking_id);
CREATE INDEX IF NOT EXISTS idx_booking_events_ts ON booking_events(ts);

-- Outbox pattern for reliable event publishing
CREATE TABLE IF NOT EXISTS outbox (
    id VARCHAR(36) PRIMARY KEY,
    topic VARCHAR(255) NOT NULL,
    key VARCHAR(255) NOT NULL,
    payload BYTEA NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    retry_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox(status, created_at) WHERE status = 'pending';



