package consoles

import "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"

// Switch is the Nintendo Switch: a game is its base plus updates and DLC,
// and one upload may bring several of them ("Limbo [BASE].xci",
// "Limbo [UPDATE v1.0.4].nsp", "Limbo [DLC Fuga Maestra].nsp").
func Switch() domain.ConsoleDefinition {
	return domain.ConsoleDefinition{
		Slug:              "switch",
		Name:              "Nintendo Switch",
		IGDBPlatformID:    130,
		ReleaseYear:       2017,
		ExtensionsEnv:     "SWITCH_EXTENSIONS",
		DefaultExtensions: []string{".nsp", ".xci"},
		Kinds:             []domain.ItemKind{domain.KindBase, domain.KindUpdate, domain.KindDLC},
		MultipleFiles:     true,
	}
}
