# Fase 0.5 — Resultados del spike de archivos grandes

Fecha: 2026-10-06/07 · Código: [`spikes/large-files`](../spikes/large-files) · Imagen: `ghcr.io/carlossv923/gameexplorer-spike`

## Entorno
- **NAS**: TrueNAS SCALE, Custom App (Docker), dataset de prueba con ACL SMB y app corriendo como `3000:3000`.
- **Cliente**: PC con Windows por Wi-Fi 6 (Realtek 8852BE, enlace de 1,2 Gbps).
- **Datos reales**: 7 `.rar` de Switch (15 GB) de 3 juegos: base, updates y 3 DLC.

## Resultados
| Prueba | Resultado |
|---|---|
| Subida tus (7 archivos, 15 GB) | 83–105 MB/s, al límite del Wi-Fi. La base de Animal Crossing (6,9 GB) en 65 s |
| Reanudación tus | Subida cortada en 167 MB y retomada desde ese offset; archivo completo |
| Extracción 7zz en el NAS | 860–990 MB/s. 6,9 GB en 7,3 s |
| Verificación (tamaño + CRC32) | 9/9 archivos OK. 6,7 GB verificados en 4,3 s |
| Mover a la biblioteca | `rename` instantáneo, sin EXDEV |
| Descarga con Range | 206 con los bytes exactos |
| Descarga completa | 105 MB/s (1,5 GB y 6,7 GB) |
| Zip *store* en streaming | 108 MB/s; válido según `7zz t`, con nombres Unicode |
| RAM del proceso | **Pico de 14,7 MB** durante todo el ciclo (objetivo: < 300 MB) |
| Recuperación tras reinicio | Las subidas se re-registran desde los `.info` de tusd |

**Conclusión**: el stack Go + tusd + 7zz + `http.ServeContent` cumple RNF-01 con mucho margen. El cuello de botella es la red del cliente. Se continúa con el plan (Fase 1).

## Hallazgos que cambian la implementación
1. **7-Zip de las distribuciones sin RAR.** Los paquetes `7zip` de Alpine y Debian main vienen sin el códec RAR, y todos los RAR5 reales fallaban con *Cannot open the file as archive*. → La imagen de producción usa el **binario oficial de 7-Zip** (`7zzs`, versión fijada y verificada por SHA-256).
2. **ACL de TrueNAS: el contenedor no hereda grupos.** El dataset SMB da escritura a un grupo (p. ej. `builtin_users`), pero el contenedor solo tiene su UID/GID principal, y fallaba `mkdir … permission denied`. → La guía de instalación debe incluir una entrada ACL para el usuario de la app (*Modify*, *Inherit*, recursivo). La app reporta el error en la UI en vez de reiniciarse en bucle.
3. **`chmod` prohibido (aclmode restricted).** Los RAR creados en Unix guardan permisos y 7zz falla con *Cannot set file attribute*; los creados en Windows solo guardan el atributo "A". → Se tratan como advertencia los errores que son *solo de atributos*, y siempre se **verifica cada archivo contra el índice del comprimido (tamaño + CRC32)**. Los permisos finales los da la ACL heredada (`rwxrwx---`, uid de la app), no el umask.
4. **Carpetas envolventes.** Algunos comprimidos traen `Juego NSP BASE GAME/archivo.nsp`. → El escáner aplana los directorios que solo contienen un nivel de envoltura.
5. **Nombres reales de Switch.**
   - El Title ID puede ir seguido de región (`[US]`), con espacios (`[01008D9022462800] [v131072]Update 1.0.4`), o no estar (`v-animal_crossing_new_horizons_v2228224.nsp`).
   - La versión es un **código numérico** (`v196608`), a veces acompañado de la versión legible (`[1.0.3]`, `Update 1.0.4`).
   - Aparecen `™` y `꞉` (U+A789, sustituto de `:` en Windows).

   → Vectores actualizados en `contracts/detection-cases.json` y `contracts/naming-cases.json`; reglas en `spec.md` §5 y §6.
6. **Nombres con `[` `]`.** Herramientas como curl los interpretan como patrones; en la app no aplica porque la subida va por tus desde el navegador. → Solo cuidar el escape en scripts y tests.

## Pendiente de confirmar por el usuario
- [ ] Renombrar un archivo de `spike-output/` por Samba y devolverle su nombre (valida que la ACL heredada permite editar desde SMB).
