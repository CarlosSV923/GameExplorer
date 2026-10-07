package database_test

import (
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
)

func TestOpenMigratesAndSeedsConsoles(t *testing.T) {
	t.Parallel()

	conn, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	var n int
	if err := conn.QueryRowContext(t.Context(), "SELECT count(*) FROM consoles").Scan(&n); err != nil {
		t.Fatalf("count consoles: %v", err)
	}
	if n != 6 {
		t.Fatalf("seeded consoles = %d, want 6", n)
	}

	var journal string
	if err := conn.QueryRowContext(t.Context(), "PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if journal != "wal" {
		t.Errorf("journal_mode = %q, want wal", journal)
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	for range 2 {
		conn, err := database.Open(t.Context(), dir)
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		_ = conn.Close()
	}
}
