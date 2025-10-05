-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS bus_statuses
(
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO bus_statuses(name)
VALUES ('свободен'),
       ('в рейсе'),
       ('на ремонте');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bus_statuses;
-- +goose StatementEnd
