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
