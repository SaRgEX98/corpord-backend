-- +goose Up
-- +goose StatementBegin
-- Prevent old application instances from inserting another duplicate between
-- cleanup and index creation. Goose runs this migration in one transaction.
LOCK TABLE refresh_tokens IN SHARE ROW EXCLUSIVE MODE;

-- Previous refresh rotation could create multiple sessions for one credential.
-- Revoke every affected credential, retaining one revoked row for uniqueness.
UPDATE refresh_tokens
SET revoked = true, updated_at = now()
WHERE token_hash IN (
    SELECT token_hash FROM refresh_tokens GROUP BY token_hash HAVING count(*) > 1
);

DELETE FROM refresh_tokens older
USING refresh_tokens keeper
WHERE older.token_hash = keeper.token_hash AND older.id > keeper.id;

CREATE UNIQUE INDEX refresh_tokens_token_hash_unique ON refresh_tokens (token_hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX refresh_tokens_token_hash_unique;
-- Revoked or removed duplicate sessions must not be restored on rollback.
-- +goose StatementEnd
