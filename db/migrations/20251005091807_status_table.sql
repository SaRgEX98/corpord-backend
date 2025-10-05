-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS bus_statuses
(
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bus_statuses;
-- +goose StatementEnd
