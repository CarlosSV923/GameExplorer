# GameExplorer

[Español](README.es.md)

> **Status:** early development (phases 0–10 of [`docs/tasks.md`](docs/tasks.md) done: the web app works end to end against the API for Switch, Wii and PSP — sign in, browse the carousel, upload with a form and resumable transfers, confirm each file, edit and move games, the unassigned section fed by a Samba scan, trash and restore; next: the backend-less demo (phase 11) and the TrueNAS release (phase 12)).

Self-hosted web app for a home NAS (TrueNAS SCALE) to **upload, extract, classify, rename, browse and download emulation game files** from any browser: desktop, tablet, touch screen or gamepad. The UI is inspired by EmulationStation.

Drop a `.rar`, `.zip`, `.7z` or a raw game file, say which game and console it is (IGDB suggests names, but any name works), and GameExplorer extracts it, checks the files fit the console and stores them as `[console]/[game]/[file]` with consistent names. Switch base games, updates and DLC are handled; Wii and PSP keep one file per game.

## Highlights (planned)
- Resumable uploads of tens of gigabytes (tus protocol), streamed to disk.
- Each console is a module defined in code ([adding one](docs/adding-a-console.md) touches nothing else); extensions come from environment variables plus your own.
- Duplicate warnings before anything is written; trash with restore.
- Files added over Samba are picked up: unknown ones go to an "Unassigned" section to be assigned later.
- Single Go binary with the React app embedded; deployed as a TrueNAS custom app.
- **Backend-less demo** on Vercel: the same UI with in-browser adapters, so you can try the full flow without a NAS or game files.

## Architecture
| Layer | Tech |
|---|---|
| Backend | Go, `net/http`, tusd, SQLite (modernc + sqlc + goose), 7-Zip |
| Frontend | React, Vite, TypeScript, Tailwind, TanStack Router/Query, tus-js-client |
| Contract | OpenAPI 3 (spec-first; Go server and TS client are generated) |
| Design | Domain-Driven Design on both sides, ports and adapters, shared golden test vectors |

## Development
Requirements: Docker, Node 24+, pnpm and [Task](https://taskfile.dev). Go tooling (tests, lint, code generation, build) runs in Docker, so a local Go install is optional.

```bash
task install
task dev:api     # Go API in Docker, http://localhost:8080 (dev password: gameexplorer)
task dev:web     # http://localhost:5173 (proxies /api)
task dev:demo    # frontend only, simulated backend
task gen         # regenerate code after editing api/openapi.yaml or SQL queries
task check       # lint + test + build, same as CI
```

Configuration lives in environment variables. See [`.env.example`](.env.example).

## Legal
GameExplorer does not include, download or distribute games. Use it only with files you are entitled to keep. Game metadata is provided by [IGDB](https://www.igdb.com).

Licensed under the [MIT License](LICENSE).
