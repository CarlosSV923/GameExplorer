# Publicar la demo en Vercel

La demo es el frontend sin backend (`VITE_DATA_SOURCE=demo`, RF-60): no necesita secretos ni servidor. La configuración está en [`vercel.json`](../../vercel.json), en la raíz del repo.

1. En [vercel.com](https://vercel.com) › *Add New… › Project*, importa el repo `CarlosSV923/GameExplorer`.
2. Deja *Root Directory* en la raíz del repo y *Framework Preset* en *Other*: `vercel.json` ya define la instalación (`pnpm install`), el build en modo demo (`VITE_DATA_SOURCE=demo pnpm build:web`), la carpeta publicada (`apps/web/dist`) y las rutas de la SPA.
3. *Deploy*. Cada push a `main` vuelve a publicar la demo.
4. Comprueba que arranca sin login, con el banner «Modo demo», y pasa la URL al README.

Las portadas y logos se cargan del CDN de IGDB (`images.igdb.com`, RF-61); no hace falta configurar nada más.
