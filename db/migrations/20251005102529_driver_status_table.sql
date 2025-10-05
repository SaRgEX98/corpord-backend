-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS driver_status
(
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO driver_status (name)
VALUES ('доступен'),
       ('в рейсе'),
       ('болезнь'),
       ('отпуск');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS driver_status;
-- +goose StatementEnd
