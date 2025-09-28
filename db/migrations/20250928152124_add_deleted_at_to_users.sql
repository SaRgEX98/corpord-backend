-- +goose Up
-- +goose StatementBegin
ALTER TABLE users 
    ADD COLUMN deleted_at TIMESTAMP WITH TIME ZONE DEFAULT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users 
    DROP COLUMN IF EXISTS deleted_at;
-- +goose StatementEnd
