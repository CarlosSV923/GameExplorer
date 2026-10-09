package consoles

import "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"

// PSP is the PlayStation Portable: one file per game, named after the game;
// no updates or DLC for now.
func PSP() domain.ConsoleDefinition {
	return domain.ConsoleDefinition{
		Slug:              "psp",
		Name:              "PlayStation Portable",
		IGDBPlatformID:    38,
		ReleaseYear:       2004,
		ExtensionsEnv:     "PSP_EXTENSIONS",
		DefaultExtensions: []string{".iso", ".cso"},
		Kinds:             []domain.ItemKind{domain.KindGame},
	}
}
