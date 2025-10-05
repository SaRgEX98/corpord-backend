-- +goose Up
-- +goose StatementBegin
CREATE TABLE stops
(
    id         SERIAL PRIMARY KEY,
    name       TEXT      NOT NULL,
    address    TEXT,
    latitude   DECIMAL(10, 8),
    longitude  DECIMAL(11, 8),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_stops_name ON stops (name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS stops CASCADE;
-- +goose StatementEnd
