-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS order_statuses
(
    id          SERIAL PRIMARY KEY,
    code        VARCHAR(20) UNIQUE NOT NULL,
    name        VARCHAR(50)        NOT NULL,
    description TEXT,
    is_active   BOOLEAN DEFAULT TRUE
);

INSERT INTO order_statuses (code, name)
VALUES ('pending', 'В ожидании'),
       ('confirmed', 'Подтвержден'),
       ('paid', 'Оплачен'),
       ('cancelled', 'Отменен'),
       ('completed', 'Завершен'),
       ('refunded', 'Возвращен');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS order_statuses CASCADE;
-- +goose StatementEnd
