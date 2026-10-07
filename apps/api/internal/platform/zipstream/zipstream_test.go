package zipstream_test

import (
	"archive/zip"
	"bytes"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/zipstream"
)

func text(name, content string, mod time.Time) zipstream.Entry {
	return zipstream.Entry{Name: name, Size: int64(len(content)), Modified: mod, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(content)), nil
	}}
}

// zeros is an entry of n zero bytes, cheap to produce at any size.
func zeros(name string, n int64) zipstream.Entry {
	return zipstream.Entry{Name: name, Size: n, Open: func() (io.ReadCloser, error) {
		return io.NopCloser(io.LimitReader(zeroReader{}, n)), nil
	}}
}

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

func TestSizeMatchesTheWriterAndTheArchiveIsValid(t *testing.T) {
	t.Parallel()
	mod := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	tests := map[string][]zipstream.Entry{
		"empty":           nil,
		"one file":        {text("Inside/Inside.nsp", "PFS0 inside", mod)},
		"unicode names":   {text("Pokémon Legends - Arceus/Pokémon™.nsp", "x", mod), text("ゲーム/disc.iso", "yy", mod)},
		"without mtime":   {text("a/b.bin", "abc", time.Time{})},
		"folder game":     {text("Demon's Souls/PS3_GAME/PARAM.SFO", "sfo", mod), text("Demon's Souls/PS3_GAME/USRDIR/EBOOT.BIN", "", mod)},
		"many small ones": manyEntries(300),
	}
	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := zipstream.Write(&buf, entries); err != nil {
				t.Fatal(err)
			}
			if got, want := zipstream.Size(entries), int64(buf.Len()); got != want {
				t.Fatalf("Size = %d, written = %d", got, want)
			}
			r, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
			if err != nil {
				t.Fatal(err)
			}
			if len(r.File) != len(entries) {
				t.Fatalf("archive has %d files, want %d", len(r.File), len(entries))
			}
			for i, f := range r.File {
				if f.Name != entries[i].Name || f.Method != zip.Store || int64(f.UncompressedSize64) != entries[i].Size {
					t.Fatalf("file %d = %s method %d size %d", i, f.Name, f.Method, f.UncompressedSize64)
				}
				rc, err := f.Open()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := io.Copy(io.Discard, rc); err != nil { // checks the CRC
					t.Fatalf("%s: %v", f.Name, err)
				}
				_ = rc.Close()
			}
		})
	}
}

func manyEntries(n int) []zipstream.Entry {
	out := make([]zipstream.Entry, n)
	for i := range out {
		out[i] = text("g/f"+strconv.Itoa(i), strconv.Itoa(i), time.Time{})
	}
	return out
}

// Files of 4 GB and more need zip64 records, and so do entries that start
// after the first 4 GB of the archive.
func TestSizeWithZip64(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("writes 4 GB through the zip writer")
	}
	mod := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	entries := []zipstream.Entry{
		zeros("Game/Game.xci", 1<<32+5),
		text("Game/Game [Update v1.0.1].nsp", "small after the 4 GB mark", mod),
	}
	exactly := []zipstream.Entry{zeros("Game/edge.iso", 1<<32-1)} // reaches 4 GiB - 1: zip64 extra, 32-bit descriptor
	var c counter
	if err := zipstream.Write(&c, entries); err != nil {
		t.Fatal(err)
	}
	if got := zipstream.Size(entries); got != c.n {
		t.Fatalf("Size = %d, written = %d", got, c.n)
	}
	c = counter{}
	if err := zipstream.Write(&c, exactly); err != nil {
		t.Fatal(err)
	}
	if got := zipstream.Size(exactly); got != c.n {
		t.Fatalf("edge Size = %d, written = %d", got, c.n)
	}
}

func TestManyEntriesUseZip64EndRecords(t *testing.T) {
	t.Parallel()
	entries := manyEntries(70000)
	var c counter
	if err := zipstream.Write(&c, entries); err != nil {
		t.Fatal(err)
	}
	if got := zipstream.Size(entries); got != c.n {
		t.Fatalf("Size = %d, written = %d", got, c.n)
	}
}

func TestEntriesThatChangedSizeFail(t *testing.T) {
	t.Parallel()
	e := text("a", "abc", time.Time{})
	e.Size = 10
	if err := zipstream.Write(io.Discard, []zipstream.Entry{e}); err == nil {
		t.Fatal("a short file must fail the archive")
	}
}
