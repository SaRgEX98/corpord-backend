-- +goose Up
-- +goose StatementBegin
create table users
(
    id serial primary key,
    email varchar(255) not null unique,
    password_hash varchar(255) not null,
    name varchar(255) not null,
    created_at timestamp not null default now(),
    updated_at timestamp not null default now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
drop table users;
-- +goose StatementEnd
