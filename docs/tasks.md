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
- [x] `gameCount` por consola en `GET /api/consoles`; `GET /api/consoles/{slug}/games`, `GET /api/games?q=` (sin mayúsculas ni acentos) y `GET /api/games/{id}`
- [x] `GET /api/items/{id}/download`: archivo suelto con `http.ServeContent` (Range, If-Range); discos y juegos en carpeta como zip
- [x] `GET /api/games/{id}/download`: zip *store* en streaming (`platform/zipstream`) con `Content-Length` exacto, calculado de antemano y fijado con pruebas contra el escritor real de Go, zip64 incluido
- [x] Probado con la biblioteca de la fase 5: zip de 12 GB con el tamaño anunciado exacto (~112 MB/s en Docker Desktop), archivo de 6,7 GB a ~130 MB/s, Range sobre el final del archivo, memoria plana; zip validado con `7zz t`

## Fase 7 — Re-emparejar, papelera, consolas e integridad
Depende de: 6 · Cubre: RF-24 a RF-26, RF-30, RF-41
- [x] Motor de operaciones común (`catalog/application/engine.go`) para commit, papelera, restaurar y re-emparejar: journal, deshacer, recuperación al arrancar, renombrados que solo cambian mayúsculas (datasets SMB sin distinción de mayúsculas) vía nombre temporal, reescritura de `.cue` con el original guardado en `.gameexplorer/ops` hasta confirmar
- [x] Re-emparejar con vista previa y renombrado reversible; fusión en el juego existente con decisiones de duplicado (decisión)
- [x] Papelera (decisión: queda dentro de `catalog`, porque enviar y restaurar deben ser atómicos con la biblioteca): entradas por elemento o por juego completo, restaurar con "Reemplazar" si el lugar está ocupado, borrar una entrada, vaciar, purga cada hora tras `TRASH_RETENTION_DAYS` y barrido de carpetas huérfanas (respetando deshacer pendientes)
- [x] Consolas: crear desde una plataforma de IGDB, editar (slug solo sin juegos), borrar (solo agregadas y vacías) y reordenar
- [x] Integridad al arrancar, cada hora y a pedido (`POST /api/library/check`); `missingSince`/`missingCount`; "Olvidar" un elemento faltante
- [x] Migración de los elementos reemplazados en la fase 5 a entradas de papelera
- [x] Probado con la biblioteca real: papelera y restaurar (incluido el juego de 12 GB) en ~0,1 s, re-emparejar de ida y vuelta, integridad con un archivo movido por fuera

## Fase 8 — Diseño → código
Depende de: 0 · Cubre: RNF-05 a RNF-07
- [x] `design:design-critique` y `design:accessibility-review` sobre los mockups → [`design-review.md`](./design-review.md) (14 hallazgos WCAG con su corrección)
- [x] Lienzo corregido (decisión: dibujar lo que faltaba): foco visible, bordes a 3:1, objetivos de 44 px, radios reales en IGDB, flechas para reordenar consolas, estados de error en el login, subidas fallidas y en espera de partes, faltantes y "Olvidar", papelera completa con conflicto y confirmación, y dos pantallas nuevas (re-emparejar, estados vacíos y de error)
- [x] `design:design-handoff` → [`design-handoff.md`](./design-handoff.md): tokens, componentes, estados, movimiento, entrada, tamaños de pantalla, casos límite e i18n
- [x] Tokens como `@theme` de Tailwind v4 (`shared/ui/tokens.css`; la paleta por defecto de Tailwind se borra para que solo existan los tokens). Fuentes servidas por la app (`@fontsource`), sin Google
- [x] `shared/ui`: Button, LinkButton, IconButton, FileButton, TextField, SearchField, Chip, ChoiceChips, ChoiceList, SegmentedToggle, Banner, ProgressBar, EmptyState, Dialog, ConfirmDialog, PageHeader, TableBox, HelpBar, Glyph e íconos
- [x] i18n ES/EN (`shared/i18n`): español por defecto, elección recordada, `<html lang>`, tamaños y fechas con `Intl`; pruebas de claves iguales y sin repetir; ESLint prohíbe texto escrito en JSX
- [x] Capa de entrada (`shared/input`): acciones en lugar de teclas, pila de manejadores (el diálogo atiende primero), gamepad con repetición y zona muerta, navegación espacial propia (decisión: en lugar de `norigin-spatial-navigation`, para funcionar con elementos nativos), glifos solo con control conectado
- [x] Logos de IGDB (decisión): siempre en blanco en el carrusel, como ES
- [x] Página de componentes (`app/UiKit.tsx`) como entregable visible hasta la fase 9; revisada en el navegador a 1440 y 375 px, con teclado y en ES/EN

## Fase 9 — Frontend con adaptadores HTTP
Depende de: 7, 8 · Cubre: RF-01 a RF-51 (UI)
- [x] Puertos de la aplicación y adaptadores `http/` por módulo (`catalog`, `ingestion`, `metadata`, `identity`, `system`): las pantallas no saben qué adaptador está activo; los errores llegan como `AppError` por tipo (sin sesión, conflicto, IGDB caído…) y la interfaz muestra su propio texto
- [x] Router (TanStack Router, rutas en español) y datos (TanStack Query). Sesión vencida → login y vuelta a donde estabas
- [x] Pantallas: login, carrusel, consola (lista + detalle, descargas, papelera, «Olvidar»), búsqueda, re-emparejar, subidas (zona de soltar en toda la ventana + panel), revisión, ajustes (consolas, papelera, general)
- [x] Subidas con `tus-js-client` (decisión: en lugar de Uppy, porque la interfaz es propia): 2 a la vez, cola, reintento, reanudar tras recargar eligiendo el mismo archivo, aviso al cerrar la pestaña; progreso en vivo por SSE
- [x] Contrato ampliado: `Console.builtIn` y `detection`, `GameSummary.igdbId` («Ya en tu biblioteca»), `GET /library/check` (última revisión de integridad)
- [x] Prueba real contra la API y la biblioteca de `.dev`: subir, revisar con IGDB, guardar, duplicado con Reemplazar, restaurar con conflicto (intercambio), vista previa de fusión, integridad; a 1280 y 375 px sin desbordes. Hallazgos corregidos: la navegación tras guardar se perdía cuando el evento SSE llegaba antes que la respuesta, y una lista vieja podía pisar un evento en vivo (ahora gana la versión más reciente de cada trabajo)

## Fase 10 — Ajustes de alcance
Depende de: 9 · Cubre: el ajuste del 2026-10-08 (aviso al inicio de `spec.md`): RF-01 a RF-13a, RF-20, RF-21, RF-24 a RF-27, RF-30, RF-40 a RF-42, RF-52, RF-54, RNF-08, §5 y §6
- [ ] **Diseño primero**: mockups nuevos en el lienzo y pasada con `design:design-critique` y `design:accessibility-review`, luego actualizar `design-handoff.md`. Pantallas:
  - formulario de subida (nombre con sugerencias de IGDB y texto libre, consola, elección para varios archivos);
  - datos por archivo de Switch;
  - error de validación con sus cuatro salidas;
  - sección No asignados en el carrusel y su pantalla, con «Asignar»;
  - edición de juego y de archivo, y mover de consola o a No asignados;
  - Ajustes › Consolas (extensiones propias, orden, nombre);
  - Inicio vacío;
  - portada genérica para nombres sin IGDB.
- [ ] **Consolas como módulos**: registro único en Go y en TS con Switch, Wii y PSP (§6). Se quitan GameCube, PS1, PS2 y PS3, la creación y el borrado de consolas (RF-40) y la tabla de consolas pasa a guardar solo orden, nombre visible y extensiones propias. Guía [`adding-a-console.md`](./adding-a-console.md) con los pasos para agregar una consola (escrita para que la siga una IA)
- [ ] **Extensiones**: variables `SWITCH_EXTENSIONS`, `WII_EXTENSIONS`, `PSP_EXTENSIONS` con valores por defecto; extensiones propias desde la web, que solo se quitan si no las usa ningún archivo; coincidencia más larga (`.nkit.iso`) (RF-41)
- [ ] **Sin detección**: se borran los detectores por cabecera, el lector ISO9660, `contracts/detection-cases.json`, la clasificación cue/bin y de juegos en carpeta, `guessTitle` y las sugerencias de tipo/versión por nombre. Se mantiene el reconocimiento de comprimidos por bytes mágicos
- [ ] **Nuevo flujo de subida**: formulario antes de subir, con pre-validación de no comprimidos (RF-03); varios archivos con sus tres modos (RF-03a); descompresión tras el formulario; validación por extensión, descartando el resto (RF-07); error con sus cuatro salidas, incluido cambiar de consola sin volver a descomprimir; datos por archivo de Switch (RF-08); duplicados por nombre final (RF-09)
- [ ] **Nomenclatura nueva** (§5) en Go, TS y `contracts/naming-cases.json`: `[BASE]`, `[UPDATE vX]`, `[DLC nombre]`; Wii/PSP sin etiqueta; extensión doble conservada
- [ ] **Identidad por nombre**: juego = consola + nombre saneado sin distinguir mayúsculas; enlace opcional a IGDB (RF-11). Se quitan la carpeta con año o id de IGDB (antiguo RF-11a) y el re-emparejar (antiguo RF-24, pantalla `/reemparejar`)
- [ ] **Edición** (RF-24): renombrar (IGDB o libre) con fusión; mover a otra consola si todas las extensiones valen allí; mover juego o archivo a No asignados; editar tipo, versión o DLC de un archivo de Switch. Todo con journal
- [ ] **No asignados y escaneo** (RF-26, RF-27): `_unassigned/`; escaneo al arrancar, cada `LIBRARY_SCAN_INTERVAL` y a pedido; mover lo desconocido conservando ruta, solo si no cambió desde el escaneo anterior; borrados por Samba desaparecen; acciones de asignar, descargar, papelera y borrar. Reemplaza al chequeo de integridad y a «Olvidar»
- [ ] **Carrusel** (RF-20): solo consolas con juegos; No asignados al final si tiene archivos; estado vacío. Se borran las carpetas de juego y de consola que quedan vacías (RF-25)
- [ ] **IGDB opcional** (RF-54): la app arranca y funciona sin credenciales
- [ ] **Datos**: se reemplazan las migraciones por un esquema nuevo y se borra `.dev` (no hay nada en producción); `openapi.yaml`, `task gen`, tests de Go y web, `CLAUDE.md` y README EN/ES al día
- [ ] Prueba real con `~/Downloads/PruebaJuegos` (Switch, Wii, PSP) y con archivos agregados, renombrados y borrados por Samba en `.dev/library`

## Fase 11 — Modo demo
Depende de: 10 · Cubre: RF-60 a RF-65
- [ ] Adaptadores `demo/`, catálogo fijo, archivos de ejemplo, descargas simuladas, banner
- [ ] E2E en modo demo (escritorio y perfiles táctiles) en CI
- [ ] Proyecto en Vercel (`apps/web`, `VITE_DATA_SOURCE=demo`)

## Fase 12 — Producción y portfolio
Depende de: 11
- [ ] Dockerfile multi-stage (web → go con embed → alpine + 7zip) y release en GHCR
- [ ] `deploy/truenas/compose.yaml` + guía (datasets, PUID/PGID, veto files para `.gameexplorer`)
- [ ] README EN/ES con capturas/GIF y diagramas de arquitectura

## Hallazgos de pruebas reales
Prueba del 2026-10-08 con 17 archivos reales (~37 GB): 7 `.rar` de Switch (base, updates y DLC de juegos que ya estaban en la biblioteca), 1 `.iso` de PS2, 4 `.iso` de PSP y 5 `.7z`/`.zip` de Wii. Todo subió (~50 MB/s por archivo, 2 a la vez), se descomprimió sin errores y la API se mantuvo en ~100 MB de RAM. La detección acertó en Switch, PS2 y Wii, y los duplicados de Switch (base, update y 3 DLC) se marcaron todos. Se guardaron 9 juegos (PS2, Wii y PSP como consola agregada). La descarga por rango (206) y la descarga completa salieron idénticas al original (MD5) y el zip del juego se generó bien. Después se borró todo.

Estos hallazgos llevaron al ajuste de alcance de la Fase 10:

- [x] **PSP sin soporte** → resuelto por la Fase 10: PSP es una de las tres consolas y no hay detección.
- [x] **Juegos que no están en IGDB** (el mod «GTA - Sindacco Chronicles») → resuelto por la Fase 10: nombre libre (RF-03, RF-11).
- [x] **NKit se perdía en el nombre** (`.nkit.iso` → `.iso`) → resuelto por la Fase 10: extensión por coincidencia más larga (§5).
- [x] **Título sugerido para IGDB (`guessTitle`) con versiones y guiones** → obsoleto: ya no se sugiere el nombre desde el archivo.
- [x] **Búsqueda IGDB sin consola** («Ookami» no encontraba «Ōkami») → obsoleto: las sugerencias siempre se filtran por la consola elegida.
- [x] **Update de Switch sin Title ID** y **nombre de DLC no sugerido** → obsoletos: el usuario indica tipo, versión y DLC (RF-08).
- [ ] **Carpetas vacías** al quedar sin juegos una consola → incluido en la Fase 10 (RF-25).
- [ ] **Carrusel en pantallas medianas.** A ~800 px de ancho, los nombres laterales casi se tocan, y los logos muy anchos (PSP) se ven pequeños porque se ajustan por alto. *(Fase 10, en el diseño del carrusel)*
- [ ] **Descarga con nombre no ASCII.** `Ōkami.zip` solo manda `filename*=utf-8''…`; agregar también `filename="Okami.zip"` como respaldo para clientes antiguos. *(Fase 12)*
