# GameExplorer

[English](README.md)

> **Estado:** desarrollo inicial (fases 0–10 de [`docs/tasks.md`](docs/tasks.md) completadas: la app web funciona de punta a punta contra la API para Switch, Wii y PSP: entrar, recorrer el carrusel, subir con formulario y transferencias reanudables, confirmar cada archivo, editar y mover juegos, la sección No asignados alimentada por el escaneo de Samba, y la papelera con restauración; sigue la demo sin backend (fase 11) y la publicación para TrueNAS (fase 12)).

App web autoalojada para un NAS doméstico (TrueNAS SCALE) que permite **subir, descomprimir, clasificar, renombrar, explorar y descargar juegos de emulación** desde cualquier navegador: escritorio, tablet, pantalla táctil o gamepad. La interfaz está inspirada en EmulationStation.

Arrastra un `.rar`, `.zip`, `.7z` o el archivo del juego, indica qué juego y consola es (IGDB sugiere nombres, pero sirve cualquiera) y GameExplorer lo descomprime, comprueba que los archivos valgan para la consola y los guarda como `[consola]/[juego]/[archivo]` con nombres consistentes. En Switch maneja juego base, updates y DLC; Wii y PSP guardan un archivo por juego.

## Características (planeadas)
- Subidas reanudables de decenas de gigabytes (protocolo tus), escritas a disco en streaming.
- Cada consola es un módulo definido en código ([agregar una](docs/adding-a-console.md) no toca nada más); las extensiones vienen de variables de entorno más las tuyas.
- Aviso de duplicados antes de escribir nada; papelera con restauración.
- Lo que se agrega por Samba se detecta: lo desconocido va a una sección «No asignados» para asignarlo después.
- Un solo binario de Go con la app React incrustada, desplegado como Custom App de TrueNAS.
- **Demo sin backend** en Vercel: la misma interfaz con adaptadores en el navegador, para probar el flujo completo sin NAS ni juegos.

## Arquitectura
| Capa | Tecnología |
|---|---|
| Backend | Go, `net/http`, tusd, SQLite (modernc + sqlc + goose), 7-Zip |
| Frontend | React, Vite, TypeScript, Tailwind, TanStack Router/Query, tus-js-client |
| Contrato | OpenAPI 3 (spec-first; el servidor Go y el cliente TS se generan) |
| Diseño | Domain-Driven Design en ambos lados, puertos y adaptadores, vectores de prueba compartidos |

## Desarrollo
Requisitos: Docker, Node 24+, pnpm y [Task](https://taskfile.dev). Las herramientas de Go (tests, lint, generación de código, build) corren en Docker, así que instalar Go es opcional.

```bash
task install
task dev:api     # API de Go en Docker, http://localhost:8080 (contraseña de desarrollo: gameexplorer)
task dev:web     # http://localhost:5173 (proxy de /api)
task dev:demo    # solo frontend, backend simulado
task gen         # regenera el código tras editar api/openapi.yaml o consultas SQL
task check       # lint + tests + build, igual que CI
```

La configuración se hace con variables de entorno. Ver [`.env.example`](.env.example).

## Aviso legal
GameExplorer no incluye, descarga ni distribuye juegos. Úsalo solo con archivos que tengas derecho a conservar. Los metadatos de juegos provienen de [IGDB](https://www.igdb.com).

Licencia [MIT](LICENSE).
