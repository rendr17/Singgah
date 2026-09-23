# Singgah

Calm, transit-first companion for exploring the city by public transport.
Product direction, architecture, and delivery rules live in [`docs/`](docs/00_START_HERE.md).

## Stack

| Side     | Stack                                                          |
| -------- | -------------------------------------------------------------- |
| Client   | SvelteKit + Svelte 5 + strict TypeScript, Tailwind CSS, Vitest |
| Server   | Go (`net/http` + `chi`), PostgreSQL/PostGIS, `pgx` + `sqlc`    |
| Tooling  | pnpm workspaces + Turborepo (client), `go.work` (services)     |
| Contract | OpenAPI → generated TypeScript client (arrives with the API)   |

## Prerequisites

- Node.js 24 (`.nvmrc`)
- pnpm 11.13.1 (`packageManager` field — use Corepack or a matching install)
- Go 1.26 (`go.work`)

## Getting started

```bash
pnpm install
pnpm dev        # SvelteKit dev server (apps/web)

cd services/api && go run ./cmd/api   # API on :8080 — /health, /version
```

Quality gates (run from repo root):

```bash
pnpm format:check   # prettier over the whole repo
pnpm lint           # eslint via turbo
pnpm check          # svelte-check typecheck
pnpm test           # vitest unit tests
pnpm build          # production build

test -z "$(gofmt -l services)"   # Go formatting
cd services/api                  # Go commands run inside each module
go vet ./... && go test ./... && go build ./...
```

## Layout

```text
apps/web                SvelteKit client (@singgah/web)
packages/ui             shared Svelte UI primitives (@singgah/ui)
packages/design-tokens  CSS design tokens (@singgah/design-tokens)
services/api            Go HTTP API — chi router, /health, /version
contracts/openapi       OpenAPI contract — singgah.yaml skeleton, grows with the API
database/               goose migrations + sqlc queries (added with the DB foundation)
scripts/                repo helpers — check.sh runs all local quality gates
docs/                   development blueprint (local reference, not committed)
.github/                CI workflow + PR template
```

The full target tree and the incremental-growth rule live in
`docs/64_FOLDER_STRUCTURE.md`.

Run everything CI runs locally with `bash scripts/check.sh`.

## Rules of the house

- Formatting is unified at the root (`pnpm format`); do not add per-package prettier configs.
- Frontend consumes canonical API models only — never provider payloads.
- Authoritative backend logic is Go. Do not add a TypeScript backend.
- Full rules: `docs/AGENTS.md`.
