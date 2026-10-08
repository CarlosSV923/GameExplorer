# Revisión de diseño (Fase 8)

Revisión de los mockups del lienzo [`/design`](https://claude.ai/artifact/CNf4mdh7jjnMfnjeNFXe56) (7 pantallas, versión del 6 oct 2026) con las skills `design:design-critique` y `design:accessibility-review`. Las correcciones se aplican en el lienzo y se fijan en [`design-handoff.md`](./design-handoff.md).

## Crítica

**Lo que funciona.**
- El carrusel estilo EmulationStation (nombre grande al centro, líneas que se desvanecen) se reconoce al instante.
- La paleta carbón con acento ámbar.
- Los glifos del gamepad solo aparecen con un control conectado.
- Los inputs de archivo nativos para táctil.
- La ruta final "Se guardará como…" en la revisión.
- Botones reales con `aria-label` en los íconos.

**Lo que falta o hay que corregir.**

| Hallazgo | Severidad | Decisión |
|---|---|---|
| No hay flujo de re-emparejar (búsqueda, vista previa, fusión con duplicados) | Crítica | Se dibuja un diálogo |
| La papelera solo tiene "Restaurar": faltan "Borrar para siempre", "Vaciar" y el conflicto al restaurar | Crítica | Se dibujan |
| Elementos faltantes (borrados por SMB) sin representación | Alta | Distintivo, "Olvidar" y "Revisar ahora" |
| Ajustes: reordenar solo arrastrando; no se ve cómo agregar, editar ni borrar una consola | Alta | Subir/bajar, formulario y estados deshabilitados con motivo |
| Faltan estados vacíos y de error (consola vacía, IGDB no configurado, biblioteca no escribible, partes esperando, subida fallida) | Alta | Plantilla de estado y tarjeta de subida por estado |
| Alturas de control dispares (28 a 56 px) y tres grises de texto sin regla | Media | Escala de 3 alturas y tokens `text-1/2/3` |
| Chips de tipo: "Juego completo" se ve igual que "Disco" | Baja | Un color por tipo |
| Logos de IGDB de PS2 y PS3 casi invisibles sobre el carbón | Media | **Decisión:** todos los logos en blanco en el carrusel (como ES) |

## Auditoría de accesibilidad (WCAG 2.1 AA)

**Resumen:** 14 hallazgos · 3 críticos · 7 mayores · 4 menores.

### Perceptible
| # | Hallazgo | Criterio | Severidad | Corrección |
|---|---|---|---|---|
| 1 | Bordes de botones e inputs: `#5a5a5f` 1,70:1 y `#6c6c72` 2,24–2,67:1 | 1.4.11 | Mayor | Borde de control `#8a8a8e` (3,40:1) |
| 2 | Año de las consolas laterales con opacidad .72: 4,49:1 | 1.4.3 | Menor | Color sólido `#c9c9ce` en el año, opacidad solo en el nombre (≥ 6,9:1) |
| 3 | El estado de una subida se distingue solo por el color de la barra (blanca o azul) | 1.4.1 | Mayor | El texto del estado siempre visible ("Subiendo", "Descomprimiendo") junto al color |
| 4 | Chips de 11 px | 1.4.4 | Menor | Mínimo 12 px |

### Operable
| # | Hallazgo | Criterio | Severidad | Corrección |
|---|---|---|---|---|
| 5 | Ninguna pantalla dibuja el foco visible; con gamepad es la única pista de dónde estás | 2.4.7 | **Crítica** | Anillo de foco de 3 px en ámbar `#f2b544` con separación de 2 px, en todo lo interactivo |
| 6 | Resultados de IGDB como `<li role="option">`: no reciben foco ni se activan con teclado o gamepad | 2.1.1 | **Crítica** | Radios dentro de un `fieldset` (o botones) |
| 7 | Reordenar consolas solo arrastrando | 2.1.1 | **Crítica** | Botones "Subir" y "Bajar" por fila; arrastrar queda como extra |
| 8 | Objetivos táctiles por debajo de 44 px: chips de tipo (40), radios de duplicado (36), ES/EN (36), "Volver" (28) | 2.5.5 | Mayor | Mínimo 44 px |
| 9 | El carrusel no define las teclas | 2.1.1 | Mayor | ←/→ cambian de consola, Enter abre, Inicio/Fin van a los extremos |
| 10 | En revisión, los radios ocultos (opacidad 0, 1 px) no muestran el foco | 2.4.7 | Mayor | Foco visible en el `label` vía `:focus-visible` del input |

### Comprensible
| # | Hallazgo | Criterio | Severidad | Corrección |
|---|---|---|---|---|
| 11 | Login sin estado de error (contraseña incorrecta, demasiados intentos) | 3.3.1 | Mayor | Mensaje bajo el campo, `aria-describedby` y `aria-invalid` |
| 12 | Las acciones irreversibles (vaciar papelera, borrar para siempre) no tienen confirmación | 3.3.4 (AAA, buena práctica) | Menor | Diálogo de confirmación con el tamaño a liberar |

### Robusto
| # | Hallazgo | Criterio | Severidad | Corrección |
|---|---|---|---|---|
| 13 | Barras de progreso como `<div>` sin rol | 4.1.2 | Mayor | `<progress>` o `role="progressbar"` con `aria-valuenow` y etiqueta |
| 14 | Los cambios de estado de las subidas no se anuncian | 4.1.3 | Menor | Región `aria-live="polite"` en el panel de subidas |

### Contraste (texto)
| Uso | Color | Fondo | Ratio | AA |
|---|---|---|---|---|
| Texto principal | `#ffffff` | `#38383b` | 11,69 | ✅ |
| Texto secundario | `#c9c9ce` | `#38383b` | 7,08 | ✅ |
| Texto terciario | `#a3a3a8` | `#38383b` | 4,65 | ✅ (no usar sobre superficies más claras que `#38383b`) |
| Botón primario | `#1a1a1a` | `#f2b544` | 9,50 | ✅ |
| Acento como texto | `#f2b544` | `#38383b` | 6,38 | ✅ |
| Peligro | `#ff8a78` | `#38383b` | 5,10 | ✅ |
| Éxito | `#9be29b` | `#343437` | 8,12 | ✅ |
| Update (chip) | `#10203d` | `#8ab4ff` | 7,76 | ✅ |

### Teclado y gamepad
| Elemento | Tab | Enter/Espacio · A | Escape · B | Flechas · cruceta |
|---|---|---|---|---|
| Carrusel | Una parada (la consola actual) | Abre la consola | — | ←/→ cambian de consola |
| Lista de juegos | Una parada | Abre el detalle | Vuelve a consolas | ↑/↓ recorren la lista |
| Diálogos | Foco atrapado dentro | Acción principal | Cierra | Entre controles |
| Barra inferior | Al final del orden | — | — | — |

Gamepad: la cruceta y el stick mueven el foco espacialmente; A = Enter, B = Escape/atrás, X e Y = acciones de la pantalla, RT = subir o agregar juegos.

### Lectores de pantalla
- **Inicio:** falta un `<h1>`; el nombre de la consola actual debe serlo, dentro del enlace.
- **Carrusel:** se anuncia como región con `aria-roledescription="carrusel"`, y cada cambio de consola en una región `aria-live`.
- **Superposición de soltar:** necesita un `<main>`, o un diálogo con su nombre.

# Revisión de diseño (Fase 10)

Pantallas nuevas o rehechas por el ajuste de alcance: Inicio (solo consolas con juegos, No asignados y estado vacío), formulario de subida, datos de los archivos (Switch), error de validación, lista y detalle con edición, No asignados, Ajustes y estados. Se quitaron Revisión y Re-emparejar.

## Crítica

**Impresión general.** El flujo es más corto y predecible: el usuario dice qué es antes de subir y la app solo pide más datos cuando hacen falta (tipos de Switch) o cuando algo no encaja. El mayor riesgo era la densidad de acciones en el detalle del juego.

| Hallazgo | Severidad | Cambio |
|---|---|---|
| El detalle tenía cinco botones del mismo peso (descargar, renombrar, cambiar consola, mover a No asignados, papelera) y cada fila cuatro iconos, uno de ellos ambiguo (carpeta = «mover a No asignados») | 🟡 Moderada | «Mover a No asignados» pasa a un menú **Más**, en el juego y en cada archivo. Quedan visibles las acciones frecuentes |
| «Cambiar consola» abría un diálogo donde todas las opciones estaban desactivadas (un juego de Switch no cabe en Wii ni PSP) | 🟡 Moderada | El botón se desactiva y debajo se explica por qué |
| En No asignados, «Asignar» llevaba a un callejón sin salida con archivos que ninguna consola acepta (`.z64`) | 🟡 Moderada | «Asignar» se desactiva en esos archivos; la fila ya dice «Ninguna consola acepta .z64». Coincide con la pre-validación del formulario (RF-03) |
| Riesgo de enlazar a IGDB un juego equivocado sin darse cuenta | 🟡 Moderada | Regla de traspaso: ninguna sugerencia se elige sola; el enlace se confirma con un aviso «Enlazado a IGDB: …» y se rompe si se edita el texto |
| Los chips de extensiones medían 36 px de alto | 🟢 Menor | Pasan a 44 px |
| Guardado del nombre visible de una consola sin indicar | 🟢 Menor | Se guarda al salir del campo o con Enter, con estado «Guardado» |

**Lo que funciona.** La ruta final se ve en todo momento («Se guardará en…»); el error de validación ofrece primero la salida que conserva el trabajo (cambiar de consola sin volver a descomprimir); las extensiones con candado explican de dónde vienen.

## Auditoría de accesibilidad (WCAG 2.1 AA)

| # | Hallazgo | Criterio | Severidad | Solución |
|---|---|---|---|---|
| 1 | Botones desactivados con explicación («Cambiar consola», «Asignar», quitar una extensión en uso): con `disabled` no reciben foco y la explicación no se anuncia | 4.1.2, 3.3.2 | 🟡 Mayor | Usar `aria-disabled="true"` (sigue enfocable), `aria-describedby` hacia el texto visible y bloquear la acción en el manejador |
| 2 | Combobox del nombre: las opciones son `li` con clic | 2.1.1, 4.1.2 | 🟡 Mayor | Patrón combobox de ARIA: el foco queda en el campo, flechas mueven `aria-activedescendant`, Enter elige, Esc cierra la lista; la opción «Usar … tal cual» siempre es la última |
| 3 | Menú «Más» | 4.1.2 | 🟡 Mayor | `aria-haspopup="menu"`, `role="menu"`/`menuitem`, flechas, Esc devuelve el foco al botón |
| 4 | Diálogos (renombrar, cambiar consola, editar archivo, borrar definitivamente) | 2.4.3 | 🟡 Mayor | Foco atrapado, foco inicial en el primer campo (o en «Cancelar» en el borrado definitivo), Esc cierra, el foco vuelve al botón que lo abrió. El borrado definitivo es `alertdialog` |
| 5 | El prefijo «v» del campo versión es decorativo | 1.3.1 | 🟢 Menor | `aria-hidden` en la «v» y la etiqueta dice «Versión»; la vista previa del nombre final la muestra completa |
| 6 | Escribir con gamepad | 2.1.1 | 🟢 Menor | Los campos de texto usan el teclado en pantalla del sistema; el resto del formulario se completa con la cruceta y A/B/X |

### Contraste (pares nuevos)
| Texto | Fondo | Ratio | Mínimo | ¿Pasa? |
|---|---|---|---|---|
| `#a3a3a8` (notas) | `#2a2a2d` / `#343437` / `#38383b` | 5,70 / 4,94 / 4,65 | 4,5 | ✅ |
| `#ffb3a6` (errores) | `#2a2a2d` / `#343437` | 8,35 / 7,24 | 4,5 | ✅ |
| `#9be29b` (ruta final) | `#232326` | 10,26 | 4,5 | ✅ |
| `#c9c9ce` (modo elegido) | `#3a3327` | 7,56 | 4,5 | ✅ |
| `#ff8a78` (acciones peligrosas) | `#38383b` | 5,10 | 4,5 | ✅ |
| `#10203d` sobre chip Update | `#8ab4ff` | 7,76 | 4,5 | ✅ |
| Bordes de controles `#8a8a8e` | `#2a2a2d` | 4,16 | 3 (1.4.11) | ✅ |

Los controles desactivados (`#4a4a4f`, `#6c6c72`) quedan fuera de 1.4.11; su motivo siempre está escrito al lado.
