package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/libraryfs"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
)

type oneGame struct{}

func (oneGame) GameByID(_ context.Context, id int64) (application.GameInfo, error) {
	if id != 427 {
		return application.GameInfo{}, application.ErrUnknownGame
	}
	return application.GameInfo{IGDBID: 427, Name: "Final Fantasy VII"}, nil
}

// flakyFiles fails (or panics, to simulate a crash) on chosen Move calls.
type flakyFiles struct {
	application.Files
	calls   int
	failOn  map[int]bool
	crashOn int
}

var errDiskGone = errors.New("disk gone")

func (f *flakyFiles) Move(from, to string) error {
	f.calls++
	if f.calls == f.crashOn {
		panic("simulated crash")
	}
	if f.failOn[f.calls] {
		return errDiskGone
	}
	return f.Files.Move(from, to)
}

type fixture struct {
	root, staging string
	repo          *sqlite.LibraryRepository
	consoles      *sqlite.ConsoleRepository
	files         application.Files
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	root := t.TempDir()
	fx := fixture{
		root:     root,
		staging:  filepath.Join(root, ".gameexplorer", "staging", "job1"),
		repo:     sqlite.NewLibraryRepository(db),
		consoles: sqlite.NewConsoleRepository(db),
		files:    libraryfs.New(root, filepath.Join(root, ".gameexplorer", "trash")),
	}
	write := func(name, content string) {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(fx.staging, name)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fx.staging, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("ff7.cue", "FILE \"t1.bin\" BINARY\nFILE \"t2.bin\" BINARY\n")
	write("t1.bin", "one")
	write("t2.bin", "two")
	return fx
}

func (fx fixture) service(t *testing.T, files application.Files) *application.LibraryService {
	t.Helper()
	return application.NewLibraryService(fx.consoles, fx.repo, oneGame{}, files,
		slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func (fx fixture) disc() application.StoreRequest {
	return application.StoreRequest{Source: "job1", Console: "psx", IGDBGameID: 427, Items: []application.NewItem{{
		Ref: "ff7.cue", Shape: domain.ShapeDisc, Root: fx.staging, Parts: []string{"ff7.cue", "t1.bin", "t2.bin"},
		Size: 6, Kind: domain.KindDisc, DiscNumber: 1,
	}}}
}

// assertUntouched checks that the staging area is as uploaded and the
// library has no game folder and no journal.
func (fx fixture) assertUntouched(t *testing.T) {
	t.Helper()
	entries, _ := os.ReadDir(fx.staging)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if want := []string{"ff7.cue", "t1.bin", "t2.bin"}; !slices.Equal(names, want) {
		t.Fatalf("staging = %v, want %v", names, want)
	}
	if b, _ := os.ReadFile(filepath.Join(fx.staging, "ff7.cue")); string(b) != "FILE \"t1.bin\" BINARY\nFILE \"t2.bin\" BINARY\n" {
		t.Fatalf("original cue changed: %q", b)
	}
	if _, err := os.Stat(filepath.Join(fx.root, "psx")); !os.IsNotExist(err) {
		t.Fatalf("console folder left behind: %v", err)
	}
	if ops, _ := fx.repo.Operations(t.Context()); len(ops) != 0 {
		t.Fatalf("journal not cleared: %+v", ops)
	}
	if has, _ := fx.repo.HasItemsFrom(t.Context(), "job1"); has {
		t.Fatal("items recorded for a failed commit")
	}
}

func TestStoreUndoesEveryMoveWhenOneFails(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, &flakyFiles{Files: fx.files, failOn: map[int]bool{3: true}})

	if _, err := svc.Store(t.Context(), fx.disc()); !errors.Is(err, errDiskGone) {
		t.Fatalf("err = %v", err)
	}
	fx.assertUntouched(t)

	// The same request succeeds once the disk is back.
	res, err := fx.service(t, fx.files).Store(t.Context(), fx.disc())
	if err != nil || res.Stored != 1 || res.Path != "psx/Final Fantasy VII" {
		t.Fatalf("retry = %+v, %v", res, err)
	}
}

func TestStoreKeepsTheJournalWhenUndoFails(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	// Move 2 fails, then undoing move 1 fails too.
	svc := fx.service(t, &flakyFiles{Files: fx.files, failOn: map[int]bool{2: true, 3: true}})

	if _, err := svc.Store(t.Context(), fx.disc()); !errors.Is(err, application.ErrUndoFailed) {
		t.Fatalf("err = %v", err)
	}
	if ops, _ := fx.repo.Operations(t.Context()); len(ops) != 1 {
		t.Fatalf("journal = %+v, want it kept for the next start", ops)
	}
	if n, err := fx.service(t, fx.files).Recover(t.Context()); n != 1 || err != nil {
		t.Fatalf("recover = %d, %v", n, err)
	}
	_ = os.Remove(filepath.Join(fx.staging, "ff7.cue.gameexplorer-commit")) // see the crash test
	fx.assertUntouched(t)
}

func TestRecoverUndoesACommitInterruptedByACrash(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, &flakyFiles{Files: fx.files, crashOn: 3})

	func() {
		defer func() { _ = recover() }()
		_, _ = svc.Store(t.Context(), fx.disc())
		t.Fatal("expected the simulated crash")
	}()
	if _, err := os.Stat(filepath.Join(fx.root, "psx", "Final Fantasy VII", "Final Fantasy VII (Disc 1) (Track 1).bin")); err != nil {
		t.Fatalf("the crash should leave moved files behind: %v", err)
	}

	if n, err := fx.service(t, fx.files).Recover(t.Context()); n != 1 || err != nil {
		t.Fatalf("recover = %d, %v", n, err)
	}
	// The rewritten cue sheet is a temporary file: it comes back next to the
	// original and is overwritten by the next attempt.
	_ = os.Remove(filepath.Join(fx.staging, "ff7.cue.gameexplorer-commit"))
	fx.assertUntouched(t)
}

func TestStoreRefusesFilesPutThereOverSMB(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, fx.files)
	if _, err := svc.Store(t.Context(), fx.disc()); err != nil {
		t.Fatal(err)
	}

	// A disc 2 whose cue name someone already created by hand.
	gameDir := filepath.Join(fx.root, "psx", "Final Fantasy VII")
	if err := os.WriteFile(filepath.Join(gameDir, "Final Fantasy VII (Disc 2).cue"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fx.staging, "d2.iso"), []byte("disc two"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := application.StoreRequest{Source: "job2", Console: "psx", IGDBGameID: 427, Items: []application.NewItem{{
		Ref: "d2.iso", Shape: domain.ShapeFile, Root: fx.staging, Parts: []string{"d2.iso"}, Kind: domain.KindDisc, DiscNumber: 2,
	}}}
	// Same stem, other extension: no clash.
	if _, err := svc.Store(t.Context(), req); err != nil {
		t.Fatalf("iso next to a foreign cue: %v", err)
	}

	if err := os.WriteFile(filepath.Join(gameDir, "Final Fantasy VII (Disc 3).iso"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fx.staging, "d3.iso"), []byte("disc three"), 0o600); err != nil {
		t.Fatal(err)
	}
	req.Items[0].Ref, req.Items[0].Parts, req.Items[0].DiscNumber = "d3.iso", []string{"d3.iso"}, 3
	var rej *application.Rejection
	if _, err := svc.Store(t.Context(), req); !errors.As(err, &rej) || rej.Reason != application.RejectConflict {
		t.Fatalf("err = %v, want a conflict", err)
	}
	if b, _ := os.ReadFile(filepath.Join(gameDir, "Final Fantasy VII (Disc 3).iso")); string(b) != "mine" {
		t.Fatal("a file created over SMB was overwritten")
	}
}
