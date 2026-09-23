# database/

- `migrations/` — goose SQL migrations. The schema is never changed by hand.
- `queries/` — reviewed SQL consumed by `sqlc` to generate type-safe Go.

Both directories are populated by the PostgreSQL/PostGIS foundation change.
