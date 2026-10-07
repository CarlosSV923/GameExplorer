# GameExplorer — Tareas

> Estados: `[ ]` pendiente · `[~]` en curso · `[x]` hecha. No se empieza una fase si su dependencia no está hecha.
> Requisitos: [`spec.md`](./spec.md) (los IDs RF/RNF se citan en cada fase).

## Fase 0 — Base del repositorio
Depende de: —
- [x] Monorepo: Go (`apps/api`) + pnpm (`apps/web`), `.editorconfig`, `.gitattributes`, `.gitignore`
- [x] API mínima: `/api/health`, apagado ordenado, timeouts para transferencias largas, test
- [x] Web mínima: React + Vite + TS estricto, Vitest, ESLint, Prettier, `VITE_DATA_SOURCE`
- [x] `golangci-lint` v2, `Taskfile.yml` (`task check`), CI en GitHub Actions
- [x] Esqueleto de `api/openapi.yaml` y vectores dorados en `contracts/` (RNF-08)
- [x] `docs/spec.md`, `docs/tasks.md`, `CLAUDE.md`, README EN/ES, LICENSE, `.env.example`
- [x] Repositorio público en GitHub + primer push

## Fase 0.5 — Spike de archivos grandes en el NAS
Depende de: Fase 0 · Cubre: RNF-01, RNF-03, RNF-04
- [x] Prototipo desechable: tusd + 7zz + `http.ServeContent` + zip *store* (`spikes/large-files`, validado en local)
- [x] Medir en el NAS con 15 GB reales de Switch: subida, reanudación, extracción, verificación CRC32, descarga con Range y como zip (pico de RAM 14,7 MB)
- [~] Verificar permisos SMB y `rename` dentro del dataset — `rename` OK; falta confirmar edición por Samba
- [x] Registrar los resultados en `docs/spike-results.md`

## Fase 1 — API base
Depende de: 0.5 · Cubre: RF-40, RF-50, RNF-02
- [ ] Configuración con `caarlos0/env` y `slog`
- [ ] SQLite (modernc) + goose + sqlc; seed de las 6 consolas
- [ ] Contexto `identity`: login con argon2id, cookie firmada, rate-limit; helper para generar `APP_PASSWORD_HASH`
- [ ] Generación con oapi-codegen (strict server) y cliente TS (openapi-typescript)
- [ ] SPA incrustada con `embed.FS`; fallback a `index.html`

## Fase 2 — Contexto `metadata` (IGDB)
Depende de: 1 · Cubre: RF-09, RF-20, RF-41, RF-61
- [ ] Cliente IGDB (token client-credentials cacheado, búsqueda de juegos por plataforma y de plataformas)
- [ ] Caché de portadas y logos en `/data/cache`
- [ ] Comando `cmd/demo-catalog` que genera el catálogo fijo de la demo

## Fase 3 — Contexto `ingestion` I: subidas
Depende de: 1 · Cubre: RF-01, RF-02, RF-12, RF-13
- [ ] tusd montado en `/api/uploads/` (filestore + filelocker)
- [ ] Agregado `UploadJob` (máquina de estados) persistido
- [ ] SSE de progreso; purga de abandonados

## Fase 4 — Contexto `ingestion` II: extracción y detección
Depende de: 3 · Cubre: RF-03 a RF-08
- [ ] Puerto `Extractor` + adaptador 7zz (progreso, contraseña, espacio libre, zip-slip)
- [ ] Escáner: agrupado cue/bin, juegos en carpeta, lista de ignorados
- [ ] Detectores por consola + vectores `detection-cases.json` + cabeceras sintéticas

## Fase 5 — Commit al contexto `catalog`
Depende de: 2, 4 · Cubre: RF-09 a RF-11
- [ ] `NamingPolicy` + vectores `naming-cases.json`; `DuplicatePolicy`
- [ ] Commit con journal y rollback; reescritura de `.cue`; PUID/PGID/UMASK

## Fase 6 — Lectura del catálogo y descargas
Depende de: 5 · Cubre: RF-20 a RF-23
- [ ] Listados, búsqueda global y detalle
- [ ] Descarga con Range y zip *store* en streaming

## Fase 7 — Re-emparejar, papelera, consolas e integridad
Depende de: 6 · Cubre: RF-24 a RF-26, RF-30, RF-41
- [ ] Re-emparejar con renombrado reversible
- [ ] Contexto `trash`: mover, restaurar y purga programada
- [ ] CRUD y orden de consolas; chequeo de integridad

## Fase 8 — Diseño → código
Depende de: 0 · Cubre: RNF-05 a RNF-07
- [ ] `design:design-critique` y `design:accessibility-review` sobre los mockups
- [ ] `design:design-handoff` → `docs/design-handoff.md`
- [ ] Tokens en Tailwind, `shared/ui`, i18n (ES/EN), capa de entrada táctil/teclado/gamepad

## Fase 9 — Frontend con adaptadores HTTP
Depende de: 7, 8 · Cubre: RF-01 a RF-51 (UI)
- [ ] Puertos de la aplicación y adaptadores `http/` por módulo
- [ ] Pantallas: carrusel, lista/detalle, subida + revisión, papelera, ajustes, login

## Fase 10 — Modo demo
Depende de: 9 · Cubre: RF-60 a RF-65
- [ ] Adaptadores `demo/`, catálogo fijo, archivos de ejemplo, descargas simuladas, banner
- [ ] E2E en modo demo (escritorio y perfiles táctiles) en CI
- [ ] Proyecto en Vercel (`apps/web`, `VITE_DATA_SOURCE=demo`)

## Fase 11 — Producción y portfolio
Depende de: 10
- [ ] Dockerfile multi-stage (web → go con embed → alpine + 7zip) y release en GHCR
- [ ] `deploy/truenas/compose.yaml` + guía (datasets, PUID/PGID, veto files)
- [ ] README EN/ES con capturas/GIF y diagramas de arquitectura
