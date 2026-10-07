package application_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func TestPurgeExpiredKeepsRecentEntriesAndPendingUndos(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	t0 := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	clock := t0
	svc := fx.serviceAt(t, fx.files, func() time.Time { return clock })
	res, err := svc.Store(t.Context(), fx.disc())
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.TrashGame(t.Context(), res.GameID); err != nil {
		t.Fatal(err)
	}
	trash := filepath.Join(fx.root, ".gameexplorer", "trash")
	entries := dirNames(t, trash)
	if len(entries) != 1 {
		t.Fatalf("trash = %v", entries)
	}

	// A folder nobody knows about, and one of an operation whose undo is pending.
	_ = os.MkdirAll(filepath.Join(trash, "stray"), 0o750)
	_ = os.MkdirAll(filepath.Join(trash, "pending-1"), 0o750)
	if err := fx.repo.SaveOperation(t.Context(), domain.Operation{ID: "pending", Source: "test", CreatedAt: t0}); err != nil {
		t.Fatal(err)
	}

	clock = t0.Add(29 * 24 * time.Hour)
	if n, err := svc.PurgeExpired(t.Context(), 30*24*time.Hour); n != 0 || err != nil {
		t.Fatalf("purge before expiry = %d, %v", n, err)
	}
	if got, want := dirNames(t, trash), []string{entries[0], "pending-1"}; !slices.Equal(got, want) {
		t.Fatalf("trash after sweep = %v, want %v", got, want)
	}

	clock = t0.Add(31 * 24 * time.Hour)
	if n, err := svc.PurgeExpired(t.Context(), 30*24*time.Hour); n != 1 || err != nil {
		t.Fatalf("purge after expiry = %d, %v", n, err)
	}
	if got := dirNames(t, trash); !slices.Equal(got, []string{"pending-1"}) {
		t.Fatalf("trash after purge = %v", got)
	}
	if _, err := fx.repo.GameByID(t.Context(), res.GameID); !errors.Is(err, domain.ErrGameNotFound) {
		t.Fatalf("a game with nothing left must be deleted: %v", err)
	}
}

func TestRematchUndoesEverythingWhenAMoveFails(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	res, err := fx.service(t, fx.files).Store(t.Context(), fx.disc())
	if err != nil {
		t.Fatal(err)
	}
	gameDir := filepath.Join(fx.root, "psx", "Final Fantasy VII")
	before := dirNames(t, gameDir)

	// Moves: folder, original cue to scratch, rewritten cue, track 1 (fails).
	flaky := &flakyFiles{Files: fx.files, failOn: map[int]bool{4: true}}
	req := application.RematchRequest{GameID: res.GameID, IGDBGameID: 428}
	if _, err := fx.service(t, flaky).Rematch(t.Context(), req); !errors.Is(err, errDiskGone) {
		t.Fatalf("err = %v", err)
	}
	if got := dirNames(t, gameDir); !slices.Equal(got, before) {
		t.Fatalf("game folder = %v, want %v", got, before)
	}
	if _, err := os.Stat(filepath.Join(fx.root, "psx", "Final Fantasy VII Remake")); !os.IsNotExist(err) {
		t.Fatalf("new folder left behind: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(gameDir, "Final Fantasy VII (Disc 1).cue")); string(b) !=
		"FILE \"Final Fantasy VII (Disc 1) (Track 1).bin\" BINARY\nFILE \"Final Fantasy VII (Disc 1) (Track 2).bin\" BINARY\n" {
		t.Fatalf("cue changed: %q", b)
	}
	g, _ := fx.repo.GameByID(t.Context(), res.GameID)
	if g.IGDBID != 427 || g.Folder != "Final Fantasy VII" || g.Items[0].Files[0] != "Final Fantasy VII (Disc 1).cue" {
		t.Fatalf("catalog changed: %+v", g)
	}
	if ops, _ := fx.repo.Operations(t.Context()); len(ops) != 0 {
		t.Fatalf("journal left: %+v", ops)
	}
	if names := dirNames(t, filepath.Join(fx.root, ".gameexplorer", "ops")); len(names) != 0 {
		t.Fatalf("scratch left: %v", names)
	}

	// The same rematch works once the disk is back.
	if _, err := fx.service(t, fx.files).Rematch(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	if got := dirNames(t, filepath.Join(fx.root, "psx", "Final Fantasy VII Remake")); len(got) != 3 {
		t.Fatalf("renamed folder = %v", got)
	}
}

// A title that only changes letter case goes through temporary names: on
// case-insensitive datasets the new name "exists" because it is the old one.
func TestRematchThatOnlyChangesLetterCase(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, fx.files)
	res, err := svc.Store(t.Context(), fx.disc())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Rematch(t.Context(), application.RematchRequest{GameID: res.GameID, IGDBGameID: 429}); err != nil {
		t.Fatal(err)
	}
	got := dirNames(t, filepath.Join(fx.root, "psx"))
	if !slices.Equal(got, []string{"FINAL FANTASY VII"}) {
		t.Fatalf("console folder = %v", got)
	}
	want := []string{"FINAL FANTASY VII (Disc 1) (Track 1).bin", "FINAL FANTASY VII (Disc 1) (Track 2).bin", "FINAL FANTASY VII (Disc 1).cue"}
	if got := dirNames(t, filepath.Join(fx.root, "psx", "FINAL FANTASY VII")); !slices.Equal(got, want) {
		t.Fatalf("files = %v", got)
	}
}
