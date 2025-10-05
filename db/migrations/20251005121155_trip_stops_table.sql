-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS trip_stops
(
    id              SERIAL PRIMARY KEY,
    trip_id         INT NOT NULL,
    stop_id         INT NOT NULL,
    arrival_time    TIMESTAMP NOT NULL,
    departure_time  TIMESTAMP NOT NULL,
    stop_order      INT NOT NULL,
    price_to_next   DECIMAL(10, 2),

    FOREIGN KEY (trip_id) REFERENCES trips(id) ON DELETE CASCADE,
    FOREIGN KEY (stop_id) REFERENCES stops(id) ON DELETE CASCADE,
    UNIQUE (trip_id, stop_order),
    CHECK (departure_time >= arrival_time),
    CHECK (stop_order > 0)
);

CREATE INDEX idx_trip_stops_trip_id ON trip_stops(trip_id);
CREATE INDEX idx_trip_stops_stop_id ON trip_stops(stop_id);
CREATE INDEX idx_trip_stops_arrival ON trip_stops(arrival_time);
CREATE INDEX idx_trip_stops_departure ON trip_stops(departure_time);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS trip_stops;
-- +goose StatementEnd
