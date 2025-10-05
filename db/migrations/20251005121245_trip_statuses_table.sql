-- +goose Up
-- +goose StatementBegin
CREATE TYPE trip_status AS ENUM (
    'scheduled',
    'boarding',
    'departed',
    'in_transit',
    'arrived',
    'completed',
    'cancelled',
    'delayed'
    );

CREATE TABLE IF NOT EXISTS trip_status_history
(
    id         SERIAL PRIMARY KEY,
    trip_id    INT         NOT NULL,
    status     trip_status NOT NULL,
    changed_at TIMESTAMP   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    notes      TEXT,

    FOREIGN KEY (trip_id) REFERENCES trips (id) ON DELETE CASCADE
);

CREATE INDEX idx_trip_status_history_trip_id ON trip_status_history (trip_id);
CREATE INDEX idx_trip_status_history_changed_at ON trip_status_history (changed_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trip_status_history;
DROP TYPE IF EXISTS trip_status;
-- +goose StatementEnd
