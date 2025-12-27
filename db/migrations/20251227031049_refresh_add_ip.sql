-- +goose Up

ALTER TABLE refresh_tokens
    ADD COLUMN ip TEXT,
    ADD COLUMN user_agent TEXT;

UPDATE refresh_tokens
SET
    ip = '0.0.0.0',
    user_agent = 'unknown'
WHERE ip IS NULL
   OR user_agent IS NULL;

ALTER TABLE refresh_tokens
    ALTER COLUMN ip SET NOT NULL,
    ALTER COLUMN user_agent SET NOT NULL;


-- +goose Down

ALTER TABLE refresh_tokens
    DROP COLUMN ip,
    DROP COLUMN user_agent;
