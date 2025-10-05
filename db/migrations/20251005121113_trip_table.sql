-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS trips
(
    id              SERIAL PRIMARY KEY,
    bus_id          TEXT NOT NULL,
    driver_id       INT NOT NULL,
    start_time      TIMESTAMP NOT NULL,
    end_time        TIMESTAMP,
    status          TEXT NOT NULL DEFAULT 'scheduled',
    base_price      DECIMAL(10, 2) NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    FOREIGN KEY (bus_id) REFERENCES bus(license_plate),
    FOREIGN KEY (driver_id) REFERENCES drivers(id),
    CHECK (end_time IS NULL OR end_time > start_time)
);

CREATE INDEX idx_trips_bus_id ON trips(bus_id);
CREATE INDEX idx_trips_driver_id ON trips(driver_id);
CREATE INDEX idx_trips_start_time ON trips(start_time);
CREATE INDEX idx_trips_status ON trips(status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trips CASCADE;
-- +goose StatementEnd
