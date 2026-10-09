package consoles

import "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"

// GameCube stores a game as one file ("Metroid Prime.rvz") or, when it has
// several discs, one file per disc ("Resident Evil 4 (Disc 2).iso").
func GameCube() domain.ConsoleDefinition {
	return domain.ConsoleDefinition{
		Slug:              "gc",
		Name:              "Nintendo GameCube",
		IGDBPlatformID:    21,
		ReleaseYear:       2001,
		ExtensionsEnv:     "GC_EXTENSIONS",
		DefaultExtensions: []string{".iso", ".rvz"},
		Kinds:             []domain.ItemKind{domain.KindGame, domain.KindDisc},
		MultipleFiles:     true,
	}
}
