# database/

Schema and SQL source of truth. Goose owns migrations; sqlc owns query
codegen; reviewers review the SQL here, never the generated Go.

```text
migrations/      goose SQL migrations — never edit an applied one
queries/         sqlc query sources
sqlc-catalog.sql declared schema for sqlc analysis only (not run) — mirror
                 migrations/ DDL here; goose Down sections make migrations/
                 unusable as sqlc's schema source
sqlc.yaml        sqlc codegen config → services/api/db/generated
```

## Commands

```bash
# start local Postgres+PostGIS (Docker)
docker compose -f infrastructure/local/compose.yaml up -d

# apply migrations (DATABASE_URL defaults to local compose)
bash scripts/db-migrate.sh            # up
bash scripts/db-migrate.sh status
bash scripts/db-migrate.sh down       # one step back — dev only

# regenerate services/api/db/generated after changing migrations/queries
bash scripts/db-generate.sh
```

Tool versions (`goose`, `sqlc`) are pinned inside those scripts via
`go run pkg@version` — the module cache verifies checksums and `go.mod`
stays free of tool dependency trees.

Integration tests run against `TEST_DATABASE_URL` (compose creates
`singgah_test` automatically):

```bash
export TEST_DATABASE_URL=postgres://singgah:singgah@localhost:5432/singgah_test?sslmode=disable
DATABASE_URL="$TEST_DATABASE_URL" bash scripts/db-migrate.sh   # migrate the test db
cd services/api && go test ./internal/db/
```
