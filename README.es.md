# GameExplorer

[English](README.md)

> **Estado:** desarrollo inicial (fase 0 de [`docs/tasks.md`](docs/tasks.md)). Todavía no hay nada utilizable.

App web autoalojada para un NAS doméstico (TrueNAS SCALE) que permite **subir, descomprimir, clasificar, renombrar, explorar y descargar juegos de emulación** desde cualquier navegador: escritorio, tablet, pantalla táctil o gamepad. La interfaz está inspirada en EmulationStation.

Arrastra un `.rar`, `.zip`, `.7z` o el archivo del juego. GameExplorer lo descomprime, detecta la consola por la cabecera del archivo, te deja elegir el juego en IGDB y lo guarda como `[consola]/[juego]/[archivo]` con nombres consistentes. Maneja juego base, updates, DLC y juegos de varios discos.

## Características (planeadas)
- Subidas reanudables de decenas de gigabytes (protocolo tus), escritas a disco en streaming.
- Detección de consola por bytes mágicos (cabeceras de disco de GameCube/Wii, `SYSTEM.CNF` de PS1/PS2, estructura de carpetas de PS3, Title ID de Switch).
- Aviso de duplicados antes de escribir nada; papelera con restauración.
- Un solo binario de Go con la app React incrustada, desplegado como Custom App de TrueNAS.
- **Demo sin backend** en Vercel: la misma interfaz con adaptadores en el navegador, para probar el flujo completo sin NAS ni juegos.

## Arquitectura
| Capa | Tecnología |
|---|---|
| Backend | Go, `net/http`, tusd, SQLite (modernc + sqlc + goose), 7-Zip |
| Frontend | React, Vite, TypeScript, Tailwind, TanStack Router/Query, Uppy |
| Contrato | OpenAPI 3 (spec-first; el servidor Go y el cliente TS se generan) |
| Diseño | Domain-Driven Design en ambos lados, puertos y adaptadores, vectores de prueba compartidos |

## Desarrollo
Requisitos: Go 1.27+, Node 24+, pnpm, [Task](https://taskfile.dev), golangci-lint v2.

```bash
task install
task dev:api     # http://localhost:8080
task dev:web     # http://localhost:5173 (proxy de /api)
task dev:demo    # solo frontend, backend simulado
task check       # lint + tests + build, igual que CI
```

La configuración se hace con variables de entorno. Ver [`.env.example`](.env.example).

## Aviso legal
GameExplorer no incluye, descarga ni distribuye juegos. Úsalo solo con archivos que tengas derecho a conservar. Los metadatos de juegos provienen de [IGDB](https://www.igdb.com).

Licencia [MIT](LICENSE).
