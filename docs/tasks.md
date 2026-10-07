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
- [x] Verificar permisos SMB y `rename` dentro del dataset (archivo creado por la app renombrado por Samba y visto por la app)
- [x] Registrar los resultados en `docs/spike-results.md`

## Fase 1 — API base
Depende de: 0.5 · Cubre: RF-40, RF-50, RNF-02
- [x] Configuración con `caarlos0/env` y `slog` (validación con errores claros)
- [x] SQLite (modernc, WAL) + goose + sqlc; seed de las 6 consolas; `GET /api/consoles`
- [x] Contexto `identity`: login con argon2id, cookie HMAC httpOnly/SameSite=Strict, rate-limit (5/min por IP), `hash-password` y alternativa `APP_PASSWORD`
- [x] Generación con oapi-codegen (strict server) y cliente TS (openapi-typescript + openapi-fetch); CI verifica que lo generado esté al día
- [x] SPA incrustada con `embed.FS`; fallback a `index.html`, caché inmutable para assets
- [x] `/api/health` con diagnósticos (escritura en biblioteca y datos) para RNF-03
- [x] Entorno local con `compose.dev.yaml`; herramientas de Go en Docker (Control de aplicaciones de Windows bloquea binarios recién compilados)

## Fase 2 — Contexto `metadata` (IGDB)
Depende de: 1 · Cubre: RF-09, RF-20, RF-41, RF-61
- [x] Cliente IGDB (token client-credentials cacheado y renovado ante 401, límite de 4 req/s, búsqueda de juegos por plataforma y de plataformas)
- [x] Filtro de tipos jugables (excluye bundles/DLC) y reordenamiento por nombre (exacto > prefijo > palabra)
- [x] Caché de portadas y logos en `/data/cache/images` (descarga única por imagen, escritura atómica); `GET /api/images/{size}/{id}`
- [x] Sincronización de logo y año de las consolas al arrancar; logo del modelo original (IGDB entrega el de la última revisión)
- [x] Comando `cmd/demo-catalog` (`task demo-catalog`): 81 juegos con portada, incluidos los títulos de los archivos de ejemplo

## Fase 3 — Contexto `ingestion` I: subidas
Depende de: 1 · Cubre: RF-01, RF-02, RF-12, RF-13
- [x] tusd montado en `/api/uploads/` (filestore + filelocker) dentro de la biblioteca, protegido por sesión; valida nombre y espacio libre (507)
- [x] Agregado `UploadJob` con la máquina de estados completa del pipeline, persistido en SQLite; origen de consola (RF-08) por metadata `consoleSlug`
- [x] `/api/jobs` (listar, detalle, cancelar) y SSE `/api/jobs/events` con snapshot inicial, heartbeat y cierre limpio al apagar
- [x] Purga horaria: subidas inactivas 24 h y archivos huérfanos del store
- [x] Si la biblioteca no es escribible, las subidas responden 503 y la app sigue en pie
- [x] Medido con archivos reales: 1,5 GB a 72 MB/s; RAM del proceso plana durante la subida; argon2id bajado a los parámetros mínimos de OWASP (151 → 61 MB)

## Fase 4 — Contexto `ingestion` II: extracción y detección
Depende de: 3 · Cubre: RF-03 a RF-08
- [x] 7-Zip oficial con RAR en todos lados: `scripts/install-7zip.sh` (hash verificado) usado por la imagen de desarrollo (`deploy/dev/Dockerfile`), CI y, en la fase 11, producción
- [x] Puerto `Extractor` + adaptador 7zz: listado, progreso, contraseña (cabecera cifrada y archivos cifrados), errores solo de atributos tolerados, verificación tamaño + CRC32
- [x] `Processor`: cola con workers (`EXTRACT_CONCURRENCY`), espacio libre, rechazo de rutas peligrosas antes de extraer y de enlaces después, borrado del comprimido, reanudación tras reinicio, descarte al cancelar
- [x] Archivos sueltos (no comprimidos) pasan directo a revisión (rename desde el store de tus)
- [x] Escáner: agrupado cue/bin (sin distinguir mayúsculas), juegos en carpeta (PS3), ignorados visibles, carpetas envolventes irrelevantes
- [x] Detectores por cabecera (Switch, GC, Wii, WBFS, PBP, PKG, ISO9660 cocido y crudo para PS1/PS2/PS3) + vectores `detection-cases.json` (también contra las consolas reales de la BD)
- [x] API: `progress`/`warning` en el trabajo, `GET /api/jobs/{id}/items`, `POST /api/jobs/{id}/password`; diagnóstico `extractor` en `/api/health`
- [x] Probado con 4 RAR reales de Switch: tipos y versiones detectados por Title ID, Switch confirmado por cabecera; pico de RAM 66 MB
- [x] Comprimidos multiparte (decisión: agrupar automáticamente): `.partN.rar` y numerados (`.7z.001`, `.zip.001`…) se reúnen por nombre en `staging/volumes` (estado `waiting_parts`); cuando 7-Zip puede abrir el primer volumen, ese trabajo se extrae y las demás partes quedan `merged`. Purga a las 24 h y cancelación por parte. Formato antiguo `.r00` rechazado con mensaje claro
- [ ] Probar RAR multiparte con un archivo real (no se pueden crear RAR sin WinRAR; probado con 7z partidos)

## Fase 5 — Commit al contexto `catalog`
Depende de: 2, 4 · Cubre: RF-09 a RF-11
- [x] Nombres (spec §5) + vectores `naming-cases.json` (incluye pistas estilo Redump y errores); carpeta del juego con año o id de IGDB si el título ya está ocupado (por otro juego o por una carpeta creada por SMB)
- [x] Política de duplicados (RF-10): Base, mismo Disco N, Update con la misma versión (sin importar la "v") o cualquier elemento con el mismo nombre de archivo (DLC incluidos); los elementos en la papelera no cuentan
- [x] `POST /api/jobs/{id}/plan` (vista previa: carpeta, nombres, duplicados y elementos actuales) y `POST /api/jobs/{id}/commit`
- [x] Commit con journal en SQLite (`library_operations`) y deshacer: si falla un movimiento o el registro, todo vuelve al staging; al arrancar se deshacen los cambios interrumpidos y el trabajo vuelve a revisión (o queda `done` si sus elementos ya estaban registrados). Nunca se sobrescribe un archivo existente
- [x] Reescritura de las líneas `FILE` de los `.cue` (se escribe una copia; el original del staging no cambia hasta confirmar)
- [x] Papelera mínima (decisión): "Reemplazar" mueve el elemento anterior a `.gameexplorer/trash/<operación>-<elemento>/` y lo marca en la BD (`trashed_at`); listar, restaurar y purgar llegan en la fase 7
- [x] PUID/PGID/UMASK: no hace falta código. Los permisos vienen del usuario del contenedor y de la ACL heredada (hallazgo de la fase 0.5); carpetas 0775 y archivos 0664 para los datasets sin ACL
- [x] Probado con los 7 RAR reales de Switch contra IGDB real: base, update y 3 DLC de Animal Crossing, Inside + update, Rogue Prince + update (~0,4 s por commit, casi todo la consulta a IGDB)

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
- [ ] Contraste de logos de IGDB sobre el fondo carbón: PS2 (azul) y PS3 (gris oscuro) apenas se ven. Definir tratamiento visual o permitir reemplazar el logo de una consola

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
