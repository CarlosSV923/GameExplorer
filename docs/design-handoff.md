# Traspaso de diseño (Fases 8 y 10)

Fuente: lienzo [`/design`](https://claude.ai/artifact/CNf4mdh7jjnMfnjeNFXe56) (9 pantallas, versión 14, ya corregidas según [`design-review.md`](./design-review.md)). Destino: React 19 + Vite + Tailwind v4, componentes en `apps/web/src/shared/ui`.

Regla general: en el código se usan **tokens**, nunca valores sueltos. Si algo no está aquí, se agrega aquí antes de programarlo.

## 1. Tokens

Se definen en `apps/web/src/shared/ui/tokens.css` como `@theme` de Tailwind v4, así que existen a la vez como variables CSS y como utilidades (`bg-surface`, `text-ink-2`…).

### Color
| Token | Valor | Uso |
|---|---|---|
| `bg` | `#38383b` | Fondo de toda la app |
| `surface` | `#2a2a2d` | Paneles, diálogos, listas de resultados |
| `surface-field` | `#2c2c2f` | Campos de búsqueda sobre el fondo |
| `surface-card` | `#343437` | Tarjetas dentro de un panel (subidas) |
| `surface-input` | `#232326` | Campos dentro de un panel; pista del selector de idioma |
| `surface-raised` | `#3a3a3e` | Etiquetas de extensión |
| `bar` | `#0e0e0f` | Barra de ayuda inferior |
| `overlay` | `rgb(18 18 20 / 0.84)` | Fondo de diálogos y de la zona de soltar |
| `line` | `#4a4a4f` | Separadores y bordes de contenedores (decorativos) |
| `control` | `#8a8a8e` | Borde de botones e inputs (3,4:1, WCAG 1.4.11) |
| `ink-1` | `#ffffff` | Texto principal (11,7:1) |
| `ink-2` | `#c9c9ce` | Texto secundario (7,1:1) |
| `ink-3` | `#a3a3a8` | Texto terciario (4,65:1). **Solo** sobre `bg`, `surface*` o `bar` |
| `accent` | `#f2b544` | Acción principal, foco, selección |
| `accent-hover` | `#ffd27a` | Hover de la acción principal y de los enlaces |
| `on-accent` | `#1a1a1a` | Texto sobre `accent`, `danger`, `kind-*` claros (9,5:1) |
| `success` | `#9be29b` | "Listo para revisar", detección que coincide |
| `danger` | `#ff8a78` | Acciones destructivas y errores (5,1:1 sobre `bg`) |
| `warning-surface` / `warning-ink` | `#3f3626` / `#ffe2a6` | Avisos (duplicado, fusión), con borde `accent` |
| `danger-surface` / `danger-ink` | `#4a2620` / `#ffd9d2` | Banner de error grave, con borde `danger` |
| `progress-upload` | `#ffffff` | Barra de subida |
| `progress-extract` | `#8ab4ff` | Barra de extracción (siempre con el texto del estado al lado, WCAG 1.4.1) |

**Chips por tipo de elemento** (un color por tipo, decisión de la revisión):

| Tipo | Fondo | Texto |
|---|---|---|
| Base | `#ffffff` | `on-accent` |
| Update | `#8ab4ff` | `#10203d` (7,8:1) |
| DLC | `accent` | `on-accent` |
| Juego (Wii, PSP) | `ink-2` (`kind-game`) | `on-accent` |
| Juego completo (papelera) | transparente, borde `ink-1` | `ink-1` |
| No asignado (papelera) | `surface-raised` | `ink-1` |

### Tipografía
Familias: **Quicksand** 500/600/700 para la interfaz y **JetBrains Mono** 400/500 para nombres de archivo, rutas, extensiones y atajos. Se sirven desde la app (`@fontsource`), nunca desde Google, para que el navegador no contacte a terceros (RF-20a).

| Token | Tamaño / interlínea | Peso | Uso |
|---|---|---|---|
| `display-xl` | `clamp(40px, 7vw, 76px)` / 1.02, mayúsculas, `tracking .04em` | 700 | Consola actual del carrusel (60 px si el nombre pasa de 14 caracteres) |
| `display` | 44 px / 1, mayúsculas, `tracking .04em` | 700 | `h1` de cada pantalla |
| `count` | 30 px | 500 | "14 juegos", totales de cabecera |
| `title` | 26 px | 700 | Título de diálogo, nombre del juego en el detalle |
| `heading` | 22 px | 700 | Título de panel ("Subidas", "Agregar consola") |
| `label` | 15 px, mayúsculas, `tracking .08em`, `ink-2` | 700 | Encabezado de sección ("Archivos", "Así quedará") |
| `body-lg` | 17 px / 1.5 | 600 | Texto de diálogos, campos grandes |
| `body` | 16 px / 1.5 | 600 | Texto general, botones |
| `body-sm` | 15 px | 600 | Celdas, avisos |
| `caption` | 13–14 px | 600–700 | Metadatos, ayudas |
| `chip` | 12 px, `tracking .04em` | 700 | Chips (mínimo 12 px) |
| `mono` | 13 px (12 en metadatos) | 400 | Archivos y rutas |

### Espaciado, radios y tamaños
| Token | Valor | Uso |
|---|---|---|
| `space-1…8` | 4, 8, 12, 16, 20, 24, 32, 40 px | Gaps y paddings (escala de Tailwind) |
| `page-x` | 40 px (≥1024) · 24 px (≥640) · 16 px | Margen lateral de página |
| `radius-sm` | 6 px | Atajos de teclado, recuadros de logo |
| `radius-md` | 12 px | Inputs, filas de lista, avisos |
| `radius-lg` | 14 px | Contenedores de tabla, tarjetas |
| `radius-xl` | 20 px | Paneles y diálogos |
| `radius-pill` | `9999px` | Botones y chips |
| `control-lg` | 56 px | Login (campo y botón) |
| `control-md` | 48 px | Botones y enlaces estándar |
| `control-sm` | 44 px | Botones de icono, compactos y radios. **Mínimo absoluto** (WCAG 2.5.5) |

### Foco
`:focus-visible` → `outline: 3px solid accent; outline-offset: 2px`. Los inputs ocultos dentro de un `label` (chips de tipo, resultados de IGDB, botones de archivo) muestran el anillo en el `label` con `label:has(> input:focus-visible)`. **Nunca** se quita el foco sin reemplazarlo.

## 2. Componentes (`shared/ui`)

| Componente | Variantes / props | Notas |
|---|---|---|
| `Button` | `variant: primary \| secondary \| danger \| danger-solid`, `size: lg \| md \| sm`, `icon?`, `loading?`, `disabled?` + `disabledReason?` | Píldora. `secondary` y `danger` son contorno (`control` / `danger`). `loading` mantiene el ancho y anuncia "Cargando" |
| `IconButton` | `label` (obligatorio, va en `aria-label`), `tone: default \| danger` | 44×44 |
| `LinkButton` | Igual que `Button`, pero renderiza `<a>` | Para navegar (Cancelar, Revisar) |
| `FileButton` | `multiple`, `accept?`, `onFiles` | `label` + `input[type=file]` superpuesto e invisible; el foco se ve en el `label` |
| `TextField` / `SearchField` | `label` (visible u oculto), `hint?`, `error?` | El error va en `aria-describedby` y pone `aria-invalid`. La búsqueda lleva el ícono de lupa y la tecla `/` como atajo |
| `Chip` | `kind: base \| update \| dlc \| game \| whole \| unassigned \| neutral` | Texto en mayúsculas desde i18n. `game` = archivo único de Wii/PSP |
| `ChoiceChips` | Radios con estilo de chip (tipo de elemento) | `fieldset` + `legend` |
| `ChoiceList` | Radios con estilo de fila (resultados de IGDB) | Reemplaza `role=option`. Seleccionado: fondo `accent` |
| `SegmentedToggle` | Botones con `aria-pressed` (ES/EN) | Pista `surface-input` |
| `Banner` | `tone: warning \| danger \| info`, `live: polite \| assertive` | `role=status` (polite) o `role=alert` (assertive) |
| `Dialog` | `title`, `onClose`, acciones | Modal sobre `overlay`. Atrapa el foco y lo devuelve al cerrar. Esc o B cierran. Acción principal a la derecha |
| `ConfirmDialog` | `tone: danger`, `confirmLabel` | Para acciones irreversibles (vaciar papelera, borrar para siempre); dice cuánto se libera |
| `ProgressBar` | `value`, `label`, `tone: upload \| extract` | `role=progressbar` con `aria-valuenow` |
| `EmptyState` | `icon`, `title`, `body`, `action?` | Centrado; plantilla de `States.dc.html` |
| `PageHeader` | `back?: {to, label}`, `title`, `aside?` | El enlace "Volver" mide 44 px. Separador inferior `line` |
| `TableBox` | — | Contenedor con `overflow-x: auto`, borde `line` y `radius-lg` |
| `HelpBar` | `actions: [{button, label}]` y versión sin control | Con gamepad: glifos. Sin gamepad: botones reales o nada (ver §4) |
| `Glyph` | `A \| B \| X \| Y \| RT \| LB \| RB \| MENU \| DPAD` | Dibujos propios en SVG, sin logos de marcas |

El **carrusel**, la lista con detalle, el formulario de subida y las demás pantallas son componentes de módulo (`modules/<contexto>/ui`, fase 9). Usan estas primitivas.

## 3. Estados e interacción

| Elemento | Estado | Comportamiento |
|---|---|---|
| Botón primario | hover / active | Fondo `accent-hover` / `scale(.98)` |
| Botón secundario | hover | Fondo `rgb(255 255 255 / .08)` |
| Cualquier botón | disabled | Opacidad .45, sin hover. Si hay motivo (p. ej. «Ni Wii ni PSP aceptan .nsp»), el botón usa `aria-disabled="true"` en lugar de `disabled` (sigue enfocable), el motivo va como texto visible cerca y en `aria-describedby`, y el manejador ignora la acción |
| Cualquier botón | loading | Indicador giratorio a la izquierda y etiqueta "Guardando…"; no se puede pulsar dos veces |
| Enlace | hover | `accent-hover` |
| Fila de lista | seleccionada | `aria-current="true"`, fondo `surface-card`, barra `accent` a la izquierda de 4 px |
| Campo | error | Borde `danger` y mensaje debajo (ícono y texto `danger`) |
| Consola lateral del carrusel | normal | Nombre al 78 % de opacidad; año en `ink-2` sólido |

### Movimiento
| Elemento | Disparador | Animación | Duración | Curva |
|---|---|---|---|---|
| Carrusel | Cambio de consola | Desplazamiento lateral y fundido del nombre | 220 ms | `cubic-bezier(.2,.8,.2,1)` |
| Diálogo | Abrir / cerrar | Fundido y `scale(.98 → 1)` | 160 ms | `ease-out` |
| Zona de soltar | Archivo sobre la ventana | Fundido de la superposición | 120 ms | `ease-out` |
| Botón | Hover | Color | 120 ms | `ease-out` |

Con `prefers-reduced-motion: reduce` todas las duraciones bajan a 0 ms.

## 4. Entrada: táctil, mouse, teclado y gamepad

Una capa única en `shared/input` traduce cada dispositivo a **acciones**. Las pantallas se suscriben a acciones, nunca a teclas.

| Acción | Teclado | Gamepad (estándar) | Táctil / mouse |
|---|---|---|---|
| `confirm` | Enter / Espacio | A | Tocar / clic |
| `back` | Esc | B | Botón "Volver" |
| `action1` | — (botón visible) | X | Botón visible |
| `action2` | `/` (buscar) | Y | Botón visible |
| `upload` | — (botón visible) | RT | "Subir juegos" / "Agregar juegos" |
| `menu` | — (botón visible) | Start (≡) | Botón "Menú" |
| `navigate` | Flechas (fuera de campos de texto) | Cruceta y stick izquierdo | Swipe en el carrusel |
| `prev` / `next` | — | LB / RB | — |

- **Detección:** con `gamepadconnected` y `gamepaddisconnected` y lectura por `requestAnimationFrame` (zona muerta 0,5; repetición tras 400 ms y luego cada 120 ms). Los glifos (`HelpBar`, `Glyph`) aparecen **solo mientras haya un control conectado**; sin control se usan botones normales.
- **Navegación espacial:** con la cruceta, el foco pasa al elemento enfocable más cercano en esa dirección (geometría de `getBoundingClientRect`), dentro del diálogo abierto si lo hay.
  - Con las flechas del teclado pasa lo mismo, salvo que el foco esté en un campo de texto, un `select` o un componente que ya manejó la tecla.
  - **Decisión:** se usa una implementación propia y pequeña en lugar de `norigin-spatial-navigation`, porque funciona con elementos nativos sin envolver cada componente.
- **Carrusel:** ←/→ cambian de consola, Inicio y Fin van a los extremos, Enter abre. Swipe horizontal con `touch-action: pan-y` y un umbral de 48 px.
- **Diálogos:** el foco queda atrapado, el primero es el control principal y Esc o B cierran.
- Ninguna acción depende solo de hover ni de arrastrar: reordenar consolas tiene flechas y arrastrar la zona de soltar tiene "o elige archivos".

## 5. Pantallas y tamaños

| Ancho | Cambios |
|---|---|
| ≥ 1280 | Diseño de los mockups (1440) |
| 1024–1279 | Carrusel con 2 consolas a cada lado en lugar de 3 |
| 768–1023 (tablet) | Lista y detalle se apilan: la lista es una pantalla y el detalle otra (`/consolas/:slug/:juegoId`). Datos de los archivos y error de validación en una columna. Panel de subidas debajo de la zona de soltar |
| < 768 (teléfono) | Carrusel con 1 consola a cada lado y el nombre con `clamp`. Tablas dentro de `TableBox` con desplazamiento horizontal. La barra inferior se reparte en dos líneas. Márgenes de 16 px |

Las tablets en vertical y horizontal se prueban con los perfiles táctiles de Playwright (fase 11).

### Rutas (fase 9, ajustadas en la fase 10)
| Ruta | Pantalla |
|---|---|
| `/entrar` | Login (`?redirect=` vuelve a donde se estaba) |
| `/` | Carrusel (`?consola=` recuerda la consola actual) |
| `/buscar?q=` | Resultados por consola (`&juego=` elige el detalle) |
| `/consolas/:slug` y `/consolas/:slug/:juegoId` | Lista y detalle (en < 1024 px, dos páginas) |
| `/no-asignados` | Archivos de `_unassigned/` |
| `/subidas` y `/subidas/:id` | Subidas; en `:id`, los datos de los archivos (Switch) o el error de validación. El formulario de subida es un diálogo sobre la pantalla actual, sin ruta |
| `/ajustes/consolas`, `/ajustes/papelera`, `/ajustes/general` | Ajustes (LB/RB cambian de sección) |

- **Selección por defecto:** en pantallas anchas la consola muestra el detalle del primer juego; en angostas, solo la lista, sin resaltar nada hasta elegir.
- **RT:** abre el selector de archivos del botón de subida de la pantalla. Los navegadores solo lo abren tras un clic, toque o tecla, así que con el control el botón recibe el foco.

## 6. Contenido y casos límite

- **Nombres largos** (juegos, archivos): en listas se cortan con `…` (`text-overflow: ellipsis`) y van completos en `title`; en el detalle se ajustan a varias líneas. Las rutas mono pueden partirse en cualquier carácter (`overflow-wrap: anywhere`).
- **Inglés:** los textos crecen ~30 %. Los botones nunca tienen ancho fijo y las barras hacen `flex-wrap`.
- **Tamaños y fechas:** con `Intl` según el idioma (`22,7 GB` en ES, `22.7 GB` en EN; fechas cortas como "6 oct 2026").
- **Vacíos y errores:** se usan las plantillas de `States.dc.html`:
  - última consola vaciada (al volver ya no está en el carrusel);
  - búsqueda sin resultados;
  - papelera vacía;
  - Inicio vacío (en el propio carrusel);
  - IGDB sin respuesta en el formulario (se sigue con nombre propio; Reintentar);
  - aviso tras «Revisar ahora»;
  - biblioteca no escribible: banner fijo `danger`, que sale de `/api/health`.
- **Carga:** esqueletos con la forma del contenido (filas de lista y portada) después de 300 ms; antes no se muestra nada, para evitar parpadeos.
- **Sin logo de IGDB** (o sin IGDB configurado): el nombre tipográfico. **Con logo:** siempre en blanco (`filter: brightness(0) invert(1)`), con el nombre como `alt`, dentro de una caja de ancho y alto máximos (`object-fit: contain`) para que los logos anchos como el de PSP no queden pequeños.
- **Sin portada** (nombre propio, sin IGDB): portada genérica en `surface-card` con el ícono de mando, el título y la carpeta.
- **Mensajes del backend:** el `detail` de los errores viene en español. El front muestra su propio texto traducido para los casos conocidos (por estado HTTP y operación) y usa `detail` solo como respaldo.

## 7. Idiomas (i18n)

- `i18next` + `react-i18next`, con recursos en `apps/web/src/shared/i18n/{es,en}.json`, empaquetados (sin cargas por red).
- Español por defecto. La elección se guarda en el navegador (`localStorage` `ge.lang`) y actualiza `<html lang>`.
- Claves por pantalla y componente (`trash.empty.title`). Plurales con `_one` / `_other`. **Ningún texto de interfaz va escrito directamente en un componente**; ESLint lo vigila desde esta fase.

## 8. Accesibilidad (resumen operativo)

- Cada pantalla tiene un único `<h1>`. En Inicio es el nombre de la consola actual, con `aria-live="polite"`.
- Landmarks: `header`, `main` y `footer` (la barra de ayuda). Los diálogos usan `role=dialog`, `aria-modal` y `aria-labelledby`.
- Orden de foco: cabecera → contenido → acciones → barra de ayuda.
- El panel de subidas es una región `aria-live="polite"`; los errores graves usan `role=alert`.
- Botones de icono siempre con `aria-label` traducido; los íconos decorativos con `aria-hidden`.

## 9. Fase 10: subir con formulario, editar y No asignados

Mockups: `UploadForm`, `FileDetails`, `ValidationError`, `GameList` (diálogos), `Unassigned`, `Settings` y `Main` en el lienzo. Revisión: [`design-review.md`](./design-review.md) (Fase 10).

### Componentes nuevos
| Componente | Dónde | Notas |
|---|---|---|
| `NameCombobox` | Formulario de subida, Renombrar | Campo + lista de sugerencias de IGDB de la plataforma de la consola elegida. Patrón combobox de ARIA (`aria-activedescendant`, flechas, Enter, Esc). Desde 2 caracteres, con 300 ms de espera; hasta 5 sugerencias con portada y año; la última opción siempre es «Usar «texto» tal cual». **Ninguna sugerencia se elige sola**: Enter sin opción resaltada usa el texto escrito. Al elegir una, aparece «Enlazado a IGDB: nombre»; si después se edita el texto, se desenlaza. Sin IGDB o si falla: sin lista y una nota debajo; el formulario sigue |
| `ConsolePicker` | Formulario, Cambiar consola, error de validación | Radios en tarjetas (nombre + extensiones). Las opciones que no aceptan los archivos van desactivadas con el motivo |
| `UploadModePicker` | Formulario con varios archivos | Tres radios: partes de un comprimido, archivos del mismo juego, juegos distintos. Con «juegos distintos» el formulario muestra «Juego 1 de N» y pasa al siguiente al confirmar |
| `FileTypeChips` | Datos de los archivos, Editar archivo | `ChoiceChips` con Juego base / Update / DLC (colores de `Chip`) |
| `VersionField` | Update | `TextField` con prefijo «v» decorativo (`aria-hidden`); acepta solo dígitos y puntos; si el usuario escribe «v» se quita |
| `PathPreview` | Formulario, datos, diálogos | Texto mono en `success` sobre `surface-input`: la ruta o el nombre final. Con datos incompletos: `ink-3` y «…» en el hueco |
| `ExtensionChip` | Ajustes › Consolas | 44 px de alto. Con candado (de la variable de entorno, sin quitar) o con botón quitar (propia). Si está en uso, quitar es `aria-disabled` y debajo se dice cuántos archivos la usan |
| `MoreMenu` | Detalle de juego y filas de archivo | `aria-haspopup="menu"`, `role=menu`; contiene «Mover a No asignados» |

### Flujos
- **Subir:** elegir o soltar archivos → formulario (diálogo sobre la pantalla actual; consola preseleccionada según la pantalla) → subida → descompresión → si los archivos encajan, `FileDetails` (en Switch pide tipo, versión o DLC de cada archivo; en Wii y PSP solo muestra el archivo y su nombre final para confirmar con «Guardar»); si no, `ValidationError`. El panel de subidas lleva a cada paso pendiente («Completar», «Resolver», contraseña).
- **Error de validación:** primero la salida que conserva el trabajo (cambiar de consola, con las consolas que aceptan los archivos activas), después No asignados, papelera y borrar definitivamente (`ConfirmDialog` con lo que se libera).
- **Editar:** «Renombrar» y «Cambiar consola» abren diálogos con la vista previa del cambio (carpeta y archivos). Si el destino ya tiene un juego con ese nombre, el diálogo avisa que se fusionarán y los choques se resuelven con Reemplazar u Omitir.
- **No asignados:** tabla con origen, cómo llegó, tamaño y acciones (Asignar, Descargar, Papelera, Borrar). «Asignar» abre el formulario con el archivo ya elegido; se desactiva si no es comprimido y ninguna consola acepta su extensión.
- **Ajustes › Consolas:** el nombre visible se guarda al salir del campo o con Enter y muestra «Guardado». Extensión nueva: debe empezar con punto, solo minúsculas, dígitos y puntos (`.nkit.iso`), sin repetir una de la misma consola.

### Gamepad en los formularios
A elige, B cierra o vuelve, X confirma el formulario («Subir», «Guardar»), la cruceta mueve el foco. Los campos de texto usan el teclado en pantalla del sistema.
