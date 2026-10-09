package consoles

import "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"

// PS2 stores a game as one file ("Okami.iso") or, when it has several
// discs, one file per disc ("Xenosaga Episode III (Disc 1).iso").
func PS2() domain.ConsoleDefinition {
	return domain.ConsoleDefinition{
		Slug:              "ps2",
		Name:              "PlayStation 2",
		IGDBPlatformID:    8,
		ReleaseYear:       2000,
		ExtensionsEnv:     "PS2_EXTENSIONS",
		DefaultExtensions: []string{".iso"},
		Kinds:             []domain.ItemKind{domain.KindGame, domain.KindDisc},
		MultipleFiles:     true,
	}
}
