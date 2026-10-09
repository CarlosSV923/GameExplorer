package consoles

import "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"

// Wii stores one file per game, named after the game ("Ōkami.nkit.iso");
// no updates or DLC for now.
func Wii() domain.ConsoleDefinition {
	return domain.ConsoleDefinition{
		Slug:              "wii",
		Name:              "Wii",
		IGDBPlatformID:    5,
		ReleaseYear:       2006,
		ExtensionsEnv:     "WII_EXTENSIONS",
		DefaultExtensions: []string{".iso", ".wbfs", ".rvz", ".nkit.iso"},
		Kinds:             []domain.ItemKind{domain.KindGame},
	}
}
