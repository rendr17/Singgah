#!/usr/bin/env bash
# Runs the same quality gates as CI: frontend (pnpm/turbo) then every Go module.
set -euo pipefail

pnpm install --frozen-lockfile
pnpm format:check
pnpm lint
pnpm check
pnpm test
pnpm build

test -z "$(gofmt -l services)"

for mod in services/*/go.mod; do
	(cd "$(dirname "$mod")" && go vet ./... && go test ./... && go build ./...)
done
