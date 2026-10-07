# GameExplorer — Guía para asistentes de IA

Self-hosted web app (TrueNAS SCALE) to upload, extract, classify, rename, browse and download emulation game files, with an EmulationStation-style UI. Public portfolio project with a backend-less demo on Vercel.

## Source of truth
- Requirements: [`docs/spec.md`](docs/spec.md). Do not assume requirements that are not documented there; ask and document first.
- Work status and order: [`docs/tasks.md`](docs/tasks.md). Only work on the current phase; never start a phase whose dependency is not done.
- HTTP contract: [`api/openapi.yaml`](api/openapi.yaml) (spec-first; generated code is never edited by hand).
- Rules shared by Go and TS: [`contracts/`](contracts/) golden vectors. Change the vector first, then both implementations.
- UI reference: mockups at https://claude.ai/artifact/CNf4mdh7jjnMfnjeNFXe56. Before UI work use the Anthropic design plugin skills (`design:design-critique`, `design:accessibility-review`, `design:design-handoff`).

## Layout
| Path | What |
|---|---|
| `apps/api` | Go module. `cmd/gameexplorer` is the composition root; `internal/<context>/{domain,application,infrastructure,interfaces}` |
| `apps/web` | React + Vite + TS. `src/app` (composition root), `src/modules/<context>/{domain,application,infrastructure/{http,demo},ui}`, `src/shared` |
| `api/` | OpenAPI contract |
| `contracts/` | Golden test vectors |
| `docs/` | Spec, tasks, design handoff |

Bounded contexts: `catalog` (also the trash: trashing and restoring must be atomic with the library), `ingestion`, `metadata` (IGDB anti-corruption layer), `identity`, `platform` (shared kernel). Every library change goes through the journaled operation engine in `catalog/application/engine.go`.

## Rules
- DDD: `domain` packages are pure (no I/O, no framework imports). Cross-context calls go through application ports.
- Huge files (tens of GB): never buffer file contents in memory. Stream, `rename`, delegate extraction to `7zz`, serve with `http.ServeContent`.
- Paths are built server-side from IDs; never trust client paths.
- The frontend never knows which adapter set is active; `VITE_DATA_SOURCE` is read only in `src/app`.
- Touch, keyboard and gamepad must all work; gamepad glyphs only when a gamepad is detected.
- UI copy: Spanish default, English available (i18n keys, no hardcoded strings once i18n lands in phase 8).

## Commands
`task check` runs everything CI runs (Go tasks build the `gameexplorer-dev` image first: Go + official 7-Zip with RAR). Also: `task dev:api` (Go API in Docker, password `gameexplorer`), `task dev:web`, `task dev:demo`, `task gen`, `task fmt`, `task tidy`, `task lint`, `task test`, `task build`, `task hash-password`, `task demo-catalog` (needs IGDB credentials in `.env.local`).
Go tooling (tests, lint, oapi-codegen, sqlc, build) runs in Docker through the Taskfile: Windows Application Control on the dev machine blocks freshly compiled Go binaries, and Docker matches CI. After editing `api/openapi.yaml` or SQL queries run `task gen`; CI fails on stale generated code.

## Conventions
- Go: gofumpt + goimports, golangci-lint v2 config in `apps/api/.golangci.yml`, table tests, `t.Parallel()`.
- TS: strict + `exactOptionalPropertyTypes`, ESLint `strictTypeChecked`, Prettier (no semicolons, single quotes).
- Commits: small and focused; every change keeps `task check` green.
- Docs: `docs/` in Spanish; `README.md` (EN) and `README.es.md` (ES) stay in sync.
