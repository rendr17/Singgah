-- name: PostGISVersion :one
-- Readiness probe: returns the installed PostGIS version, e.g. "3.5 ...".
-- Unqualified on purpose: postgis lives in public locally and in extensions
-- on Supabase — both are on the search_path. Signature for sqlc's analysis
-- lives in database/sqlc-catalog.sql.
SELECT postgis_version() AS postgis_version;

