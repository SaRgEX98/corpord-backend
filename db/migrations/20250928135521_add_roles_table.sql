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

-- Insert default roles
INSERT INTO roles (name, description) VALUES 
    ('user', 'Regular user'),
    ('admin', 'Administrator'),
    ('moderator', 'Content moderator');

-- Add role_id column to users table
ALTER TABLE users 
    ADD COLUMN role_id INTEGER NOT NULL DEFAULT 1 
    REFERENCES roles(id) 
    ON DELETE RESTRICT;

-- Update existing users to have the default user role
UPDATE users SET role_id = 1;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Remove role_id foreign key constraint
ALTER TABLE users DROP CONSTRAINT users_role_id_fkey;

-- Drop role_id column
ALTER TABLE users DROP COLUMN role_id;

-- Drop roles table
DROP TABLE IF EXISTS roles;

-- +goose StatementEnd
