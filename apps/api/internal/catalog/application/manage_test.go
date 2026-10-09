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

func TestPurgeExpiredKeepsRecentEntriesAndPendingUndos(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	t0 := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	clock := t0
	svc := fx.serviceAt(t, fx.files, func() time.Time { return clock })
	res, err := svc.Store(t.Context(), fx.pack())
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

func TestEditUndoesEverythingWhenAMoveFails(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	res, err := fx.service(t, fx.files).Store(t.Context(), fx.pack())
	if err != nil {
		t.Fatal(err)
	}
	gameDir := filepath.Join(fx.root, "switch", "Final Fantasy VII")
	before := dirNames(t, gameDir)

	// Moves: the folder, then each file (the third one fails).
	flaky := &flakyFiles{Files: fx.files, failOn: map[int]bool{3: true}}
	remake := int64(428)
	req := application.EditRequest{GameID: res.GameID, Console: "switch", Name: application.GameName{IGDBID: &remake}}
	if _, err := fx.service(t, flaky).Edit(t.Context(), req); !errors.Is(err, errDiskGone) {
		t.Fatalf("err = %v", err)
	}
	if got := dirNames(t, gameDir); !slices.Equal(got, before) {
		t.Fatalf("game folder = %v, want %v", got, before)
	}
	if _, err := os.Stat(filepath.Join(fx.root, "switch", "Final Fantasy VII Remake")); !os.IsNotExist(err) {
		t.Fatalf("new folder left behind: %v", err)
	}
	g, _ := fx.repo.GameByID(t.Context(), res.GameID)
	if *g.IGDBID != 427 || g.Folder != "Final Fantasy VII" || g.Items[0].File != "Final Fantasy VII [BASE].nsp" {
		t.Fatalf("catalog changed: %+v", g)
	}
	if ops, _ := fx.repo.Operations(t.Context()); len(ops) != 0 {
		t.Fatalf("journal left: %+v", ops)
	}

	// The same edit works once the disk is back.
	if _, err := fx.service(t, fx.files).Edit(t.Context(), req); err != nil {
		t.Fatal(err)
	}
	if got := dirNames(t, filepath.Join(fx.root, "switch", "Final Fantasy VII Remake")); len(got) != 3 {
		t.Fatalf("renamed folder = %v", got)
	}
}

// A name that only changes letter case goes through temporary names: on
// case-insensitive datasets the new name "exists" because it is the old one.
func TestEditThatOnlyChangesLetterCase(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, fx.files)
	res, err := svc.Store(t.Context(), fx.pack())
	if err != nil {
		t.Fatal(err)
	}
	upper := int64(429)
	if _, err := svc.Edit(t.Context(), application.EditRequest{GameID: res.GameID, Console: "switch", Name: application.GameName{IGDBID: &upper}}); err != nil {
		t.Fatal(err)
	}
	if got := dirNames(t, filepath.Join(fx.root, "switch")); !slices.Equal(got, []string{"FINAL FANTASY VII"}) {
		t.Fatalf("console folder = %v", got)
	}
	want := []string{"FINAL FANTASY VII [BASE].nsp", "FINAL FANTASY VII [DLC Extra].nsp", "FINAL FANTASY VII [UPDATE v1.0.2].nsp"}
	if got := dirNames(t, filepath.Join(fx.root, "switch", "FINAL FANTASY VII")); !slices.Equal(got, want) {
		t.Fatalf("files = %v", got)
	}
}

func TestPutAsideSanitizesTheFolder(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, fx.files)
	err := svc.PutAside(t.Context(), application.SetAside{
		Source: "job1", Root: fx.staging, Files: []string{"base.nsp"}, Folder: "Zelda: TP/../x", Origin: "z.zip",
		Reason: domain.UnassignedUpload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(fx.root, "_unassigned", "Zelda - TP", "x", "base.nsp")); err != nil {
		t.Fatalf("set aside: %v", err)
	}
}

func TestFlattenConsoleFoldersOfTheUnassignedSection(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, fx.files)
	// What earlier scans left: the console folder kept inside _unassigned/.
	legacy := filepath.Join(fx.root, "_unassigned", "switch", "Zelda", "z.nsp")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("z"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := fx.repo.Apply(t.Context(), "", func(tx domain.LibraryTx) error {
		_, err := tx.InsertUnassigned(t.Context(), domain.UnassignedFile{
			Path: "switch/Zelda/z.nsp", Origin: "switch/Zelda/z.nsp", Reason: domain.UnassignedSamba, Size: 1, ArrivedAt: time.Now(),
		})
		return err
	}); err != nil {
		t.Fatal(err)
	}

	if n, err := svc.FlattenConsoleFolders(t.Context()); n != 1 || err != nil {
		t.Fatalf("flatten = %d, %v", n, err)
	}
	files, err := fx.repo.UnassignedFiles(t.Context())
	if err != nil || len(files) != 1 || files[0].Path != "Zelda/z.nsp" || files[0].Console != "switch" {
		t.Fatalf("files = %+v, %v", files, err)
	}
	if _, err := os.Stat(filepath.Join(fx.root, "_unassigned", "Zelda", "z.nsp")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(fx.root, "_unassigned", "switch")); !os.IsNotExist(err) {
		t.Fatalf("the empty console folder must go: %v", err)
	}
	if n, _ := svc.FlattenConsoleFolders(t.Context()); n != 0 {
		t.Fatalf("second run = %d", n)
	}
}
