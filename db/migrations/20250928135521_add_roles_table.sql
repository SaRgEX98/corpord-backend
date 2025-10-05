-- +goose Up
-- +goose StatementBegin

-- Create roles table
CREATE TABLE roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

INSERT INTO roles (name, description) VALUES
    ('user', 'Regular user'),
    ('admin', 'Administrator'),
    ('moderator', 'Content moderator');

ALTER TABLE users 
    ADD COLUMN role_id INTEGER NOT NULL DEFAULT 1 
    REFERENCES roles(id) 
    ON DELETE RESTRICT;

UPDATE users SET role_id = 1;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP CONSTRAINT users_role_id_fkey;

ALTER TABLE users DROP COLUMN role_id;

DROP TABLE IF EXISTS roles;

-- +goose StatementEnd
