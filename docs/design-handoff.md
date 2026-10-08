# Traspaso de diseño (Fase 8)

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
| `warning-surface` / `warning-ink` | `#3f3626` / `#ffe2a6` | Avisos (duplicado, faltante, fusión), con borde `accent` |
| `danger-surface` / `danger-ink` | `#4a2620` / `#ffd9d2` | Banner de error grave, con borde `danger` |
| `progress-upload` | `#ffffff` | Barra de subida |
| `progress-extract` | `#8ab4ff` | Barra de extracción (siempre con el texto del estado al lado, WCAG 1.4.1) |

**Chips por tipo de elemento** (un color por tipo, decisión de la revisión):

| Tipo | Fondo | Texto |
|---|---|---|
| Base | `#ffffff` | `on-accent` |
| Update | `#8ab4ff` | `#10203d` (7,8:1) |
| DLC | `accent` | `on-accent` |
| Disco | `ink-2` | `on-accent` |
| Juego completo (papelera) | transparente, borde `ink-1` | `ink-1` |
| Falta en el disco | transparente, borde `accent` | `accent` |

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
| `Chip` | `kind: base \| update \| dlc \| disc \| whole \| missing \| neutral` | Texto en mayúsculas desde i18n |
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

El **carrusel**, la lista con detalle, la revisión y las demás pantallas son componentes de módulo (`modules/<contexto>/ui`, fase 9). Usan estas primitivas.

## 3. Estados e interacción

| Elemento | Estado | Comportamiento |
|---|---|---|
| Botón primario | hover / active | Fondo `accent-hover` / `scale(.98)` |
| Botón secundario | hover | Fondo `rgb(255 255 255 / .08)` |
| Cualquier botón | disabled | Opacidad .45, sin hover. Si hay motivo (p. ej. "La consola tiene juegos"), va en `aria-describedby` y como texto visible cerca |
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
| 768–1023 (tablet) | Lista y detalle se apilan: la lista es una pantalla y el detalle otra (`/consolas/:slug/:juegoId`). Revisión en una columna. Panel de subidas debajo de la zona de soltar |
| < 768 (teléfono) | Carrusel con 1 consola a cada lado y el nombre con `clamp`. Tablas dentro de `TableBox` con desplazamiento horizontal. La barra inferior se reparte en dos líneas. Márgenes de 16 px |

Las tablets en vertical y horizontal se prueban con los perfiles táctiles de Playwright (fase 11).

### Rutas (fase 9)
| Ruta | Pantalla |
|---|---|
| `/entrar` | Login (`?redirect=` vuelve a donde se estaba) |
| `/` | Carrusel (`?consola=` recuerda la consola actual) |
| `/buscar?q=` | Resultados por consola (`&juego=` elige el detalle) |
| `/consolas/:slug` y `/consolas/:slug/:juegoId` | Lista y detalle (en < 1024 px, dos páginas) |
| `/consolas/:slug/:juegoId/reemparejar` | Cambiar juego IGDB |
| `/subidas` y `/subidas/:id` | Subidas y revisión |
| `/ajustes/consolas`, `/ajustes/papelera`, `/ajustes/general` | Ajustes (LB/RB cambian de sección) |

- **Selección por defecto:** en pantallas anchas la consola muestra el detalle del primer juego; en angostas, solo la lista, sin resaltar nada hasta elegir.
- **RT:** abre el selector de archivos del botón de subida de la pantalla. Los navegadores solo lo abren tras un clic, toque o tecla, así que con el control el botón recibe el foco.

## 6. Contenido y casos límite

- **Nombres largos** (juegos, archivos): en listas se cortan con `…` (`text-overflow: ellipsis`) y van completos en `title`; en el detalle se ajustan a varias líneas. Las rutas mono pueden partirse en cualquier carácter (`overflow-wrap: anywhere`).
- **Inglés:** los textos crecen ~30 %. Los botones nunca tienen ancho fijo y las barras hacen `flex-wrap`.
- **Tamaños y fechas:** con `Intl` según el idioma (`22,7 GB` en ES, `22.7 GB` en EN; fechas cortas como "6 oct 2026").
- **Vacíos y errores:** se usan las plantillas de `States.dc.html`:
  - consola sin juegos;
  - búsqueda sin resultados;
  - papelera vacía;
  - primera vez;
  - IGDB no configurado o sin respuesta (con Reintentar);
  - biblioteca no escribible: banner fijo `danger`, que sale de `/api/health`.
- **Carga:** esqueletos con la forma del contenido (filas de lista y portada) después de 300 ms; antes no se muestra nada, para evitar parpadeos.
- **Sin logo de IGDB:** el nombre tipográfico. **Con logo:** siempre en blanco (`filter: brightness(0) invert(1)`), con el nombre como `alt`.
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
