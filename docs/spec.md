# GameExplorer — Especificación del MVP

> Fuente de verdad de los requisitos. Si una decisión no está aquí, se consulta y se documenta antes de implementarla.
> Estado de trabajo: [`tasks.md`](./tasks.md). Mockups: https://claude.ai/artifact/CNf4mdh7jjnMfnjeNFXe56

## 1. Problema y objetivo

Los juegos de emulación (Switch, Wii, GameCube, PS1, PS2, PS3) se guardan en un TrueNAS SCALE compartido por SMB. Agregar uno exige descargar, descomprimir, comprobar si ya existía, conectarse por SMB, crear carpetas y renombrar; además se olvida qué hay guardado.

GameExplorer es una app web autoalojada, con estética EmulationStation, que desde cualquier navegador (escritorio, tablet, táctil o gamepad) permite subir, descomprimir, clasificar, renombrar, explorar y descargar esos juegos. Es también un proyecto público de portfolio con una demo sin backend.

## 2. Glosario

| Término | Significado |
|---|---|
| Consola | Plataforma con carpeta propia en la biblioteca (`switch/`, `ps2/`…) |
| Juego | Un título de IGDB dentro de una consola: `[slug]/[Juego]/` |
| Elemento (item) | Unidad lógica dentro de un juego: Base, Update, DLC o Disco N |
| Parte | Archivo físico de un elemento (un disco `.cue` + sus `.bin` = 1 elemento, varias partes) |
| Juego en carpeta | Formato que es un directorio (PS3 JB `PS3_GAME/`); se trata como una unidad y no se renombra por dentro |
| Staging | Zona temporal donde se extrae y revisa una subida |

## 3. Requisitos funcionales

### Subida e ingesta
- **RF-01** Subir uno o varios archivos arrastrándolos a cualquier parte de la ventana o con los botones "Subir juegos" (Inicio) y "Agregar juegos" (pantalla de una consola), que abren el selector de archivos nativo. Con gamepad, ambos botones se activan con RT.
- **RF-02** Las subidas son reanudables (protocolo tus) y no tienen límite práctico de tamaño.
- **RF-03** Si el archivo es un comprimido (zip, 7z, rar/rar5, multiparte), se detecta por bytes mágicos y se extrae automáticamente. El comprimido original se borra tras una extracción exitosa.
- **RF-04** Si el comprimido está cifrado, el formulario pide la contraseña. La contraseña no se guarda.
- **RF-05** Antes de extraer se comprueba el espacio libre. Ningún archivo extraído puede salir del directorio de staging (zip-slip) y los symlinks se rechazan.
- **RF-05a** Tras extraer, **cada archivo se verifica contra el índice del comprimido (tamaño + CRC32)**. Los errores de 7zz que son *solo de atributos* (*Cannot set file attribute*, porque los datasets con ACL prohíben `chmod`) se registran como advertencia si la verificación pasa; cualquier otro error falla la extracción. *(Hallazgo de la Fase 0.5.)*
- **RF-06** Lo extraído se clasifica en elementos: se agrupa `.cue` con sus `.bin`, se reconocen juegos en carpeta, **se aplanan las carpetas envolventes** (p. ej. `Juego NSP BASE GAME/archivo.nsp`) y se ignoran archivos basura (`.txt`, `.nfo`, `.url`, `.sfv`, lista configurable). La revisión muestra qué se ignoró.
- **RF-07** Detección de consola por cabecera y extensión (reglas en §6). El resultado es una lista de candidatas con nivel de confianza.
- **RF-08** **Consola preseleccionada**: si la subida se inicia desde la pantalla de una consola, esa consola viene preseleccionada. Si la detección indica otra consola sin ambigüedad, aparece el aviso "Usar <consola detectada>" y la consola no se cambia sola. Desde Inicio se preselecciona la detectada; si es ambigua, el campo queda vacío y es obligatorio.
- **RF-09** Formulario de revisión:
  - búsqueda en IGDB filtrada por la plataforma elegida, con portada y año;
  - por cada elemento: tipo (Base/Update/DLC/Disco), versión, nombre del DLC o número de disco;
  - vista previa de la ruta final.
- **RF-10** Duplicados (IGDB id + consola + tipo): al elegir un juego que ya existe se muestran sus elementos actuales. Una Base, un Disco N o un Update con la misma versión ya existentes cuentan como duplicado, con las opciones **Reemplazar** (el anterior va a la papelera) u **Omitir**.
- **RF-11** Al confirmar, la app crea o reutiliza el juego, mueve los archivos a `[slug]/[Juego]/[archivo]` con los nombres de §5, reescribe las referencias `FILE` de los `.cue` y limpia el staging. La operación se puede revertir si falla a mitad.
- **RF-12** Las subidas abandonadas y el staging huérfano se purgan tras 24 h.
- **RF-13** El progreso (subida, extracción, commit) se ve en vivo en un panel persistente.

### Biblioteca
- **RF-20** Inicio: carrusel de consolas con logo de IGDB (o el nombre en texto si no hay logo), año de lanzamiento y número de juegos. El logo es el del **modelo original** de la consola (la versión lanzada primero), porque el logo general de IGDB suele ser el de la última revisión (p. ej. "Switch OLED Model"). Logo y año se actualizan desde IGDB al arrancar.
- **RF-20a** Búsqueda de juegos en IGDB: solo tipos jugables (juego principal, expansión independiente, remake, remaster, expandido y port; sin bundles, DLC ni packs) y resultados reordenados por coincidencia del nombre (exacto > prefijo > palabra completa). Las imágenes se sirven desde una caché local, de modo que el navegador nunca contacta a terceros.
- **RF-21** Pantalla de consola: lista de juegos y panel de detalle (portada, año, géneros, resumen, carpeta, elementos con tipo y tamaño).
- **RF-22** Búsqueda global en la biblioteca.
- **RF-23** Descargar un elemento (reanudable con HTTP Range) o el juego completo como zip sin compresión. Los juegos en carpeta se descargan siempre como zip.
- **RF-24** Re-emparejar un juego con otro resultado de IGDB, lo que renombra la carpeta y todos los archivos (reversible si falla).
- **RF-25** Enviar un elemento o un juego a la papelera.
- **RF-26** Chequeo periódico de integridad: marca como *faltante* lo borrado por fuera de la app (por SMB).

### Papelera
- **RF-30** Los elementos eliminados se mueven a `.gameexplorer/trash` con su metadata, se pueden restaurar y se purgan tras `TRASH_RETENTION_DAYS` (30 por defecto).

### Consolas
- **RF-40** Vienen precargadas las 6 consolas: Nintendo Switch (`switch`), Wii (`wii`), Nintendo GameCube (`gc`), PlayStation (`psx`), PlayStation 2 (`ps2`) y PlayStation 3 (`ps3`).
- **RF-41** Se pueden agregar consolas buscando la plataforma en IGDB y definiendo slug, nombre visible y extensiones; también se pueden reordenar. Las consolas agregadas se detectan solo por extensión.

### Acceso y preferencias
- **RF-50** Contraseña única, configurada en el entorno como `APP_PASSWORD_HASH` (argon2id, recomendado) o `APP_PASSWORD` (texto plano, se hashea en memoria al arrancar). Todas las operaciones requieren sesión salvo las marcadas como públicas (salud, login, logout). La sesión se recuerda por dispositivo con una cookie httpOnly, firmada y `SameSite=Strict`. El login tiene rate-limit.
- **RF-51** Interfaz en español (por defecto) e inglés. La elección se recuerda en el navegador.

### Modo demo
- **RF-60** Con `VITE_DATA_SOURCE=demo` el frontend funciona sin backend: adaptadores en memoria y estado solo de la sesión (se reinicia al recargar).
- **RF-61** Los datos de IGDB vienen de un catálogo fijo versionado, generado con un comando de Go y credenciales de desarrollo.
- **RF-62** La subida, la extracción, la contraseña, la revisión y el commit se simulan con la misma máquina de estados que el backend.
- **RF-63** Acepta archivos propios: solo lee nombre, tamaño y los primeros KB para la detección por cabecera. El archivo nunca sale del navegador.
- **RF-64** Botón "Probar con archivos de ejemplo" que carga:
  - un `.rar` de Switch con base duplicada, update y DLC;
  - un `.7z` multidisco de PS1;
  - un `.rar` con contraseña (con la contraseña como pista).
- **RF-65** Las descargas generan un `.txt` explicativo. La demo no tiene login y muestra un banner fijo de modo demo.

## 4. Requisitos no funcionales

- **RNF-01a Memoria medida** (Fase 3): proceso de la API en ~40 MB al arrancar y ~60 MB tras logins y subidas de 1,5–6,9 GB; la subida no añade memoria. argon2id usa los parámetros mínimos de OWASP (19 MiB, t=2, p=1), porque con 64 MiB cada login dejaba ~65 MB de heap residente.
- **RNF-01 Archivos grandes**: los bytes de los juegos nunca se cargan en memoria. Se usan streams y `rename`, la extracción la hace `7zz` en un proceso hijo, y las descargas usan `http.ServeContent` o un zip *store* en streaming. Objetivo: saturar la red con RAM estable < 300 MB.
- **RNF-02 Seguridad**: las rutas se construyen siempre en el servidor a partir de IDs y nunca se aceptan rutas del cliente. Protección contra zip-slip, nombres saneados y ningún secreto versionado.
- **RNF-03 Permisos**: la app corre con `PUID/PGID`, y ese usuario necesita una entrada ACL propia en el dataset (*Modify*, *Inherit*), porque un contenedor no hereda grupos suplementarios. Los permisos de lo escrito vienen de la ACL heredada (en datasets con ACL el umask y `chmod` no aplican). Si la app no puede escribir al arrancar, lo muestra en la UI en lugar de reiniciarse en bucle.
- **RNF-03a Extracción**: se usa el **binario oficial de 7-Zip** (versión fijada y SHA-256 verificado), porque los paquetes de las distribuciones vienen sin el códec RAR.
- **RNF-04 Atomicidad**: staging y papelera viven en el mismo dataset que la biblioteca, así que mover es `rename`. Si hay `EXDEV`, se copia y luego se borra.
- **RNF-05 Táctil**:
  - objetivos de 44 px como mínimo;
  - nada que dependa solo de pasar el mouse;
  - swipe en el carrusel;
  - layouts para tablet vertical y horizontal.
- **RNF-06 Entrada**: mouse, táctil, teclado y gamepad. Los glifos del gamepad aparecen solo con un control detectado por la Gamepad API; si no, se usan botones normales.
- **RNF-07 Accesibilidad**: WCAG 2.1 AA (contraste, foco visible, elementos nativos, `aria-label` en botones de solo icono).
- **RNF-08 Arquitectura**: DDD en back y front, contrato OpenAPI spec-first y reglas compartidas fijadas con vectores dorados (`contracts/`).
- **RNF-09 Calidad**: lint, tipos estrictos, tests unitarios, de integración y E2E en CI.
- **RNF-10 Legal**: la app no distribuye ROMs, muestra atribución a IGDB y se publica con licencia MIT.

## 5. Nomenclatura (vectores: `contracts/naming-cases.json`)

| Tipo | Archivo |
|---|---|
| Base | `Juego.ext` |
| Update | `Juego [Update <versión>].ext` |
| DLC | `Juego [DLC] <nombre>.ext` |
| Disco | `Juego (Disc N).ext` |

Saneamiento del título (carpeta y archivos), en este orden:
1. Unicode NFC.
2. `:` y `꞉` (U+A789, sustituto habitual de `:` en Windows) → ` -`. Otros símbolos válidos, como `™`, se conservan.
3. `/` y `\` → espacio.
4. Se eliminan `? < > " | *` y los caracteres de control.
5. Los bloques de espacios en blanco se reducen a uno.
6. Se recortan los espacios y puntos finales y los espacios iniciales.
7. La extensión va en minúsculas.

## 6. Detección (vectores: `contracts/detection-cases.json`)

| Consola | Extensiones | Confirmación por contenido |
|---|---|---|
| Switch | `.nsp .xci .nsz .xcz` | Title ID de 16 hex en el nombre: `…000` Base, `…800` Update, otro DLC. `[vN]` es un código numérico; la versión legible (`[1.0.3]`, `Update 1.0.4`) se extrae si aparece y se sugiere como etiqueta del Update. Puede haber región (`[US]`), espacios entre etiquetas o faltar el Title ID (entonces no se sugiere tipo) |
| GameCube | `.iso .gcm .ciso .rvz` | Magic `0xC2339F3D` en offset `0x1C` |
| Wii | `.iso .wbfs .rvz` | Magic `0x5D1C9EA3` en offset `0x18` |
| PS1 | `.cue .bin .chd .pbp` | ISO9660 → `SYSTEM.CNF` con `BOOT=` |
| PS2 | `.iso .chd .bin .cue` | ISO9660 → `SYSTEM.CNF` con `BOOT2=` |
| PS3 | carpeta JB, `.iso`, `.pkg` | `PS3_GAME/PARAM.SFO` o `PS3_DISC.SFB` |
| Agregadas por el usuario | las configuradas | Solo extensión |

`(Disc N)` en el nombre sugiere el tipo Disco con su número.

## 7. Ambientes

| | Local | Producción (TrueNAS) | Demo (Vercel) |
|---|---|---|---|
| Back | Go (`air` o docker compose) | Contenedor GHCR con la SPA incrustada | — |
| Front | Vite dev con proxy `/api` | Servido por Go | Estático, `VITE_DATA_SOURCE=demo` |
| Datos | `./.dev/` | Datasets `/library` y `/data` | Memoria de la sesión |
| IGDB | Credenciales de desarrollo | Credenciales de producción | Catálogo fijo |

Variables: ver [`.env.example`](../.env.example). Cada push a `main` despliega la demo; los tags `v*` publican la imagen en GHCR.

## 8. Fuera de alcance del MVP

Importar la biblioteca existente · cambiar el tipo o la etiqueta de un elemento · mover elementos entre juegos o consolas · hash para duplicados exactos · IGDB real en la demo · salvapantallas.
