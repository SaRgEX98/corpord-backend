-- +goose Up
-- +goose StatementBegin
CREATE TABLE order_items (
    id SERIAL PRIMARY KEY,
    order_id INT NOT NULL,
    trip_id INT NOT NULL,
    departure_stop_id INT NOT NULL,
    arrival_stop_id INT NOT NULL,
    passenger_name VARCHAR(100) NOT NULL,
    passenger_document_number VARCHAR(50),
    seat_number VARCHAR(10),
    price DECIMAL(10, 2) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    FOREIGN KEY (trip_id) REFERENCES trips(id),
    FOREIGN KEY (departure_stop_id) REFERENCES stops(id),
    FOREIGN KEY (arrival_stop_id) REFERENCES stops(id),
    CONSTRAINT chk_positive_price CHECK (price >= 0),
    CONSTRAINT chk_different_stops CHECK (departure_stop_id != arrival_stop_id)
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_trip_id ON order_items(trip_id);
CREATE INDEX idx_order_items_passenger_name ON order_items(passenger_name);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS order_items CASCADE;
-- +goose StatementEnd
