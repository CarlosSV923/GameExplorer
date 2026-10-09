# Instalar GameExplorer en TrueNAS SCALE

1. **Datasets.** Usa el dataset de juegos que ya compartes por SMB (la biblioteca) y crea otro pequeño para el estado de la app, por ejemplo `apps/gameexplorer` (SQLite y caché de imágenes).
2. **Permisos.** La app corre como el usuario `3000:3000` (`user:` en el YAML). Dale una entrada ACL propia con **Modify** e **Inherit** en los dos datasets: un contenedor no hereda grupos suplementarios (RNF-03).
3. **App.** En *Apps › Discover Apps › Custom App › Install via YAML*, pon como *Name* `gameexplorer` (TrueNAS solo acepta minúsculas, números y guiones), pega [`compose.yaml`](compose.yaml) y cambia lo marcado con `CHANGE`:
   - las dos rutas `/mnt/...`;
   - `APP_PASSWORD` (o `APP_PASSWORD_HASH`, generado con `task hash-password`);
   - `SESSION_SECRET`: 32 caracteres aleatorios o más;
   - `IGDB_CLIENT_ID` e `IGDB_CLIENT_SECRET` si quieres sugerencias y portadas (opcional).
4. Abre `http://<ip-del-nas>:8080`. Si la app no puede escribir en la biblioteca, lo dice en un banner en vez de reiniciarse.

**Actualizar:** la imagen `ghcr.io/carlossv923/gameexplorer:latest` se publica con cada tag `v*`; en TrueNAS, *Update* o *Pull image* sobre la app.

**SMB:** conviene ocultar `.gameexplorer` (staging y papelera) con `veto files = /.gameexplorer/` en las opciones auxiliares del share. `_unassigned/` sí debe verse: es la sección No asignados.
