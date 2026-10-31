# User role response contract

2026-11-01. `GET /users/me` and administrative user responses previously returned
an empty `role`: the user repository selected user columns without reading the
related `roles.name`. Authentication already used a separate repository with a
role join, so a server-authorized administrator could still appear unprivileged
to the frontend.

`GetByID`, `GetAll` and `GetByEmail` now join the persisted role. Create returns the
actual database-default role in the same `INSERT ... RETURNING` statement; Update
continues to read its response through `GetByID`. Existing response fields, missing
record behavior and authorization checks remain unchanged. There are no schema,
role-assignment, seed or deployment changes.

The role migration includes `user`, `admin` and `moderator`. Clients must grant
administrator UI access only for the exact `admin` role.

Validation: `TestUserRepositoryPostgresReturnsCurrentRoles` applies real migrations
in an isolated PostgreSQL schema and checks default-role creation, profile/email/
list reads and update responses while changing persisted roles from user to admin,
moderator and back to user. It also verifies that another user's role and missing
record errors remain intact. Run using a disposable database:

```bash
GOMAXPROCS=2 go test -p 2 -race -count=1 ./internal/repository/pg \
  -run TestUserRepositoryPostgresReturnsCurrentRoles
```

Set `TEST_DATABASE_URL` in the environment. The regression passed against a separate
`corpord_roles_regression` database inside the disposable `corpord-ui-test-postgres`
container. The test removes its own schema; no application data was accessed.
