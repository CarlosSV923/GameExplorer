package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

type planJSON struct {
	Console  string `json:"console"`
	Title    string `json:"title"`
	Folder   string `json:"folder"`
	GameID   *int64 `json:"gameId"`
	Existing []struct {
		ID    int64    `json:"id"`
		Kind  string   `json:"kind"`
		Files []string `json:"files"`
	} `json:"existing"`
	Items []struct {
		Path      string   `json:"path"`
		Files     []string `json:"files"`
		Action    string   `json:"action"`
		Duplicate *struct {
			ID    int64    `json:"id"`
			Files []string `json:"files"`
		} `json:"duplicate"`
	} `json:"items"`
}

type commitJSON struct {
	Job      fullJob `json:"job"`
	GameID   int64   `json:"gameId"`
	Path     string  `json:"path"`
	Stored   int     `json:"stored"`
	Replaced int     `json:"replaced"`
	Skipped  int     `json:"skipped"`
}

func commitBody(t *testing.T, console string, game int64, items ...map[string]any) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"console": console, "igdbGameId": game, "items": items})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func post(t *testing.T, srv string, cookie *http.Cookie, id, op, body string) *http.Response {
	t.Helper()
	return do(t, http.MethodPost, srv+"/api/jobs/"+id+"/"+op, body, cookie)
}

func problemDetail(t *testing.T, res *http.Response) string {
	t.Helper()
	b, _ := io.ReadAll(res.Body)
	return string(b)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCommitStoresARawFile(t *testing.T) {
	t.Parallel()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) {
		c.LibraryPath = library
		withFakeIGDB(fakeIGDBServer(t))(c)
	})
	cookie := login(t, srv)

	const name = "Mario Kart 8 Deluxe [0100152000022000][v0].nsp"
	id := upload(t, srv, cookie, name, []byte("PFS0 mario kart"))
	waitStatus(t, srv, cookie, id, "review")
	body := commitBody(t, "switch", 26764, map[string]any{"path": name, "kind": "base"})

	plan := decode[planJSON](t, post(t, srv.URL, cookie, id, "plan", body))
	if plan.Folder != "Mario Kart 8 Deluxe" || plan.GameID != nil || len(plan.Items) != 1 ||
		plan.Items[0].Action != "store" || !slices.Equal(plan.Items[0].Files, []string{"Mario Kart 8 Deluxe.nsp"}) {
		t.Fatalf("plan = %+v", plan)
	}

	res := decode[commitJSON](t, post(t, srv.URL, cookie, id, "commit", body))
	if res.Job.Status != "done" || res.Path != "switch/Mario Kart 8 Deluxe" || res.Stored != 1 || res.GameID == 0 {
		t.Fatalf("commit = %+v", res)
	}
	if got := readFile(t, filepath.Join(library, "switch", "Mario Kart 8 Deluxe", "Mario Kart 8 Deluxe.nsp")); got != "PFS0 mario kart" {
		t.Fatalf("stored content = %q", got)
	}
	if _, err := os.Stat(filepath.Join(stagingDir(library), id)); !os.IsNotExist(err) {
		t.Fatalf("staging must be removed after commit: %v", err)
	}
	if r := post(t, srv.URL, cookie, id, "commit", body); r.StatusCode != http.StatusConflict {
		t.Fatalf("second commit status = %d, want 409", r.StatusCode)
	}
}

func TestCommitDuplicatesNeedADecision(t *testing.T) {
	t.Parallel()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) {
		c.LibraryPath = library
		withFakeIGDB(fakeIGDBServer(t))(c)
	})
	cookie := login(t, srv)
	gameDir := filepath.Join(library, "switch", "Mario Kart 8 Deluxe")

	store := func(name, content string, extra map[string]any) (string, *http.Response) {
		t.Helper()
		id := upload(t, srv, cookie, name, []byte(content))
		waitStatus(t, srv, cookie, id, "review")
		item := map[string]any{"path": name, "kind": "base"}
		for k, v := range extra {
			item[k] = v
		}
		return id, post(t, srv.URL, cookie, id, "commit", commitBody(t, "switch", 26764, item))
	}

	if _, res := store("mk8.nsp", "PFS0 first", nil); res.StatusCode != http.StatusOK {
		t.Fatalf("first commit = %d %s", res.StatusCode, problemDetail(t, res))
	}

	// Another base game in another format is a duplicate.
	id, res := store("mk8.xci", "HEAD second", nil)
	if res.StatusCode != http.StatusConflict {
		t.Fatalf("undecided duplicate = %d, want 409", res.StatusCode)
	}
	if j := waitStatus(t, srv, cookie, id, "review"); j.Error != nil {
		t.Fatalf("a refused commit must not leave an error: %v", *j.Error)
	}
	plan := decode[planJSON](t, post(t, srv.URL, cookie, id, "plan",
		commitBody(t, "switch", 26764, map[string]any{"path": "mk8.xci", "kind": "base"})))
	if plan.GameID == nil || len(plan.Existing) != 1 || plan.Items[0].Action != "undecided" ||
		plan.Items[0].Duplicate == nil || plan.Items[0].Duplicate.Files[0] != "Mario Kart 8 Deluxe.nsp" {
		t.Fatalf("plan = %+v", plan)
	}

	res = post(t, srv.URL, cookie, id, "commit",
		commitBody(t, "switch", 26764, map[string]any{"path": "mk8.xci", "kind": "base", "onDuplicate": "replace"}))
	replaced := decode[commitJSON](t, res)
	if replaced.Job.Status != "done" || replaced.Replaced != 1 {
		t.Fatalf("replace = %+v", replaced)
	}
	if _, err := os.Stat(filepath.Join(gameDir, "Mario Kart 8 Deluxe.nsp")); !os.IsNotExist(err) {
		t.Fatalf("replaced file still in the library: %v", err)
	}
	if got := readFile(t, filepath.Join(gameDir, "Mario Kart 8 Deluxe.xci")); got != "HEAD second" {
		t.Fatalf("new file = %q", got)
	}
	trashed, _ := filepath.Glob(filepath.Join(trashDir(library), "*", "Mario Kart 8 Deluxe.nsp"))
	if len(trashed) != 1 || readFile(t, trashed[0]) != "PFS0 first" {
		t.Fatalf("old file not in the trash: %v", trashed)
	}

	// Skip keeps the library as it is and finishes the upload.
	id, res = store("mk8-again.xci", "HEAD third", map[string]any{"onDuplicate": "skip"})
	skipped := decode[commitJSON](t, res)
	if skipped.Job.Status != "done" || skipped.Skipped != 1 || skipped.Stored != 0 {
		t.Fatalf("skip = %+v", skipped)
	}
	if got := readFile(t, filepath.Join(gameDir, "Mario Kart 8 Deluxe.xci")); got != "HEAD second" {
		t.Fatalf("skip changed the library: %q", got)
	}
	if _, err := os.Stat(filepath.Join(stagingDir(library), id)); !os.IsNotExist(err) {
		t.Fatalf("staging of a skipped upload must be removed: %v", err)
	}

	// An update is a new item next to the base game.
	_, res = store("mk8 update.nsp", "PFS0 update", map[string]any{"kind": "update", "label": "v3.0.1"})
	if res.StatusCode != http.StatusOK {
		t.Fatalf("update commit = %d %s", res.StatusCode, problemDetail(t, res))
	}
	if _, err := os.Stat(filepath.Join(gameDir, "Mario Kart 8 Deluxe [Update v3.0.1].nsp")); err != nil {
		t.Fatal(err)
	}
}

func TestCommitFolderTakenOverSMBGetsTheYear(t *testing.T) {
	t.Parallel()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) {
		c.LibraryPath = library
		withFakeIGDB(fakeIGDBServer(t))(c)
	})
	cookie := login(t, srv)
	foreign := filepath.Join(library, "switch", "The Legend of Zelda - Tears of the Kingdom")
	if err := os.MkdirAll(foreign, 0o750); err != nil {
		t.Fatal(err)
	}

	id := upload(t, srv, cookie, "totk.nsp", []byte("PFS0 zelda"))
	waitStatus(t, srv, cookie, id, "review")
	res := decode[commitJSON](t, post(t, srv.URL, cookie, id, "commit",
		commitBody(t, "switch", 119388, map[string]any{"path": "totk.nsp", "kind": "base"})))
	if res.Path != "switch/The Legend of Zelda - Tears of the Kingdom (2023)" {
		t.Fatalf("path = %q", res.Path)
	}
	if entries, _ := os.ReadDir(foreign); len(entries) != 0 {
		t.Fatalf("the folder created over SMB was touched: %v", entries)
	}
}

func TestCommitDiscRenamesTracksAndRewritesTheCue(t *testing.T) {
	t.Parallel()
	require7zz(t)
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) {
		c.LibraryPath = library
		withFakeIGDB(fakeIGDBServer(t))(c)
	})
	cookie := login(t, srv)

	cue := "FILE \"ff7 (Track 1).bin\" BINARY\r\n  TRACK 01 MODE2/2352\r\nFILE \"ff7 (Track 2).bin\" BINARY\r\n  TRACK 02 AUDIO\r\n"
	id := upload(t, srv, cookie, "ff7.zip", zipOf(t, map[string]string{
		"FF7/ff7.cue":           cue,
		"FF7/ff7 (Track 1).bin": "data track",
		"FF7/ff7 (Track 2).bin": "audio track",
		"FF7/readme.txt":        "junk",
	}))
	waitStatus(t, srv, cookie, id, "review")

	res := post(t, srv.URL, cookie, id, "commit",
		commitBody(t, "psx", 427, map[string]any{"path": "FF7/ff7.cue", "kind": "disc", "discNumber": 1}))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("commit = %d %s", res.StatusCode, problemDetail(t, res))
	}
	dir := filepath.Join(library, "psx", "Final Fantasy VII")
	entries, _ := os.ReadDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{"Final Fantasy VII (Disc 1) (Track 1).bin", "Final Fantasy VII (Disc 1) (Track 2).bin", "Final Fantasy VII (Disc 1).cue"}
	if !slices.Equal(names, want) {
		t.Fatalf("files = %q, want %q", names, want)
	}
	wantCue := "FILE \"Final Fantasy VII (Disc 1) (Track 1).bin\" BINARY\r\n  TRACK 01 MODE2/2352\r\n" +
		"FILE \"Final Fantasy VII (Disc 1) (Track 2).bin\" BINARY\r\n  TRACK 02 AUDIO\r\n"
	if got := readFile(t, filepath.Join(dir, "Final Fantasy VII (Disc 1).cue")); got != wantCue {
		t.Fatalf("cue =\n%s", got)
	}
	if got := readFile(t, filepath.Join(dir, "Final Fantasy VII (Disc 1) (Track 2).bin")); got != "audio track" {
		t.Fatalf("track 2 = %q", got)
	}
}

func TestCommitValidation(t *testing.T) {
	t.Parallel()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) {
		c.LibraryPath = library
		withFakeIGDB(fakeIGDBServer(t))(c)
	})
	cookie := login(t, srv)

	id := upload(t, srv, cookie, "a.nsp", []byte("PFS0 a"))
	waitStatus(t, srv, cookie, id, "review")
	base := map[string]any{"path": "a.nsp", "kind": "base"}

	tests := []struct {
		name   string
		body   string
		status int
		detail string
	}{
		{"missing item decision", commitBody(t, "switch", 26764), http.StatusBadRequest, "a.nsp"},
		{"unknown path", commitBody(t, "switch", 26764, base, map[string]any{"path": "../etc/passwd", "kind": "base"}), http.StatusBadRequest, "no es un elemento"},
		{"every item skipped", commitBody(t, "switch", 26764, map[string]any{"path": "a.nsp", "skip": true}), http.StatusBadRequest, "al menos un elemento"},
		{"unknown console", commitBody(t, "n64", 26764, base), http.StatusBadRequest, "n64"},
		{"unknown game", commitBody(t, "switch", 999, base), http.StatusBadRequest, "IGDB"},
		{"update without version", commitBody(t, "switch", 26764, map[string]any{"path": "a.nsp", "kind": "update"}), http.StatusBadRequest, "versión"},
		{"disc without number", commitBody(t, "switch", 26764, map[string]any{"path": "a.nsp", "kind": "disc"}), http.StatusBadRequest, "disco"},
		{"unknown kind", commitBody(t, "switch", 26764, map[string]any{"path": "a.nsp", "kind": "demo"}), http.StatusBadRequest, ""},
	}
	for _, tt := range tests {
		res := post(t, srv.URL, cookie, id, "commit", tt.body)
		detail := problemDetail(t, res)
		if res.StatusCode != tt.status || !strings.Contains(detail, tt.detail) {
			t.Errorf("%s: %d %s", tt.name, res.StatusCode, detail)
		}
	}
	if j := waitStatus(t, srv, cookie, id, "review"); j.Status != "review" {
		t.Fatalf("job = %s after refused commits", j.Status)
	}
	if entries, _ := os.ReadDir(library); len(entries) != 1 { // only .gameexplorer
		t.Fatalf("refused commits created files: %v", entries)
	}

	noIGDB := newTestServer(t, nil)
	c2 := login(t, noIGDB)
	id2 := upload(t, noIGDB, c2, "a.nsp", []byte("PFS0 a"))
	waitStatus(t, noIGDB, c2, id2, "review")
	if res := post(t, noIGDB.URL, c2, id2, "plan", commitBody(t, "switch", 26764, base)); res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("without IGDB = %d, want 503", res.StatusCode)
	}
}
