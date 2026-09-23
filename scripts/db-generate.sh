#!/usr/bin/env bash
# Regenerates services/api/db/generated from database/queries + migrations.
# Output is committed; CI fails on drift.
set -euo pipefail

cd "$(dirname "$0")/.."
go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate -f database/sqlc.yaml
