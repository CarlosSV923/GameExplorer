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
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain/consoles"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/libraryfs"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
)

type oneGame struct{}

// games known to the fake IGDB: 429 differs from 427 only in letter case.
var games = map[int64]string{427: "Final Fantasy VII", 428: "Final Fantasy VII Remake", 429: "FINAL FANTASY VII"}

func (oneGame) GameByID(_ context.Context, id int64) (application.GameInfo, error) {
	name, ok := games[id]
	if !ok {
		return application.GameInfo{}, application.ErrUnknownGame
	}
	return application.GameInfo{IGDBID: id, Name: name}, nil
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
	consoles      *application.ConsoleService
	files         application.Files
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	db, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	svc, err := application.NewConsoleService(consoles.All(), sqlite.NewConsoleRepository(db), nil)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	fx := fixture{
		root:     root,
		staging:  filepath.Join(root, ".gameexplorer", "staging", "job1"),
		repo:     sqlite.NewLibraryRepository(db),
		consoles: svc,
		files: libraryfs.New(root, filepath.Join(root, ".gameexplorer", "trash"), filepath.Join(root, ".gameexplorer", "ops"),
			filepath.Join(root, "_unassigned")),
	}
	for name, content := range map[string]string{"base.nsp": "base", "update.nsp": "update", "dlc.nsp": "dlc"} {
		write(t, filepath.Join(fx.staging, name), content)
	}
	return fx
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (fx fixture) service(t *testing.T, files application.Files) *application.LibraryService {
	t.Helper()
	return fx.serviceAt(t, files, nil)
}

func (fx fixture) serviceAt(t *testing.T, files application.Files, now func() time.Time) *application.LibraryService {
	t.Helper()
	return application.NewLibraryService(fx.consoles, fx.repo, oneGame{}, files,
		slog.New(slog.NewTextHandler(io.Discard, nil)), now)
}

// pack stores three Switch files as Final Fantasy VII.
func (fx fixture) pack() application.StoreRequest {
	id := int64(427)
	return application.StoreRequest{Source: "job1", Console: "switch", Name: application.GameName{IGDBID: &id}, Files: []application.NewFile{
		{Ref: "base.nsp", Root: fx.staging, Path: "base.nsp", Size: 4, Kind: domain.KindBase},
		{Ref: "update.nsp", Root: fx.staging, Path: "update.nsp", Size: 6, Kind: domain.KindUpdate, Label: "v1.0.2"},
		{Ref: "dlc.nsp", Root: fx.staging, Path: "dlc.nsp", Size: 3, Kind: domain.KindDLC, Label: "Extra"},
	}}
}

func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// assertUntouched checks that the staging area is as uploaded and the
// library has no game folder and no journal.
func (fx fixture) assertUntouched(t *testing.T) {
	t.Helper()
	if got, want := dirNames(t, fx.staging), []string{"base.nsp", "dlc.nsp", "update.nsp"}; !slices.Equal(got, want) {
		t.Fatalf("staging = %v, want %v", got, want)
	}
	if _, err := os.Stat(filepath.Join(fx.root, "switch")); !os.IsNotExist(err) {
		t.Fatalf("console folder left behind: %v", err)
	}
	if ops, _ := fx.repo.Operations(t.Context()); len(ops) != 0 {
		t.Fatalf("journal not cleared: %+v", ops)
	}
	if has, _ := fx.repo.HasItemsFrom(t.Context(), "job1"); has {
		t.Fatal("files recorded for a failed commit")
	}
}

func TestStoreNamesFilesForTheirKind(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	res, err := fx.service(t, fx.files).Store(t.Context(), fx.pack())
	if err != nil || res.Stored != 3 || res.Path != "switch/Final Fantasy VII" {
		t.Fatalf("store = %+v, %v", res, err)
	}
	want := []string{"Final Fantasy VII [BASE].nsp", "Final Fantasy VII [DLC Extra].nsp", "Final Fantasy VII [UPDATE v1.0.2].nsp"}
	if got := dirNames(t, filepath.Join(fx.root, "switch", "Final Fantasy VII")); !slices.Equal(got, want) {
		t.Fatalf("files = %v", got)
	}
	g, _ := fx.repo.GameByID(t.Context(), res.GameID)
	if *g.IGDBID != 427 || g.Items[1].Label != "1.0.2" {
		t.Fatalf("game = %+v (versions are stored without v)", g)
	}
}

func TestStoreUndoesEveryMoveWhenOneFails(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, &flakyFiles{Files: fx.files, failOn: map[int]bool{3: true}})

	if _, err := svc.Store(t.Context(), fx.pack()); !errors.Is(err, errDiskGone) {
		t.Fatalf("err = %v", err)
	}
	fx.assertUntouched(t)

	// The same request succeeds once the disk is back.
	if res, err := fx.service(t, fx.files).Store(t.Context(), fx.pack()); err != nil || res.Stored != 3 {
		t.Fatalf("retry = %+v, %v", res, err)
	}
}

func TestStoreKeepsTheJournalWhenUndoFails(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	// Move 2 fails, then undoing move 1 fails too.
	svc := fx.service(t, &flakyFiles{Files: fx.files, failOn: map[int]bool{2: true, 3: true}})

	if _, err := svc.Store(t.Context(), fx.pack()); !errors.Is(err, application.ErrUndoFailed) {
		t.Fatalf("err = %v", err)
	}
	if ops, _ := fx.repo.Operations(t.Context()); len(ops) != 1 {
		t.Fatalf("journal = %+v, want it kept for the next start", ops)
	}
	if n, err := fx.service(t, fx.files).Recover(t.Context()); n != 1 || err != nil {
		t.Fatalf("recover = %d, %v", n, err)
	}
	fx.assertUntouched(t)
}

func TestRecoverUndoesACommitInterruptedByACrash(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, &flakyFiles{Files: fx.files, crashOn: 3})

	func() {
		defer func() { _ = recover() }()
		_, _ = svc.Store(t.Context(), fx.pack())
		t.Fatal("expected the simulated crash")
	}()
	if _, err := os.Stat(filepath.Join(fx.root, "switch", "Final Fantasy VII", "Final Fantasy VII [BASE].nsp")); err != nil {
		t.Fatalf("the crash should leave moved files behind: %v", err)
	}
	if n, err := fx.service(t, fx.files).Recover(t.Context()); n != 1 || err != nil {
		t.Fatalf("recover = %d, %v", n, err)
	}
	fx.assertUntouched(t)
}

func TestStoreRefusesFilesPutThereOverSMB(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	write(t, filepath.Join(fx.root, "switch", "Final Fantasy VII", "Final Fantasy VII [DLC Extra].nsp"), "mine")

	var rej *application.Rejection
	if _, err := fx.service(t, fx.files).Store(t.Context(), fx.pack()); !errors.As(err, &rej) || rej.Reason != application.RejectConflict {
		t.Fatalf("err = %v, want a conflict", err)
	}
	if got := dirNames(t, fx.staging); len(got) != 3 {
		t.Fatalf("staging = %v", got)
	}
	if b, _ := os.ReadFile(filepath.Join(fx.root, "switch", "Final Fantasy VII", "Final Fantasy VII [DLC Extra].nsp")); string(b) != "mine" {
		t.Fatal("a file created over SMB was overwritten")
	}
}

func TestStoreRejectsWhatTheConsoleDoesNotTake(t *testing.T) {
	t.Parallel()
	fx := newFixture(t)
	svc := fx.service(t, fx.files)
	cases := map[string]application.StoreRequest{
		"no name":         {Console: "switch", Files: []application.NewFile{{Ref: "base.nsp", Root: fx.staging, Path: "base.nsp", Kind: domain.KindBase}}},
		"unknown console": {Console: "ps2", Name: application.GameName{Title: "X"}, Files: []application.NewFile{{Ref: "base.nsp", Root: fx.staging, Path: "base.nsp"}}},
		"wrong extension": {Console: "wii", Name: application.GameName{Title: "X"}, Files: []application.NewFile{{Ref: "base.nsp", Root: fx.staging, Path: "base.nsp"}}},
		"no kind":         {Console: "switch", Name: application.GameName{Title: "X"}, Files: []application.NewFile{{Ref: "base.nsp", Root: fx.staging, Path: "base.nsp"}}},
		"same name twice": {Console: "switch", Name: application.GameName{Title: "X"}, Files: []application.NewFile{
			{Ref: "base.nsp", Root: fx.staging, Path: "base.nsp", Kind: domain.KindBase},
			{Ref: "dlc.nsp", Root: fx.staging, Path: "dlc.nsp", Kind: domain.KindBase},
		}},
	}
	for name, req := range cases {
		var rej *application.Rejection
		if _, err := svc.Plan(t.Context(), req); !errors.As(err, &rej) || rej.Reason != application.RejectInvalid {
			t.Errorf("%s: err = %v, want invalid", name, err)
		}
	}
}
