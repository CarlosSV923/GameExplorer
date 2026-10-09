package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// splitArchive builds a 7z split in three volumes (game.7z.001…003).
func splitArchive(t *testing.T) map[string][]byte {
	t.Helper()
	dir := t.TempDir()
	game := filepath.Join(dir, "Game.iso")
	content := strings.Repeat("0123456789", 3000)
	writeFile(t, game, content)
	//nolint:gosec // fixed arguments in a test
	if out, err := exec.CommandContext(t.Context(), "7zz", "a", "-mx0", "-v10k", filepath.Join(dir, "game.7z"), game).CombinedOutput(); err != nil {
		t.Fatalf("7zz: %v %s", err, out)
	}
	parts := map[string][]byte{}
	for i := 1; ; i++ {
		name := "game.7z.00" + strconv.Itoa(i)
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			break
		}
		parts[name] = b
	}
	if len(parts) < 3 {
		t.Fatalf("parts = %d", len(parts))
	}
	return parts
}

func TestPartsOfOneArchiveAreJoined(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv, cookie, _ := libraryServer(t)
	parts := splitArchive(t)
	size := strconv.Itoa(len(parts))

	// The last part first: nothing happens until every part has arrived.
	ids := map[string]string{}
	names := []string{"game.7z.003", "game.7z.001", "game.7z.002"}
	for _, n := range names[:len(names)-1] {
		ids[n] = upload(t, srv, cookie, n, "psp", "Game", parts[n], "group", "g1", "groupSize", size)
		waitStatus(t, srv, cookie, ids[n], "waiting_parts")
	}
	last := names[len(names)-1]
	ids[last] = upload(t, srv, cookie, last, "psp", "Game", parts[last], "group", "g1", "groupSize", size)

	first := waitStatus(t, srv, cookie, ids["game.7z.001"], "confirm", "failed")
	if first.Status != "confirm" {
		t.Fatalf("first volume = %+v", first)
	}
	for _, n := range []string{"game.7z.002", "game.7z.003"} {
		if j := waitStatus(t, srv, cookie, ids[n], "merged"); j.MergedInto == nil || *j.MergedInto != first.ID {
			t.Fatalf("%s = %+v", n, j)
		}
	}
	if files := jobFiles(t, srv, cookie, first.ID); len(files) != 1 || files[0].Path != "Game.iso" || !files[0].Valid {
		t.Fatalf("files = %+v", files)
	}
}

func TestFilesThatAreNotPartsOfOneArchiveFail(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)

	a := upload(t, srv, cookie, "a.iso", "psp", "Game", []byte("a"), "group", "g2", "groupSize", "2")
	b := upload(t, srv, cookie, "b.iso", "psp", "Game", []byte("b"), "group", "g2", "groupSize", "2")
	for _, id := range []string{a, b} {
		if j := waitStatus(t, srv, cookie, id, "failed"); j.Error == nil || !strings.Contains(*j.Error, "partes") {
			t.Fatalf("job = %+v", j)
		}
	}
}

func TestLegacyRarVolumesFailClearly(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)
	id := upload(t, srv, cookie, "game.r00", "psp", "Game", []byte("rar"))
	if j := waitStatus(t, srv, cookie, id, "failed"); j.Error == nil || !strings.Contains(*j.Error, ".r00") {
		t.Fatalf("job = %+v", j)
	}
}
