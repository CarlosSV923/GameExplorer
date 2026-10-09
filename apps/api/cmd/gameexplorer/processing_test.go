package main

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

func TestZipIsExtractedAndValidated(t *testing.T) {
	t.Parallel()
	require7zz(t)
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)

	archive := zipOf(t, map[string]string{
		"Limbo NSP/Limbo [0100A8E005E7C000].nsp": "base",
		"Limbo NSP/Limbo update.nsp":             "update",
		"Limbo NSP/leeme.txt":                    "hola",
		"__MACOSX/._Limbo.nsp":                   "junk",
	})
	id := upload(t, srv, cookie, "Limbo.zip", "switch", "Limbo", archive)
	job := waitStatus(t, srv, cookie, id, "confirm", "failed")
	if job.Status != "confirm" {
		t.Fatalf("job = %+v", job)
	}
	files := jobFiles(t, srv, cookie, id)
	valid := map[string]bool{}
	for _, f := range files {
		valid[f.Path] = f.Valid
	}
	if len(files) != 3 || !valid["Limbo NSP/Limbo [0100A8E005E7C000].nsp"] || !valid["Limbo NSP/Limbo update.nsp"] || valid["Limbo NSP/leeme.txt"] {
		t.Fatalf("files = %+v (macOS forks are skipped, the .txt is discarded)", files)
	}
	// The archive is deleted once extracted (RF-06).
	if entries, _ := os.ReadDir(uploadsDir(library)); len(entries) != 0 {
		t.Fatalf("uploads left = %v", entries)
	}
}

func TestWiiTakesOneFilePerUpload(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv := newTestServer(t, nil)
	cookie := login(t, srv)

	archive := zipOf(t, map[string]string{"a.iso": "a", "b.wbfs": "b"})
	id := upload(t, srv, cookie, "two.zip", "wii", "Two", archive)
	job := waitStatus(t, srv, cookie, id, "invalid")
	if job.InvalidReason == nil || *job.InvalidReason != "many" {
		t.Fatalf("job = %+v", job)
	}
	// Switch does not accept them either: still invalid, now with none.
	res := post(t, srv, cookie, "/api/jobs/"+id+"/console", `{"console":"switch"}`)
	wantStatus(t, res, http.StatusOK, "change console")
	if j := decode[jobJSON](t, res); j.Status != "invalid" || *j.InvalidReason != "none" || j.Console != "switch" {
		t.Fatalf("job = %+v", j)
	}
}

func TestChangeConsoleKeepsTheExtractedFiles(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv := newTestServer(t, nil)
	cookie := login(t, srv)

	id := upload(t, srv, cookie, "Ookami.zip", "switch", "Ōkami", zipOf(t, map[string]string{"Ookami (USA).nkit.iso": "wii"}))
	waitStatus(t, srv, cookie, id, "invalid")
	if res := post(t, srv, cookie, "/api/jobs/"+id+"/console", `{"console":"ps3"}`); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown console = %d, want 400", res.StatusCode)
	}
	// A .nkit.iso is not an .iso: PSP does not take it (RF-41).
	res := post(t, srv, cookie, "/api/jobs/"+id+"/console", `{"console":"psp"}`)
	if j := decode[jobJSON](t, res); j.Status != "invalid" {
		t.Fatalf("psp: job = %+v", j)
	}
	res = post(t, srv, cookie, "/api/jobs/"+id+"/console", `{"console":"wii"}`)
	if j := decode[jobJSON](t, res); j.Status != "confirm" || j.Console != "wii" || j.InvalidReason != nil {
		t.Fatalf("wii: job = %+v", j)
	}
}

func TestEncryptedArchiveWaitsForPassword(t *testing.T) {
	t.Parallel()
	require7zz(t)
	dir := t.TempDir()
	game := filepath.Join(dir, "Game.iso")
	writeFile(t, game, "disc image")
	archive := filepath.Join(dir, "secret.7z")
	//nolint:gosec // fixed arguments in a test
	if out, err := exec.CommandContext(t.Context(), "7zz", "a", "-pS3cr3t", "-mhe=on", archive, game).CombinedOutput(); err != nil {
		t.Fatalf("7zz: %v %s", err, out)
	}
	srv := newTestServer(t, nil)
	cookie := login(t, srv)

	id := upload(t, srv, cookie, "secret.7z", "psp", "Game", []byte(readFile(t, archive)))
	job := waitStatus(t, srv, cookie, id, "needs_password")
	if job.Error == nil || !strings.Contains(*job.Error, "contraseña") {
		t.Fatalf("job = %+v", job)
	}
	post(t, srv, cookie, "/api/jobs/"+id+"/password", `{"password":"wrong"}`)
	deadline := time.Now().Add(15 * time.Second)
	for {
		j := waitStatus(t, srv, cookie, id, "needs_password")
		if j.Error != nil && strings.Contains(*j.Error, "incorrecta") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("wrong password: job = %+v", j)
		}
		time.Sleep(50 * time.Millisecond)
	}
	post(t, srv, cookie, "/api/jobs/"+id+"/password", `{"password":"S3cr3t"}`)
	waitStatus(t, srv, cookie, id, "confirm")
}

func TestZipSlipIsRejected(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv := newTestServer(t, nil)
	cookie := login(t, srv)

	id := upload(t, srv, cookie, "evil.zip", "psp", "Evil", zipOf(t, map[string]string{"../../escape.iso": "x"}))
	job := waitStatus(t, srv, cookie, id, "failed")
	if job.Error == nil || !strings.Contains(*job.Error, "rutas peligrosas") {
		t.Fatalf("job = %+v", job)
	}
}

func TestInvalidUploadsAreSetAside(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv, cookie, library := libraryServer(t)
	archive := zipOf(t, map[string]string{"Disc/Game.iso": "iso", "Disc/readme.txt": "txt"})

	// To the unassigned section, under the game's name.
	toSection := upload(t, srv, cookie, "game.zip", "switch", "Game", archive)
	waitStatus(t, srv, cookie, toSection, "invalid")
	res := post(t, srv, cookie, "/api/jobs/"+toSection+"/resolve", `{"action":"unassigned"}`)
	wantStatus(t, res, http.StatusOK, "resolve unassigned")
	if j := decode[jobJSON](t, res); j.Status != "unassigned" {
		t.Fatalf("job = %+v", j)
	}
	if got := readFile(t, filepath.Join(library, "_unassigned", "Game", "Disc", "Game.iso")); got != "iso" {
		t.Fatalf("set aside = %q", got)
	}
	files := unassignedList(t, srv, cookie)
	if len(files) != 2 || files[0].Reason != "upload" || files[0].Origin != "game.zip" {
		t.Fatalf("unassigned = %+v", files)
	}

	// To the trash: restorable into the section.
	toTrash := upload(t, srv, cookie, "game.zip", "switch", "Game", archive)
	waitStatus(t, srv, cookie, toTrash, "invalid")
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+toTrash+"/resolve", `{"action":"trash"}`), http.StatusOK, "resolve trash")
	trash := trashList(t, srv, cookie)
	if len(trash) != 1 || trash[0].Kind != "unassigned" || len(trash[0].Files) != 2 || trash[0].GameID != nil {
		t.Fatalf("trash = %+v", trash)
	}
	wantStatus(t, post(t, srv, cookie, "/api/trash/"+id(trash[0].ID)+"/restore", ""), http.StatusOK, "restore")
	if got := unassignedList(t, srv, cookie); len(got) != 4 {
		t.Fatalf("unassigned after restore = %+v (names taken get a suffix)", got)
	}
	if !exists(filepath.Join(library, "_unassigned", "Game", "Disc", "Game (2).iso")) {
		t.Fatal("restored file should take a free name")
	}

	// Deleted for good.
	gone := upload(t, srv, cookie, "game.zip", "switch", "Gone", archive)
	waitStatus(t, srv, cookie, gone, "invalid")
	res = post(t, srv, cookie, "/api/jobs/"+gone+"/resolve", `{"action":"delete"}`)
	if j := decode[jobJSON](t, res); j.Status != "cancelled" {
		t.Fatalf("job = %+v", j)
	}
	if exists(filepath.Join(stagingDir(library), gone)) {
		t.Fatal("staging left behind")
	}

	// Only invalid uploads can be set aside.
	fits := upload(t, srv, cookie, "fits.iso", "psp", "Fits", []byte("iso"))
	waitStatus(t, srv, cookie, fits, "confirm")
	if res := post(t, srv, cookie, "/api/jobs/"+fits+"/resolve", `{"action":"trash"}`); res.StatusCode != http.StatusConflict {
		t.Fatalf("resolve a fitting upload = %d, want 409", res.StatusCode)
	}
}

func TestCancelRemovesStaging(t *testing.T) {
	t.Parallel()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)

	id := upload(t, srv, cookie, "Game.cso", "psp", "Game", []byte("cso"))
	waitStatus(t, srv, cookie, id, "confirm")
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+id+"/cancel", ""), http.StatusOK, "cancel")
	if exists(filepath.Join(stagingDir(library), id)) {
		t.Fatal("staging left behind")
	}
}

func TestHealthReportsExtractor(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv := newTestServer(t, nil)
	h := decode[struct {
		Checks []struct {
			Name string `json:"name"`
			Ok   bool   `json:"ok"`
		} `json:"checks"`
	}](t, do(t, http.MethodGet, srv.URL+"/api/health", ""))
	for _, c := range h.Checks {
		if c.Name == "extractor" && c.Ok {
			return
		}
	}
	t.Fatalf("health = %+v", h)
}
