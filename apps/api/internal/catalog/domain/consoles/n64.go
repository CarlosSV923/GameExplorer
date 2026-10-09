package consoles

import "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"

// N64 stores one ROM per game, named after the game ("Super Mario 64.z64"),
// in any of the three dump formats.
func N64() domain.ConsoleDefinition {
	return domain.ConsoleDefinition{
		Slug:              "n64",
		Name:              "Nintendo 64",
		IGDBPlatformID:    4,
		ReleaseYear:       1996,
		ExtensionsEnv:     "N64_EXTENSIONS",
		DefaultExtensions: []string{".z64", ".n64", ".v64"},
		Kinds:             []domain.ItemKind{domain.KindGame},
	}
}
