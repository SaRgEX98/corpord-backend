-- +goose Up
-- +goose StatementBegin
alter table user_identities
    add updated_at timestamp default now();
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
alter table user_identities
    drop column updated_at;
-- +goose StatementEnd
