-- +goose Up
-- +goose StatementBegin
CREATE TABLE refresh_tokens
(
    id         UUID PRIMARY KEY   DEFAULT uuid_generate_v4(),
    user_id    int       NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT      NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT now(),
    updated_at TIMESTAMP NOT NULL DEFAULT now(),
    revoked    BOOLEAN   NOT NULL DEFAULT FALSE,
    ip         inet      NOT NULL,
    user_agent TEXT      NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE refresh_tokens;
-- +goose StatementEnd
