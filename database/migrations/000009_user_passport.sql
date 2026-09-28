-- +goose Up
-- User-owned data foundation (docs/32 user-owned block, ADR-010).
-- Identity is deliberately minimal: an anonymous-first `users` row plus an
-- opaque session token. Only the SHA-256 hash of the token is stored — a
-- database leak must not leak usable credentials, and there is deliberately
-- no email/password/PII to protect.
--
-- visit_events is append-only (docs/33): corrections are new events, never
-- edits. (user_id, client_mutation_id) uniqueness makes offline replay
-- idempotent (docs/24 mutation queue).

CREATE TABLE users (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	created_at timestamptz NOT NULL DEFAULT now(),
	-- Cheap activity signal refreshed on session verification; also the
	-- basis for future inactive-account cleanup.
	last_seen_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE auth_sessions (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
	-- SHA-256(token) — the raw token is only ever returned to the client once.
	token_hash bytea NOT NULL UNIQUE,
	created_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL,
	revoked_at timestamptz
);
CREATE INDEX auth_sessions_user_ix ON auth_sessions (user_id);

CREATE TABLE visit_events (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id uuid NOT NULL REFERENCES users (id),
	stop_id uuid NOT NULL REFERENCES stops (id),
	client_mutation_id uuid NOT NULL,
	-- When the user says the visit happened; server acceptance time is
	-- created_at. Keep both honest — replayed offline events arrive late.
	observed_at timestamptz NOT NULL,
	-- geofence = verified against device location at tap time;
	-- manual = user-attested without location proof (still recorded, but
	-- marked by status, never silently upgraded).
	validation_method text NOT NULL CHECK (validation_method IN ('geofence', 'manual', 'trip')),
	distance_m numeric,
	-- confirmed = geofence inside threshold; low_confidence = outside or
	-- no location (docs/42: mark, don't block — passport is personal).
	status text NOT NULL CHECK (status IN ('confirmed', 'low_confidence')),
	created_at timestamptz NOT NULL DEFAULT now(),
	UNIQUE (user_id, client_mutation_id)
);
CREATE INDEX visit_events_user_stop_ix ON visit_events (user_id, stop_id);

CREATE TABLE journal_entries (
	id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
	user_id uuid NOT NULL REFERENCES users (id),
	stop_id uuid REFERENCES stops (id),
	visit_event_id uuid REFERENCES visit_events (id),
	body text NOT NULL,
	visibility text NOT NULL DEFAULT 'private' CHECK (visibility IN ('private')),
	created_at timestamptz NOT NULL DEFAULT now(),
	updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX journal_entries_user_ix ON journal_entries (user_id);

-- +goose Down
DROP TABLE journal_entries;
DROP TABLE visit_events;
DROP TABLE auth_sessions;
DROP TABLE users;
