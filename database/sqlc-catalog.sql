-- Function signatures declared for sqlc's static analysis only.
-- These are NOT migrations and never run — the real functions come from
-- extensions created in migrations/ (e.g. 000001_extensions.sql).
CREATE FUNCTION postgis_version() RETURNS text AS
$$ SELECT ''::text $$ LANGUAGE sql;
