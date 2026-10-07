package sevenzip_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sevenzip"
)

// require7zz skips without 7-Zip, except where REQUIRE_7ZZ is set (the dev
// image and CI), so these tests can never silently stop running there.
func require7zz(t *testing.T) *sevenzip.Extractor {
	t.Helper()
	e := sevenzip.New("")
	if !e.Available() {
		if os.Getenv("REQUIRE_7ZZ") != "" {
			t.Fatal("7zz not found but REQUIRE_7ZZ is set")
		}
		t.Skip("7zz not installed")
	}
	return e
}

// makeArchive writes files into a temp dir and packs them with 7zz.
func makeArchive(t *testing.T, name string, files map[string]string, args ...string) string {
	t.Helper()
	src := t.TempDir()
	for p, content := range files {
		full := filepath.Join(src, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	archive := filepath.Join(t.TempDir(), name)
	cmd := exec.CommandContext(t.Context(), "7zz", append(append([]string{"a", "-bso0", "-bsp0"}, args...), archive, ".")...)
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("7zz a: %v\n%s", err, out)
	}
	return archive
}

var gameFiles = map[string]string{
	"Wrapper/INSIDE [0100D2D009028000][v0].nsp":             "PFS0 base game bytes",
	"Wrapper/INSIDE [0100D2D009028800][v196608][1.0.3].nsp": "PFS0 update bytes",
	"Wrapper/leeme.txt": "hola",
}

func TestListAndExtractVerifies(t *testing.T) {
	t.Parallel()
	e := require7zz(t)

	for _, format := range []string{"zip", "7z"} {
		t.Run(format, func(t *testing.T) {
			t.Parallel()
			archive := makeArchive(t, "game."+format, gameFiles, "-t"+format)

			listing, err := e.List(t.Context(), archive, "")
			if err != nil {
				t.Fatal(err)
			}
			if listing.Encrypted() || listing.TotalSize() != int64(len("PFS0 base game bytes")+len("PFS0 update bytes")+4) {
				t.Fatalf("listing = %+v", listing)
			}
			files := 0
			for _, entry := range listing.Entries {
				if !entry.IsDir {
					files++
					if entry.CRC == "" {
						t.Errorf("%s has no CRC", entry.Path)
					}
				}
			}
			if files != 3 {
				t.Fatalf("files in listing = %d", files)
			}

			dest := t.TempDir()
			var last int
			warning, err := e.Extract(t.Context(), archive, dest, "", listing, func(p int) { last = p })
			if err != nil || warning != "" {
				t.Fatalf("Extract: %v (warning %q)", err, warning)
			}
			got, err := os.ReadFile(filepath.Join(dest, "Wrapper", "INSIDE [0100D2D009028000][v0].nsp"))
			if err != nil || string(got) != "PFS0 base game bytes" {
				t.Fatalf("extracted = %q, %v", got, err)
			}
			_ = last // tiny archives may finish before 7zz reports progress
		})
	}
}

func TestHeaderEncryptedArchive(t *testing.T) {
	t.Parallel()
	e := require7zz(t)
	archive := makeArchive(t, "secret.7z", gameFiles, "-t7z", "-pS3cret", "-mhe=on")

	if _, err := e.List(t.Context(), archive, ""); !errors.Is(err, application.ErrPasswordRequired) {
		t.Fatalf("no password: %v", err)
	}
	if _, err := e.List(t.Context(), archive, "nope"); !errors.Is(err, application.ErrWrongPassword) {
		t.Fatalf("wrong password: %v", err)
	}
	listing, err := e.List(t.Context(), archive, "S3cret")
	if err != nil || !listing.Encrypted() {
		t.Fatalf("right password: %+v %v", listing, err)
	}
	if _, err := e.Extract(t.Context(), archive, t.TempDir(), "S3cret", listing, nil); err != nil {
		t.Fatalf("extract: %v", err)
	}
}

func TestFileEncryptedZip(t *testing.T) {
	t.Parallel()
	e := require7zz(t)
	archive := makeArchive(t, "secret.zip", gameFiles, "-tzip", "-pS3cret")

	listing, err := e.List(t.Context(), archive, "")
	if err != nil || !listing.Encrypted() {
		t.Fatalf("names are visible, contents encrypted: %+v %v", listing, err)
	}
	if _, err := e.Extract(t.Context(), archive, t.TempDir(), "nope", listing, nil); !errors.Is(err, application.ErrWrongPassword) {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := e.Extract(t.Context(), archive, t.TempDir(), "S3cret", listing, nil); err != nil {
		t.Fatalf("right password: %v", err)
	}
}

func TestCorruptArchive(t *testing.T) {
	t.Parallel()
	e := require7zz(t)
	archive := makeArchive(t, "game.7z", map[string]string{"a.iso": string(make([]byte, 4096))}, "-t7z", "-mx0")
	listing, err := e.List(t.Context(), archive, "")
	if err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(archive)
	data[100] ^= 0xFF // flip a byte inside the stored data
	if err := os.WriteFile(archive, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Extract(t.Context(), archive, t.TempDir(), "", listing, nil); !errors.Is(err, application.ErrCorrupt) {
		t.Fatalf("corrupt archive err = %v", err)
	}
}

func TestVerifyCatchesMismatchedIndex(t *testing.T) {
	t.Parallel()
	e := require7zz(t)
	archive := makeArchive(t, "game.zip", map[string]string{"a.iso": "real data"}, "-tzip")
	listing, _ := e.List(t.Context(), archive, "")
	for i := range listing.Entries {
		listing.Entries[i].CRC = "DEADBEEF"
	}
	if _, err := e.Extract(t.Context(), archive, t.TempDir(), "", listing, nil); !errors.Is(err, application.ErrCorrupt) {
		t.Fatalf("CRC mismatch err = %v", err)
	}
}

func TestNotAnArchive(t *testing.T) {
	t.Parallel()
	e := require7zz(t)
	p := filepath.Join(t.TempDir(), "game.nsp")
	_ = os.WriteFile(p, []byte("PFS0 not an archive"), 0o600)
	if _, err := e.List(t.Context(), p, ""); !errors.Is(err, application.ErrCorrupt) {
		t.Fatalf("err = %v", err)
	}
}
