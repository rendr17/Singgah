-- +goose Up
-- pg_trgm installs into a dedicated `extensions` schema to match the managed
-- Supabase layout (docs/32_DATABASE_SCHEMA.md). PostGIS is NOT relocatable in
-- stock builds — `WITH SCHEMA` is silently ignored and it lands in `public`.
-- On Supabase it lives in `extensions`, and both schemas are on the default
-- search_path there, so queries must call extension functions UNQUALIFIED.
CREATE SCHEMA IF NOT EXISTS extensions;
CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA extensions;
CREATE EXTENSION IF NOT EXISTS postgis;

-- +goose Down
-- postgis is intentionally retained: the container image and managed
-- Supabase pre-install it, and dropping it cascades into spatial objects.
DROP EXTENSION IF EXISTS pg_trgm;
DROP SCHEMA IF EXISTS extensions;
