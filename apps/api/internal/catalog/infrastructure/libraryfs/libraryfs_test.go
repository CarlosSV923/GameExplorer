package libraryfs_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/libraryfs"
)

func TestMoveNeverOverwrites(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	f := libraryfs.New(root, filepath.Join(root, ".trash"), filepath.Join(root, ".ops"))
	a, b := f.LibraryPath("a"), f.LibraryPath("b")
	_ = os.WriteFile(a, []byte("a"), 0o600)
	_ = os.WriteFile(b, []byte("b"), 0o600)

	if err := f.Move(a, b); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("err = %v, want ErrExist", err)
	}
	if got, _ := os.ReadFile(b); string(got) != "b" {
		t.Fatal("destination overwritten")
	}
	if err := f.Move(a, f.LibraryPath("c")); err != nil {
		t.Fatal(err)
	}
}

func TestPathsOutsideTheLibraryAreRefused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	f := libraryfs.New(root, filepath.Join(root, ".trash"), filepath.Join(root, ".ops"))
	outside := filepath.Join(t.TempDir(), "x")
	_ = os.WriteFile(outside, []byte("x"), 0o600)

	if err := f.Move(outside, f.LibraryPath("x")); err == nil {
		t.Error("move from outside accepted")
	}
	if err := f.RemoveAll(outside); err == nil {
		t.Error("remove outside accepted")
	}
	if err := f.RemoveAll(root); err == nil {
		t.Error("removing the whole library accepted")
	}
	if _, err := f.Exists(f.LibraryPath("..", "escape")); err == nil {
		t.Error("parent path accepted")
	}
	if err := f.RemoveEmptyDir(root); err == nil {
		t.Error("removing the library root accepted")
	}
}

func TestMkdirAllReportsWhatItCreated(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	f := libraryfs.New(root, filepath.Join(root, ".trash"), filepath.Join(root, ".ops"))
	_ = os.Mkdir(f.LibraryPath("psx"), 0o750)

	created, err := f.MkdirAll(f.LibraryPath("psx", "Game", "Sub"))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{f.LibraryPath("psx", "Game"), f.LibraryPath("psx", "Game", "Sub")}; !slices.Equal(created, want) {
		t.Fatalf("created = %v, want %v", created, want)
	}

	_ = os.WriteFile(f.LibraryPath("psx", "Game", "file"), nil, 0o600)
	if err := f.RemoveEmptyDir(f.LibraryPath("psx", "Game")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(f.LibraryPath("psx", "Game")); err != nil {
		t.Fatal("a non-empty folder was removed")
	}
}
