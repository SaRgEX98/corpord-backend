-- +goose Up
-- +goose StatementBegin
ALTER TABLE bus
    RENAME COLUMN category to category_id;

ALTER TABLE bus
    RENAME COLUMN status to status_id;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE bus
    RENAME COLUMN category_id to category;

ALTER TABLE bus
    RENAME COLUMN status_id to status;
-- +goose StatementEnd
