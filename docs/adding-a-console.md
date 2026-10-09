# Cómo agregar una consola

Las consolas no se crean desde la app: cada una tiene reglas propias (qué archivos acepta, qué datos pide por archivo, cómo se nombran) y vive en el código (RF-40). Esta guía es la receta completa; está escrita para que la siga una persona o una IA sin conocer el resto del sistema.

## Qué define una consola

Todo está en un solo valor, `domain.ConsoleDefinition` ([`apps/api/internal/catalog/domain/console.go`](../apps/api/internal/catalog/domain/console.go)):

| Campo | Para qué sirve | Ejemplo (PSP) |
|---|---|---|
| `Slug` | Carpeta en la biblioteca. Minúsculas, números, `-` o `_`. **No cambia nunca** una vez que hay juegos. | `psp` |
| `Name` | Nombre visible por defecto (el usuario puede cambiarlo en Ajustes). | `PlayStation Portable` |
| `IGDBPlatformID` | Plataforma de IGDB: filtra las sugerencias de nombres y trae el logo y el año. Se busca en [api-docs.igdb.com](https://api-docs.igdb.com/#platform). | `38` |
| `ReleaseYear` | Año que se muestra hasta que IGDB responde. | `2004` |
| `ExtensionsEnv` | Variable de entorno que reemplaza las extensiones por defecto. | `PSP_EXTENSIONS` |
| `DefaultExtensions` | Extensiones aceptadas, en minúsculas y con punto. Pueden tener varios puntos (`.nkit.iso`). | `.iso`, `.cso` |
| `Kinds` | Tipos que puede tener un archivo. `KindGame` solo: un archivo por juego, sin más datos. `KindBase` + `KindUpdate` + `KindDLC`: juego base con updates y DLC. | `KindGame` |
| `MultipleFiles` | Si una subida puede traer varios archivos válidos (solo tiene sentido con base/update/DLC). | `false` |

## Pasos

1. **Crea el archivo de la consola** en [`apps/api/internal/catalog/domain/consoles/`](../apps/api/internal/catalog/domain/consoles/), copiando el más parecido (`wii.go` o `psp.go` para un archivo por juego; `switch.go` para base/update/DLC):

   ```go
   package consoles

   import "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"

   // GameCube stores one file per game, named after the game.
   func GameCube() domain.ConsoleDefinition {
   	return domain.ConsoleDefinition{
   		Slug:              "gc",
   		Name:              "Nintendo GameCube",
   		IGDBPlatformID:    21,
   		ReleaseYear:       2001,
   		ExtensionsEnv:     "GC_EXTENSIONS",
   		DefaultExtensions: []string{".iso", ".rvz", ".nkit.iso"},
   		Kinds:             []domain.ItemKind{domain.KindGame},
   	}
   }
   ```

2. **Regístrala** agregándola a `All()` en [`consoles.go`](../apps/api/internal/catalog/domain/consoles/consoles.go). Su posición es el orden por defecto del carrusel.

3. **Documenta la variable de entorno** en [`.env.example`](../.env.example) (sección de extensiones) y pásala en [`compose.dev.yaml`](../compose.dev.yaml) igual que `PSP_EXTENSIONS`. Si ya existe la guía de TrueNAS (`deploy/truenas/`), agrégala también ahí.

4. **Actualiza la spec**: la tabla de §6 en [`spec.md`](./spec.md) y, si hace falta, RF-40.

5. **Corre `task check`.** `TestRegistryIsConsistent` revisa que el slug, la variable y la plataforma no se repitan, que las extensiones estén normalizadas y que los tipos tengan sentido. No hace falta tocar la base de datos, el contrato HTTP ni la interfaz: todo lee las consolas de `/api/consoles`.

## Cuándo no alcanza con esto

- **Un tipo de archivo nuevo** (por ejemplo, discos numerados): agrega el tipo en [`naming.go`](../apps/api/internal/catalog/domain/naming.go) (`ItemKind`, `Stem` y `CleanLabel`), el valor en el enum `ItemKind` de [`api/openapi.yaml`](../api/openapi.yaml) y casos en [`contracts/naming-cases.json`](../contracts/naming-cases.json). Luego `task gen` y adapta los campos del formulario de datos en la web.
- **Una regla de validación distinta** a «uno o varios archivos con estas extensiones»: hoy la validación es genérica ([`ingestion/domain/item.go`](../apps/api/internal/ingestion/domain/item.go), `Validate`). Una regla nueva va ahí, como un campo más de `ConsoleRule` que la definición llena.
- **Quitar una consola** con juegos: no hay migración automática. Mueve antes sus juegos a No asignados desde la app.
