-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS drivers
(
    id           SERIAL PRIMARY KEY,
    first_name   TEXT NOT NULL,
    last_name    TEXT NOT NULL,
    middle_name  TEXT NOT NULL,
    phone_number TEXT NOT NULL,
    status       INT  NOT NULL DEFAULT 1,
    FOREIGN KEY (status) REFERENCES driver_status (id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS drivers;
-- +goose StatementEnd
