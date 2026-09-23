#!/usr/bin/env bash
# Applies pending goose migrations to DATABASE_URL (default: local compose).
# Tool version is pinned here — `go run pkg@version` downloads through the
# checksum database without polluting go.mod with the tool's dep tree.
set -euo pipefail

cd "$(dirname "$0")/.."
DBSTRING="${DATABASE_URL:-postgres://singgah:singgah@localhost:5432/singgah?sslmode=disable}"
exec go run github.com/pressly/goose/v3/cmd/goose@v3.28.0 -dir database/migrations postgres "$DBSTRING" "${@:-up}"
