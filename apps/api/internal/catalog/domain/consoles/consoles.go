// Package consoles is the registry of the consoles the app supports (RF-40).
// Each console lives in its own file with its rules; adding one is adding a
// file and a line in All. Step by step: docs/adding-a-console.md.
package consoles

import "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"

// All returns every console in its default carousel order: by maker,
// then by year (spec §6).
func All() []domain.ConsoleDefinition {
	return []domain.ConsoleDefinition{
		N64(),
		GameCube(),
		Wii(),
		Switch(),
		PS2(),
		PSP(),
	}
}
