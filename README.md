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
apps/web          SvelteKit client (@singgah/web)
services/api      Go HTTP API module (minimal placeholder for now)
contracts/        OpenAPI contract (added with the API foundation)
packages/         shared client packages (added when real)
docs/             development blueprint — read docs/00_START_HERE.md first
.github/          CI workflow + PR template
```

## Rules of the house

- Formatting is unified at the root (`pnpm format`); do not add per-package prettier configs.
- Frontend consumes canonical API models only — never provider payloads.
- Authoritative backend logic is Go. Do not add a TypeScript backend.
- Full rules: `docs/AGENTS.md`.
