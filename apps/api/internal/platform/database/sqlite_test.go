package database_test

import (
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
)

func TestOpenMigrates(t *testing.T) {
	t.Parallel()

	conn, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	var n int
	if err := conn.QueryRowContext(t.Context(), "SELECT count(*) FROM games").Scan(&n); err != nil || n != 0 {
		t.Fatalf("games table: %d, %v", n, err)
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
