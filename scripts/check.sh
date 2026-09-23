#!/usr/bin/env bash
# Runs the same quality gates as CI: frontend (pnpm/turbo) then every Go module.
set -euo pipefail

pnpm install --frozen-lockfile
pnpm format:check
pnpm lint
pnpm check
pnpm test
pnpm build

pnpm contract:validate
pnpm client:generate
if [ -n "$(git status --porcelain -- packages/api-client/src/generated)" ]; then
	echo "generated client drift — run 'pnpm client:generate' and commit the output" >&2
	exit 1
fi

bash scripts/db-generate.sh
if [ -n "$(git status --porcelain -- services/api/db/generated)" ]; then
	echo "sqlc drift — run 'bash scripts/db-generate.sh' and commit the output" >&2
	exit 1
fi

test -z "$(gofmt -l services)"

for mod in services/*/go.mod; do
	(cd "$(dirname "$mod")" && go vet ./... && go test ./... && go build ./...)
done
