-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS bus
(
    license_plate TEXT PRIMARY KEY,
    brand TEXT NOT NULL,
    capacity INT NOT NULL CHECK ( capacity > 0 ),
    category INT NOT NULL,
    status INT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    FOREIGN KEY (status) REFERENCES bus_statuses(id)

);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bus;
-- +goose StatementEnd
