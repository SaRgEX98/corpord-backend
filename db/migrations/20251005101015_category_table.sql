-- +goose Up
-- +goose StatementBegin
CREATE TABLE bus_categories
(
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO bus_categories (name)
VALUES ('economy'),
       ('comfort');

ALTER TABLE bus
    ADD CONSTRAINT fk_bus_category
        FOREIGN KEY (category)
            REFERENCES bus_categories (id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE bus
    DROP CONSTRAINT IF EXISTS fk_bus_category;

DROP TABLE IF EXISTS bus_categories;
-- +goose StatementEnd
