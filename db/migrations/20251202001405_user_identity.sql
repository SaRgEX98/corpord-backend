-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_identities
(
    id          UUID PRIMARY KEY,
    user_id     integer      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider    TEXT      NOT NULL,
    provider_id TEXT      NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE user_identities;
-- +goose StatementEnd
