package main

import (
	"crypto/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

type partJSON struct {
	fullJob
	VolumeIndex *int    `json:"volumeIndex"`
	MergedInto  *string `json:"mergedInto"`
}

// splitArchive packs a ~2.5 MB file into 1 MB 7z volumes: set.7z.001..003.
func splitArchive(t *testing.T) map[string][]byte {
	t.Helper()
	src, out := t.TempDir(), t.TempDir()
	data := make([]byte, 2_500_000)
	_, _ = rand.Read(data) // incompressible, so the volumes keep their size
	_ = os.WriteFile(filepath.Join(src, "Shadow of the Colossus.iso"), data, 0o600)
	cmd := exec.CommandContext(t.Context(), "7zz", "a", "-bso0", "-bsp0", "-t7z", "-mx0", "-v1m",
		filepath.Join(out, "Shadow of the Colossus.7z"), ".")
	cmd.Dir = src
	if o, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("7zz a: %v %s", err, o)
	}
	parts := map[string][]byte{}
	entries, _ := os.ReadDir(out)
	for _, e := range entries {
		parts[e.Name()], _ = os.ReadFile(filepath.Join(out, e.Name()))
	}
	if len(parts) != 3 {
		t.Fatalf("expected 3 volumes, got %v", slices.Collect(func(yield func(string) bool) {
			for k := range parts {
				if !yield(k) {
					return
				}
			}
		}))
	}
	return parts
}

func TestVolumesArrivingOutOfOrderAreMerged(t *testing.T) {
	t.Parallel()
	require7zz(t)
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)
	parts := splitArchive(t)

	id3 := upload(t, srv, cookie, "Shadow of the Colossus.7z.003", parts["Shadow of the Colossus.7z.003"])
	id2 := upload(t, srv, cookie, "Shadow of the Colossus.7z.002", parts["Shadow of the Colossus.7z.002"])
	waitStatus(t, srv, cookie, id3, "waiting_parts")
	waitStatus(t, srv, cookie, id2, "waiting_parts")

	id1 := upload(t, srv, cookie, "Shadow of the Colossus.7z.001", parts["Shadow of the Colossus.7z.001"])
	first := waitStatus(t, srv, cookie, id1, "review", "failed")
	if first.Status != "review" {
		t.Fatalf("first volume = %+v (error %v)", first, first.Error)
	}
	for _, id := range []string{id2, id3} {
		p := decode[partJSON](t, do(t, http.MethodGet, srv.URL+"/api/jobs/"+id, "", cookie))
		if p.Status != "merged" || p.MergedInto == nil || *p.MergedInto != id1 || p.VolumeIndex == nil {
			t.Fatalf("part %s = %+v", id, p)
		}
	}

	got := items(t, srv, cookie, id1)
	if len(got) != 1 || got[0].Path != "Shadow of the Colossus.iso" {
		t.Fatalf("items = %+v", got)
	}
	if entries, _ := os.ReadDir(volumesDir(library)); len(entries) != 0 {
		t.Fatalf("volumes must be deleted after extraction, found %v", entries)
	}
}

func TestIncompleteSetWaitsAndPartsCanBeCancelled(t *testing.T) {
	t.Parallel()
	require7zz(t)
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)
	parts := splitArchive(t)

	id1 := upload(t, srv, cookie, "Shadow of the Colossus.7z.001", parts["Shadow of the Colossus.7z.001"])
	id2 := upload(t, srv, cookie, "Shadow of the Colossus.7z.002", parts["Shadow of the Colossus.7z.002"])
	waitStatus(t, srv, cookie, id1, "waiting_parts")
	waitStatus(t, srv, cookie, id2, "waiting_parts")

	// Volume 3 is missing: 7-Zip cannot open the set, so nothing is promoted.
	if j := decode[fullJob](t, do(t, http.MethodGet, srv.URL+"/api/jobs/"+id1, "", cookie)); j.Status != "waiting_parts" {
		t.Fatalf("first volume = %s, want waiting_parts", j.Status)
	}

	if res := do(t, http.MethodPost, srv.URL+"/api/jobs/"+id2+"/cancel", "", cookie); res.StatusCode != http.StatusOK {
		t.Fatalf("cancel part = %d", res.StatusCode)
	}
	remaining := 0
	_ = filepath.WalkDir(volumesDir(library), func(_ string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			remaining++
		}
		return nil
	})
	if remaining != 1 {
		t.Fatalf("files left in volumes = %d, want only volume 1", remaining)
	}
}

func TestLegacyRarVolumesFailClearly(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)
	cookie := login(t, srv)

	id := upload(t, srv, cookie, "game.r00", []byte("Rar!\x1A\x07\x00 old volume"))
	job := waitStatus(t, srv, cookie, id, "failed")
	if job.Error == nil || !strings.Contains(*job.Error, ".part1.rar") {
		t.Fatalf("error = %v", job.Error)
	}
}
