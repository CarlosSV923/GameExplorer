package sqlite_test

import (
	"slices"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
)

func TestConsoleRepositoryListsSeededConsolesInOrder(t *testing.T) {
	t.Parallel()

	conn, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	consoles, err := sqlite.NewConsoleRepository(conn).List(t.Context())
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	var slugs []string
	for _, c := range consoles {
		slugs = append(slugs, string(c.Slug))
	}
	want := []string{"switch", "wii", "gc", "psx", "ps2", "ps3"}
	if !slices.Equal(slugs, want) {
		t.Fatalf("slugs = %v, want %v", slugs, want)
	}

	ps2 := consoles[4]
	if ps2.DisplayName != "PlayStation 2" || ps2.IGDBPlatformID == nil || *ps2.IGDBPlatformID != 8 {
		t.Errorf("ps2 = %+v", ps2)
	}
	if ps2.ReleaseYear == nil || *ps2.ReleaseYear != 2000 {
		t.Errorf("ps2 release year = %v, want 2000", ps2.ReleaseYear)
	}
	if !slices.Contains(ps2.Extensions, ".iso") {
		t.Errorf("ps2 extensions = %v, want .iso", ps2.Extensions)
	}
}
