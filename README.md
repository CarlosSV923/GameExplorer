# GameExplorer

[Español](README.es.md)

> **Status:** early development (phase 1 of [`docs/tasks.md`](docs/tasks.md) done). Nothing usable yet.

Self-hosted web app for a home NAS (TrueNAS SCALE) to **upload, extract, classify, rename, browse and download emulation game files** from any browser: desktop, tablet, touch screen or gamepad. The UI is inspired by EmulationStation.

Drop a `.rar`, `.zip`, `.7z` or a raw game file. GameExplorer extracts it, detects the console from the file header, lets you pick the game from IGDB, and stores it as `[console]/[game]/[file]` with consistent names. Base games, updates, DLC and multi-disc games are all handled.

## Highlights (planned)
- Resumable uploads of tens of gigabytes (tus protocol), streamed to disk.
- Console detection from magic bytes (GameCube/Wii disc headers, PS1/PS2 `SYSTEM.CNF`, PS3 folder layout, Switch title IDs).
- Duplicate warnings before anything is written; trash with restore.
- Single Go binary with the React app embedded; deployed as a TrueNAS custom app.
- **Backend-less demo** on Vercel: the same UI with in-browser adapters, so you can try the full flow without a NAS or game files.

## Architecture
| Layer | Tech |
|---|---|
| Backend | Go, `net/http`, tusd, SQLite (modernc + sqlc + goose), 7-Zip |
| Frontend | React, Vite, TypeScript, Tailwind, TanStack Router/Query, Uppy |
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
