# GameExplorer — Especificación del MVP

> Fuente de verdad de los requisitos. Si una decisión no está aquí, se consulta y se documenta antes de implementarla.
> Estado de trabajo: [`tasks.md`](./tasks.md). Mockups: https://claude.ai/artifact/CNf4mdh7jjnMfnjeNFXe56
>
> **Ajuste de alcance (2026-10-08, Fase 10).** Tras probar con archivos reales se simplificó el sistema: tres consolas definidas en código (Switch, Wii, PSP), sin detección automática de consola ni de nombre (el usuario lo indica), IGDB opcional (solo sugiere nombres y aporta portadas), sección **No asignados** y escaneo de lo que llega por Samba. Implementado en la Fase 10.
>
> **Ajustes tras la prueba en el NAS (2026-10-09, Fase 10.5).** `_unassigned/` existe siempre y lo copiado ahí por Samba aparece al momento; No asignados agrupa por carpeta y se asigna una entrada completa de una vez; la consola pasa a ser opcional al subir (sin consola, la subida va a No asignados). Cambian RF-03, RF-07, RF-26 y RF-27; se agregan RF-07b, RF-26a y RF-27a.
>
> **Nuevas consolas (2026-10-09, Fase 11.5).** PlayStation 2, GameCube y Nintendo 64 se suman con las reglas de Wii y PSP (sin DLC ni updates). GameCube y PS2 admiten juegos de varios discos (`Juego (Disc 1).iso`). Cambian §5, §6, RF-07, RF-08, RF-24 y RF-27a, y el orden por defecto de las consolas.

## 1. Problema y objetivo

Los juegos de emulación se guardan en un TrueNAS SCALE compartido por SMB. Agregar uno exige descargar, descomprimir, conectarse por SMB, crear carpetas y renombrar; además se olvida qué hay guardado.

GameExplorer es una app web autoalojada, con estética EmulationStation, que desde cualquier navegador (escritorio, tablet, táctil o gamepad) permite **subir, descomprimir, renombrar y organizar** juegos en carpetas por consola y nombre, y luego explorarlos, editarlos y descargarlos. El usuario decide la consola y el nombre; la app no los adivina. Es también un proyecto público de portfolio con una demo sin backend.

## 2. Glosario

| Término | Significado |
|---|---|
| Consola | Plataforma con carpeta propia en la biblioteca (`n64/`, `gc/`, `wii/`, `switch/`, `ps2/`, `psp/`). Se define en código con sus reglas (§6); no se crea desde la app |
| Juego | Un nombre dentro de una consola: `[slug]/[Juego]/`. Puede estar enlazado a IGDB (portada, año, géneros) o tener un nombre libre |
| Archivo | Un archivo de juego dentro de la carpeta del juego. En Switch tiene tipo (Base, Update o DLC); en GameCube y PS2 es el juego o uno de sus discos; en Nintendo 64, Wii y PSP es el único archivo del juego |
| No asignados | Carpeta `_unassigned/` en la raíz del dataset con archivos que no pertenecen a ningún juego (llegados por Samba, o fallos de validación que el usuario decidió guardar) |
| Staging | Zona temporal donde se sube y descomprime una subida |

## 3. Requisitos funcionales

### Subida
- **RF-01** Subir uno o varios archivos arrastrándolos a cualquier parte de la ventana o con los botones "Subir juegos" (Inicio) y "Agregar juegos" (pantalla de una consola), que abren el selector de archivos nativo. Con gamepad, ambos botones se activan con RT. Las carpetas no se pueden soltar: hay que comprimirlas.
- **RF-02** Las subidas son reanudables (protocolo tus) y no tienen límite práctico de tamaño.
- **RF-03** **Formulario antes de subir.** Al elegir archivos se abre un formulario; la subida no empieza hasta enviarlo. Pide:
  - **Nombre del juego** (obligatorio): mientras se escribe, sugiere juegos de IGDB de la plataforma de la consola elegida. Elegir una sugerencia es opcional: se puede guardar cualquier nombre (mods, traducciones, homebrew). Sin credenciales de IGDB, el campo no sugiere nada.
  - **Consola** (opcional): viene elegida según la pantalla. En el carrusel, la del centro; en la pantalla de una consola, esa consola; en Subidas, Ajustes o No asignados, ninguna. Siempre se puede cambiar o quitar. La lista muestra todas las consolas, incluso las que no tienen juegos. **Sin consola**, la subida va a No asignados (RF-07b) y las sugerencias de IGDB no se filtran por plataforma.
  - Elegir una sugerencia de IGDB es opcional también sin consola; si se elige, queda guardada con la entrada de No asignados y Asignar la precarga.
  - Si un archivo **no es comprimido**, se eligió consola y su extensión no vale para ella, el formulario lo avisa y no deja continuar (evita subir varios GB para nada). Los comprimidos se validan al descomprimir (RF-07).
- **RF-03a** **Varios archivos a la vez.** El formulario pregunta primero cómo tratarlos:
  - **Partes de un mismo comprimido** (`.part1.rar`, `.7z.001`…): un nombre y una consola para todos; cuando todas las partes están subidas, se descomprime desde el primer volumen. Si no forman un conjunto o falta alguna parte, la subida falla con un error claro. El formato antiguo `.r00/.r01` no se admite.
  - **Archivos del mismo juego** (p. ej. una base y un update `.nsp` sueltos): un nombre y una consola para todos; cada archivo se procesa por separado.
  - **Juegos distintos**: un formulario por archivo.
  Si el usuario trata como juegos distintos las partes de un comprimido, su descompresión falla y se muestra el error.
- **RF-04** Si el comprimido está cifrado, se pide la contraseña. La contraseña no se guarda.
- **RF-05** Antes de descomprimir se comprueba el espacio libre. Ningún archivo extraído puede salir del directorio de staging (zip-slip) y los symlinks se rechazan.
- **RF-05a** Tras descomprimir, **cada archivo se verifica contra el índice del comprimido (tamaño + CRC32)**. Los errores de 7zz que son *solo de atributos* (*Cannot set file attribute*, porque los datasets con ACL prohíben `chmod`) se registran como advertencia si la verificación pasa; cualquier otro error falla la extracción. *(Hallazgo de la Fase 0.5.)*
- **RF-06** Un comprimido (zip, 7z, rar/rar5, reconocido por bytes mágicos) se descomprime **después** de enviar el formulario y terminar la subida. El comprimido original se borra tras una extracción exitosa. Un archivo que no es comprimido no se descomprime. La app no intenta adivinar la consola ni el nombre.
- **RF-07** **Validación por extensión** (solo si se eligió consola; sin consola aplica RF-07b). Al terminar la descompresión (o la subida, si no era comprimido) se buscan, en cualquier subcarpeta, los archivos cuya extensión vale para la consola elegida. Todo lo demás (`.txt`, `.nfo`, imágenes, carpetas de instrucciones) **se descarta**.
  - **Nintendo 64, Wii y PSP**: debe haber **exactamente un** archivo válido; con más de uno, es un error de validación.
  - **GameCube y PS2**: uno o varios (los discos de un mismo juego).
  - **Switch**: puede haber uno o varios (p. ej. un comprimido con base, update y DLC).
  - **Sin archivos válidos** (o más de uno en Wii/PSP): se muestra un error con estas opciones:
    - **Cambiar de consola**: si los archivos valen para la nueva consola, se reubican sin volver a descomprimir ni renombrar.
    - **Mandar a No asignados**: los archivos descomprimidos van a `_unassigned/`.
    - **Mandar a la papelera**: restaurable a No asignados.
    - **Borrar definitivamente**.
- **RF-07b** **Subida sin consola.** Tras subir y descomprimir (RF-04 a RF-06), **todo** el contenido se mueve a `_unassigned/<nombre>/` (nombre saneado como el de un juego), incluidos `.txt`, `.nfo` e imágenes, conservando sus subcarpetas. No hay validación ni paso de confirmación. Si `_unassigned/<nombre>/` ya existe, se agrega dentro (sin distinguir mayúsculas); un archivo cuyo nombre ya existe en esa ruta recibe el sufijo ` (2)`, ` (3)`… Con varios archivos (RF-03a), «Partes de un mismo comprimido» y «Archivos del mismo juego» van a la misma entrada; «Juegos distintos», cada uno a la suya.
- **RF-08** **Confirmar y datos por archivo.** Tras la validación siempre hay un paso de confirmación con la vista previa de los nombres finales y el botón Guardar. En **Switch**, ahí se pide para cada archivo válido su **tipo**: Juego base, Update o DLC; el Update exige la **versión** y el DLC exige el **nombre del DLC**. En **GameCube y PS2**, con un solo archivo válido solo se confirma; con varios, cada uno es un **disco** con su número, sugerido y editable (el que traiga el nombre, p. ej. `(Disc 2)`, o 1, 2, 3… por orden de nombre) y sin repetirse. En **Nintendo 64, Wii y PSP** solo se confirma.
- **RF-08a** **Discos de un juego ya guardado (GameCube y PS2).** Si el juego ya tiene un solo disco (`Juego.iso`) y se agrega otro, la confirmación pide el número del nuevo y del existente, y el existente se renombra (`Juego (Disc 1).iso`) en la misma operación revertible. Si ya tiene discos numerados, el nuevo pide su número; repetir uno existente aplica RF-09. Al mandar un disco a la papelera o a No asignados, los demás conservan su nombre.
- **RF-09** **Duplicados.** Si en la carpeta del juego ya existe un archivo con el mismo nombre final (§5), se avisa y se elige **Reemplazar** (el anterior va a la papelera) u **Omitir**. Los archivos en la papelera no cuentan.
- **RF-10** Al confirmar, la app crea o reutiliza la carpeta del juego, mueve los archivos a `[slug]/[Juego]/[archivo]` con los nombres de §5 y limpia el staging. La operación se puede revertir si falla a mitad: los movimientos se anotan antes en un journal y, si algo falla (o la app se reinicia a mitad), se deshacen y la subida vuelve al paso anterior. Nunca se sobrescribe un archivo existente sin la decisión de RF-09. Antes de confirmar se ve una vista previa con los nombres finales y los duplicados.
- **RF-11** **Identidad del juego.** Un juego se identifica por su consola y su nombre saneado, sin distinguir mayúsculas (los shares SMB no las distinguen). Guardar con un nombre que ya existe agrega los archivos a ese juego. Si el nombre se eligió de IGDB, el juego queda enlazado a ese id (portada, año, géneros); con un nombre libre se muestra una portada genérica con el título.
- **RF-12** Las subidas abandonadas y el staging huérfano se purgan tras 24 h.
- **RF-13** El progreso (subida, descompresión, guardado) se ve en vivo en un panel persistente: la pantalla **Subidas** (`/subidas`: zona de soltar + panel) y un indicador «Subidas · N» en la cabecera de todas las pantallas mientras haya subidas en curso o esperando al usuario. Al arrastrar archivos sobre cualquier pantalla aparece la misma zona de soltar con el panel. El panel muestra las subidas activas y las terminadas o fallidas de las últimas 24 h; estas se pueden quitar de la lista (se recuerda en el navegador).
- **RF-13a** Se suben **2 archivos a la vez**; el resto espera en cola. Si se corta la conexión, la subida se reintenta sola; agotados los reintentos, hay un botón Reintentar. Si se recarga o se cierra la pestaña, la subida queda **Interrumpida** con lo recibido, y se continúa eligiendo de nuevo el mismo archivo (mismo nombre y tamaño). Mientras haya subidas en curso, el navegador pide confirmar antes de cerrar la pestaña.

### Biblioteca
- **RF-20** Inicio: carrusel con las consolas **que tienen juegos**, en el orden de Ajustes, con logo de IGDB (o el nombre en texto si no hay logo o no hay IGDB), año de lanzamiento y número de juegos. Al final aparece **No asignados** si tiene archivos. Si no hay nada en ninguna parte, Inicio muestra un estado vacío con "Subir juegos". El logo es el del **modelo original** de la consola y se muestra **siempre en blanco**.
- **RF-20a** Sugerencias de IGDB: solo tipos jugables (juego principal, expansión independiente, remake, remaster, expandido y port; sin bundles, DLC ni packs), filtradas por la plataforma de la consola y ordenadas por coincidencia del nombre (exacto > prefijo > palabra completa). Las imágenes se sirven desde una caché local, de modo que el navegador nunca contacta a terceros.
- **RF-21** Pantalla de consola: lista de juegos por título y panel de detalle (portada, año, géneros y resumen si está enlazado a IGDB; carpeta; archivos con tipo y tamaño). En Switch los archivos se ordenan: base, updates por versión y DLC por nombre. Un juego cuyos archivos están todos en la papelera no aparece.
- **RF-22** Búsqueda global en la biblioteca: cada palabra debe aparecer en el título, sin importar mayúsculas ni acentos ("pokemon" encuentra "Pokémon"). Desde Inicio, Enter lleva a la pantalla de resultados (`/buscar`), agrupados por consola y con el mismo detalle que la pantalla de una consola. En la pantalla de una consola el campo filtra su lista.
- **RF-23** Descargar un archivo (reanudable con HTTP Range) o el juego completo como zip sin compresión. Dentro del zip los archivos van en `<carpeta del juego>/`. El zip se arma mientras se envía, pero anuncia su tamaño exacto (el navegador muestra el progreso). No se puede reanudar. Si falta un archivo en disco, la descarga falla antes de empezar con un mensaje claro.
- **RF-24** **Editar un juego** desde su detalle (todo reversible si falla a mitad):
  - **Renombrar**: con una sugerencia de IGDB (lo enlaza) o un nombre libre (lo desenlaza). Cambia la carpeta y el nombre de todos sus archivos. Si ya existe otro juego con ese nombre en la consola, se fusionan aplicando RF-09 a cada choque.
  - **Mover a otra consola**: solo si **todos** sus archivos tienen extensiones y tipos válidos en la consola destino (p. ej. `.iso` entre Wii, PSP, GameCube y PS2; un juego de varios discos solo entre GameCube y PS2). Si allí existe un juego con el mismo nombre, se fusionan aplicando RF-09.
  - **Mover a No asignados**: el juego completo o un archivo suelto.
  - **Editar un archivo (Switch, GameCube, PS2)**: cambiar su tipo, versión o nombre del DLC (Switch) o su número de disco (GameCube, PS2); el archivo se renombra (con RF-09 si choca).
- **RF-25** Enviar un archivo o un juego a la papelera. Un juego completo es **una sola entrada** (se restaura o borra junta). La carpeta del juego se borra cuando queda vacía, y la de la consola también.

### No asignados y Samba
- **RF-26** **Escaneo de la biblioteca** al arrancar, cada `LIBRARY_SCAN_INTERVAL` (15 min por defecto) y a pedido («Revisar ahora» en Ajustes › General, con la hora de la última revisión). Reemplaza al chequeo de integridad anterior.
  - Un archivo **borrado** por Samba desaparece de la biblioteca; si era el último de su juego, el juego también. No hay aviso ni estado "faltante".
  - Todo archivo que la app no conoce y que está **dentro de una carpeta de consola** o **en la raíz del dataset** (incluidas carpetas que no son de ninguna consola, p. ej. `n64/`) se **mueve a `_unassigned/` quitando la carpeta de consola** y conservando el resto de su ruta: `wii/Zelda/Zelda.iso` → `_unassigned/Zelda/Zelda.iso`; un archivo suelto en `switch/` queda suelto en `_unassigned/`. En la raíz del dataset o en carpetas que no son de ninguna consola, se conserva la ruta tal cual (`n64/Mario.z64` → `_unassigned/n64/Mario.z64`). La consola de origen se guarda como dato de la entrada y precarga Asignar. Si el destino existe, se agrega un sufijo. Se ignoran `.gameexplorer/` y `_unassigned/`.
  - Un archivo solo se mueve si no cambió (tamaño y fecha) desde el escaneo anterior, para no tocar una copia por Samba a medias.
  - Al arrancar, los archivos que escaneos anteriores a la Fase 10.5 dejaron en `_unassigned/<consola>/` salen de esa carpeta igual que arriba, guardando la consola de origen.
  - Renombrar por Samba un archivo de la app equivale a borrarlo y agregar uno desconocido.
- **RF-26a** **`_unassigned/` siempre existe.** La app la crea al arrancar si falta (las carpetas de consola se siguen creando con el primer juego). Lo que se copie por Samba **directamente dentro** de `_unassigned/` no espera al escaneo: la sección lee la carpeta al abrirse y con «Revisar ahora». Un archivo cuyo tamaño o fecha cambió desde la lectura anterior, o modificado hace menos de 1 min, se muestra como **«Copiando…»** y no admite acciones hasta estabilizarse.
- **RF-27** **Sección No asignados** (`_unassigned/`): lista **entradas**. Cada carpeta de primer nivel es una entrada (con su nombre, sus archivos de cualquier subcarpeta, tamaño total y, si se conocen, consola de origen e IGDB); cada archivo suelto en `_unassigned/` es su propia entrada. Se sube a ella eligiendo «sin consola» (RF-07b). Por entrada se puede:
  - **Asignar** (RF-27a).
  - **Descargar** (un archivo suelto, o la carpeta como zip sin compresión, igual que RF-23).
  - **Mandar a la papelera** (la entrada completa es una sola entrada de papelera).
  - **Borrar definitivamente**.
  Dentro de una entrada, cada archivo también se puede descargar, mandar a la papelera o borrar por separado.
- **RF-27a** **Asignar una entrada completa.** El formulario de subida pide una vez nombre, consola (obligatoria aquí) e IGDB, precargados con el nombre de la entrada, su consola de origen y su IGDB si se conocen. Los archivos no se mueven al asignar: la entrada queda «Asignándose», sin acciones, hasta guardar o cancelar (cancelar solo la libera). Los comprimidos de la entrada se descomprimen (RF-04 a RF-06) y se borran al guardar. Luego, en una sola confirmación, se lista cada archivo con extensión válida para la consola y se elige su tipo (Switch: Base, Update con versión, DLC con nombre; GameCube/PS2: el juego o su número de disco; Nintendo 64/Wii/PSP: el archivo del juego) o **No guardar**. Los archivos con extensión no válida aparecen como «No guardar» sin opción a cambiarlo. Aplican RF-09 y RF-10 como una sola operación revertible; lo no guardado se queda en la entrada de No asignados (que desaparece si queda vacía). Nintendo 64, Wii y PSP admiten un solo archivo guardado por juego. Si ningún archivo vale para la consola, aplica el error de RF-07 (cambiar de consola o cancelar).

### Papelera
- **RF-30** Los archivos y juegos eliminados se mueven a `.gameexplorer/trash` con su metadata, se pueden restaurar y se purgan tras `TRASH_RETENTION_DAYS` (30 por defecto). También se puede borrar para siempre una entrada o **vaciar la papelera** (con confirmación en la interfaz). Se restauran a su lugar de origen (juego, o No asignados para lo que vino de allí o de una subida fallida). Si su lugar está ocupado, se pregunta: **Reemplazar** (lo actual va a la papelera, es un intercambio) o **Cancelar**.

### Consolas
- **RF-40** Hay tres consolas, definidas en código con sus reglas: Nintendo Switch (`switch`), Wii (`wii`) y PlayStation Portable (`psp`) (§6). No se pueden crear ni borrar desde la app; agregar una consola es un cambio de código, documentado paso a paso en [`adding-a-console.md`](./adding-a-console.md).
- **RF-41** Extensiones por consola: las de fábrica vienen de una variable de entorno por consola (`SWITCH_EXTENSIONS`, `WII_EXTENSIONS`, `PSP_EXTENSIONS`, separadas por comas); si no se define, se usan los valores de §6. Desde Ajustes › Consolas se pueden **agregar** extensiones propias y **quitar solo esas**, y solo si ningún archivo de la biblioteca las usa (se muestra cuántos). Las de la variable no se pueden quitar desde la app. Una extensión puede tener varios puntos (`.nkit.iso`). **La extensión de un archivo es la más larga que coincide entre las de todas las consolas**: `Juego.nkit.iso` es `.nkit.iso` (solo Wii), aunque PSP acepte `.iso`.
- **RF-42** En Ajustes › Consolas también se puede **reordenar** el carrusel (por defecto Switch, Wii, PSP) y cambiar el **nombre visible**. La carpeta (slug) nunca cambia.

### Acceso y preferencias
- **RF-50** Contraseña única, configurada en el entorno como `APP_PASSWORD_HASH` (argon2id, recomendado) o `APP_PASSWORD` (texto plano, se hashea en memoria al arrancar). Todas las operaciones requieren sesión salvo las marcadas como públicas (salud, login, logout). La sesión se recuerda por dispositivo con una cookie httpOnly, firmada y `SameSite=Strict`. El login tiene rate-limit.
- **RF-51** Interfaz en español (por defecto) e inglés. La elección se recuerda en el navegador.
- **RF-52** El botón **Menú** (Start en el control) abre **Ajustes**, con tres secciones: **Consolas** (extensiones, orden y nombre visible), **Papelera** y **General** (idioma, escaneo de la biblioteca con su última revisión y «Revisar ahora», cerrar sesión y atribución a IGDB).
- **RF-53** Si la sesión vence mientras se usa la app, se vuelve al login y, al entrar, a la pantalla donde se estaba.
- **RF-54** **IGDB es opcional.** Sin `IGDB_CLIENT_ID`/`IGDB_CLIENT_SECRET` todo funciona: el nombre no sugiere nada, no hay portadas y los logos de consola son el nombre en texto. La atribución a IGDB solo aparece si está configurado.

### Modo demo
- **RF-60** Con `VITE_DATA_SOURCE=demo` el frontend funciona sin backend: adaptadores en memoria y estado solo de la sesión (se reinicia al recargar). Arranca con una biblioteca sembrada desde el catálogo fijo: unos 6–8 juegos por consola (Switch con base, update y DLC) y 1–2 entradas en No asignados. «Revisar ahora» no encuentra cambios.
- **RF-61** Las sugerencias de IGDB vienen de un catálogo fijo versionado (las seis consolas; unos 90 juegos), generado con `task demo-catalog` y credenciales de desarrollo. Las portadas y logos se cargan del CDN de IGDB (`images.igdb.com`) con la atribución «Datos de IGDB»: en la demo no aplica la regla de RF-20a de no contactar a terceros.
- **RF-62** La subida, la descompresión, la contraseña, la validación, los datos de Switch y el guardado se simulan con la misma máquina de estados que el backend.
- **RF-63** Acepta archivos propios: solo usa su nombre y tamaño. El archivo nunca sale del navegador. El contenido de un comprimido propio se simula como un único archivo válido para la consola elegida.
- **RF-64** Botón "Probar con archivos de ejemplo" que carga:
  - un `.rar` de Switch con base, update y DLC (pide los tipos tras descomprimir; la base sale duplicada);
  - un `.7z` de Wii con contraseña (con la contraseña como pista);
  - un `.zip` para PSP sin archivos válidos (muestra el error con cambiar de consola / No asignados);
  - un `.zip` sin consola con base, update y un `.txt`, que queda como entrada en No asignados para asignarla entera con «No guardar» (RF-07b, RF-27a).
- **RF-65** Las descargas generan un `.txt` explicativo. La demo no tiene login y muestra un banner fijo de modo demo.

## 4. Requisitos no funcionales

- **RNF-01a Memoria medida** (Fase 3): proceso de la API en ~40 MB al arrancar y ~60 MB tras logins y subidas de 1,5–6,9 GB; la subida no añade memoria. argon2id usa los parámetros mínimos de OWASP (19 MiB, t=2, p=1), porque con 64 MiB cada login dejaba ~65 MB de heap residente. Prueba real del 2026-10-08: ~100 MB con 17 archivos (~37 GB), 2 subidas y una extracción a la vez.
- **RNF-01 Archivos grandes**: los bytes de los juegos nunca se cargan en memoria. Se usan streams y `rename`, la extracción la hace `7zz` en un proceso hijo, y las descargas usan `http.ServeContent` o un zip *store* en streaming. Objetivo: saturar la red con RAM estable < 300 MB.
- **RNF-02 Seguridad**: las rutas se construyen siempre en el servidor a partir de IDs y nunca se aceptan rutas del cliente. Protección contra zip-slip, nombres saneados y ningún secreto versionado.
- **RNF-03 Permisos**: la app corre con `PUID/PGID`, y ese usuario necesita una entrada ACL propia en el dataset (*Modify*, *Inherit*), porque un contenedor no hereda grupos suplementarios. Los permisos de lo escrito vienen de la ACL heredada (en datasets con ACL el umask y `chmod` no aplican). Si la app no puede escribir al arrancar, lo muestra en la UI en lugar de reiniciarse en bucle.
- **RNF-03a Extracción**: se usa el **binario oficial de 7-Zip** (versión fijada y SHA-256 verificado), porque los paquetes de las distribuciones vienen sin el códec RAR.
- **RNF-04 Atomicidad**: staging, papelera y `_unassigned/` viven en el mismo dataset que la biblioteca, así que mover es `rename`. Si hay `EXDEV`, se copia y luego se borra.
- **RNF-05 Táctil**:
  - objetivos de 44 px como mínimo;
  - nada que dependa solo de pasar el mouse;
  - swipe en el carrusel;
  - layouts para tablet vertical y horizontal.
- **RNF-06 Entrada**: mouse, táctil, teclado y gamepad. Los glifos del gamepad aparecen solo con un control detectado por la Gamepad API; si no, se usan botones normales. Las pantallas reaccionan a acciones (confirmar, atrás, navegar…) y no a teclas; la cruceta y las flechas mueven el foco al elemento más cercano en esa dirección. El mapeo completo está en [`design-handoff.md`](./design-handoff.md) §4.
- **RNF-07 Accesibilidad**: WCAG 2.1 AA (contraste, foco visible, elementos nativos, `aria-label` en botones de solo icono).
- **RNF-08 Arquitectura**: DDD en back y front, contrato OpenAPI spec-first y reglas compartidas fijadas con vectores dorados (`contracts/`). **Cada consola es un módulo** con sus reglas (extensiones por defecto, datos por archivo, nombres, validación) registrado en un único lugar en Go y en TS; agregar una consola no debe tocar el resto del sistema (guía en [`adding-a-console.md`](./adding-a-console.md)).
- **RNF-09 Calidad**: lint, tipos estrictos, tests unitarios, de integración y E2E en CI.
- **RNF-10 Legal**: la app no distribuye ROMs, muestra atribución a IGDB cuando lo usa y se publica con licencia MIT.

## 5. Nomenclatura (vectores: `contracts/naming-cases.json`)

| Consola | Tipo | Archivo |
|---|---|---|
| Switch | Juego base | `Juego [BASE].ext` |
| Switch | Update | `Juego [UPDATE v<versión>].ext` |
| Switch | DLC | `Juego [DLC <nombre>].ext` |
| Nintendo 64, Wii, PSP | — | `Juego.ext` |
| GameCube, PS2 | Un solo disco | `Juego.ext` |
| GameCube, PS2 | Disco N (juego de varios discos) | `Juego (Disc N).ext` |

Ejemplos: `Limbo [BASE].xci`, `Limbo [UPDATE v122345].nsp`, `Limbo [DLC Fuga Maestra].nsp`, `Ōkami.nkit.iso`, `Resident Evil 4 (Disc 2).iso` (la convención No-Intro/Redump que reconocen ES-DE y RetroArch).

- **Número de disco**: entero de 1 a 99, sin ceros a la izquierda.

- **Versión del Update**: el usuario escribe solo dígitos y puntos (`1.2.1`, `122345`) y la app antepone `v`. Si escribe `v1.2.1`, la `v` no se duplica.
- **Extensión**: la de RF-41 (la coincidencia más larga entre todas las consolas: `.nkit.iso`, no `.iso`), en minúsculas.

Saneamiento del nombre del juego (carpeta y archivos) y del nombre del DLC, en este orden:
1. Unicode NFC.
2. `:` y `꞉` (U+A789, sustituto habitual de `:` en Windows) → ` -`. Otros símbolos válidos, como `™`, se conservan.
3. `/` y `\` → espacio.
4. Se eliminan `? < > " | *` y los caracteres de control.
5. Los bloques de espacios en blanco se reducen a uno.
6. Se recortan los espacios y puntos finales y los espacios iniciales.

Un nombre que supera 255 bytes se rechaza. La carpeta del juego es su nombre saneado.

## 6. Consolas

| Consola | Slug | Extensiones por defecto | Datos por archivo | Archivos válidos por subida | Plataforma IGDB |
|---|---|---|---|---|---|
| Nintendo 64 | `n64` | `.z64 .n64 .v64` | — | Exactamente uno | 4 |
| GameCube | `gc` | `.iso .rvz` | Número de disco si hay varios | Uno o varios (discos) | 21 |
| Wii | `wii` | `.iso .wbfs .rvz .nkit.iso` | — (sin DLC ni updates por ahora) | Exactamente uno | 5 |
| Nintendo Switch | `switch` | `.nsp .xci` | Tipo (Base / Update / DLC); versión si es Update; nombre si es DLC | Uno o varios | 130 |
| PlayStation 2 | `ps2` | `.iso` | Número de disco si hay varios | Uno o varios (discos) | 8 |
| PlayStation Portable | `psp` | `.iso .cso` | — (sin DLC ni updates por ahora) | Exactamente uno | 38 |

El orden de la tabla es el orden por defecto del carrusel y de Ajustes: por fabricante y, dentro de cada uno, por año. En una instalación que ya guardó su propio orden, las consolas nuevas se agregan al final. Las extensiones se pueden ampliar con la variable de entorno de cada consola o desde Ajustes (RF-41). Como en la Fase 10.5, lo que escaneos anteriores dejaron en `_unassigned/ps2/` o `_unassigned/n64/` sale de esa carpeta al arrancar, ahora que son consolas.

## 7. Ambientes

| | Local | Producción (TrueNAS) | Demo (Vercel) |
|---|---|---|---|
| Back | Go (`air` o docker compose) | Contenedor GHCR con la SPA incrustada | — |
| Front | Vite dev con proxy `/api` | Servido por Go | Estático, `VITE_DATA_SOURCE=demo` |
| Datos | `./.dev/` | Datasets `/library` y `/data` | Memoria de la sesión |
| IGDB | Credenciales de desarrollo (opcional) | Credenciales de producción (opcional) | Catálogo fijo |

Variables: ver [`.env.example`](../.env.example). Cada push a `main` despliega la demo; los tags `v*` publican la imagen en GHCR.

## 8. Fuera de alcance del MVP

Detección automática de consola o nombre · DLC y updates en Wii y PSP · crear consolas desde la app · hash para duplicados exactos · IGDB real en la demo · salvapantallas.
