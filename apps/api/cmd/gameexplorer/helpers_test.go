package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

type jobJSON struct {
	ID             string  `json:"id"`
	FileName       string  `json:"fileName"`
	Size           int64   `json:"size"`
	Received       int64   `json:"received"`
	Status         string  `json:"status"`
	Console        string  `json:"console"`
	Title          string  `json:"title"`
	IgdbID         *int64  `json:"igdbId"`
	InvalidReason  *string `json:"invalidReason"`
	FromUnassigned bool    `json:"fromUnassigned"`
	Error          *string `json:"error"`
	Warning        *string `json:"warning"`
	MergedInto     *string `json:"mergedInto"`
}

type fileJSON struct {
	Path     string   `json:"path"`
	Size     int64    `json:"size"`
	Valid    bool     `json:"valid"`
	Consoles []string `json:"consoles"`
	InPlace  bool     `json:"inPlace"`
}

type itemJSON struct {
	ID    int64   `json:"id"`
	Kind  string  `json:"kind"`
	Label *string `json:"label"`
	File  string  `json:"file"`
	Size  int64   `json:"size"`
}

type gameJSON struct {
	ID           int64      `json:"id"`
	IgdbID       *int64     `json:"igdbId"`
	Console      string     `json:"console"`
	Title        string     `json:"title"`
	Folder       string     `json:"folder"`
	Path         string     `json:"path"`
	ItemCount    int        `json:"itemCount"`
	Size         int64      `json:"size"`
	CoverImageID *string    `json:"coverImageId"`
	Genres       []string   `json:"genres"`
	Items        []itemJSON `json:"items"`
}

type planJSON struct {
	Console   string     `json:"console"`
	Title     string     `json:"title"`
	Folder    string     `json:"folder"`
	GameID    *int64     `json:"gameId"`
	Existing  []itemJSON `json:"existing"`
	Discarded []string   `json:"discarded"`
	Files     []struct {
		Path      string    `json:"path"`
		File      string    `json:"file"`
		Action    string    `json:"action"`
		Duplicate *itemJSON `json:"duplicate"`
	} `json:"files"`
}

type commitJSON struct {
	Job      jobJSON `json:"job"`
	GameID   int64   `json:"gameId"`
	Path     string  `json:"path"`
	Stored   int     `json:"stored"`
	Replaced int     `json:"replaced"`
	Skipped  int     `json:"skipped"`
}

type unassignedJSON struct {
	ID       int64    `json:"id"`
	Path     string   `json:"path"`
	Name     string   `json:"name"`
	Origin   string   `json:"origin"`
	Reason   string   `json:"reason"`
	Size     int64    `json:"size"`
	Consoles []string `json:"consoles"`
	Archive  bool     `json:"archive"`
	Copying  bool     `json:"copying"`
	Console  *string  `json:"console"`
	IgdbID   *int64   `json:"igdbId"`
}

type entryJSON struct {
	ID      int64            `json:"id"`
	Name    string           `json:"name"`
	Folder  bool             `json:"folder"`
	Size    int64            `json:"size"`
	Console *string          `json:"console"`
	IgdbID  *int64           `json:"igdbId"`
	Copying bool             `json:"copying"`
	Busy    bool             `json:"busy"`
	Files   []unassignedJSON `json:"files"`
}

type trashJSON struct {
	ID        int64            `json:"id"`
	Kind      string           `json:"kind"`
	GameID    *int64           `json:"gameId"`
	Title     string           `json:"title"`
	WholeGame bool             `json:"wholeGame"`
	Reason    string           `json:"reason"`
	TrashedAt time.Time        `json:"trashedAt"`
	ExpiresAt time.Time        `json:"expiresAt"`
	Size      int64            `json:"size"`
	Items     []itemJSON       `json:"items"`
	Files     []unassignedJSON `json:"files"`
}

func require7zz(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("7zz"); err != nil {
		if os.Getenv("REQUIRE_7ZZ") != "" {
			t.Fatal("7zz not found but REQUIRE_7ZZ is set")
		}
		t.Skip("7zz not installed")
	}
}

// libraryServer is a test server with the fake IGDB; it returns the
// library path too.
func libraryServer(t *testing.T) (*httptest.Server, *http.Cookie, string) {
	t.Helper()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) {
		c.LibraryPath = library
		withFakeIGDB(fakeIGDBServer(t))(c)
	})
	return srv, login(t, srv), library
}

// form is an upload's metadata (RF-03).
type form map[string]string

func encodeMeta(meta form) string {
	var parts []string
	for k, v := range meta {
		parts = append(parts, k+" "+base64.StdEncoding.EncodeToString([]byte(v)))
	}
	slices.Sort(parts)
	return strings.Join(parts, ",")
}

// tusCreate starts a tus upload and returns the response.
func tusCreate(t *testing.T, srv *httptest.Server, cookie *http.Cookie, meta form, size int) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/uploads/", nil)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", strconv.Itoa(size))
	req.Header.Set("Upload-Metadata", encodeMeta(meta))
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func tusPatch(t *testing.T, url string, cookie *http.Cookie, offset int, data string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPatch, url, strings.NewReader(data))
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Offset", strconv.Itoa(offset))
	req.Header.Set("Content-Type", "application/offset+octet-stream")
	req.AddCookie(cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("tus patch status = %d", res.StatusCode)
	}
}

// upload sends a file through tus with its form and returns the job id.
func upload(t *testing.T, srv *httptest.Server, cookie *http.Cookie, name, console, title string, data []byte, extra ...string) string {
	t.Helper()
	meta := form{"filename": name, "consoleSlug": console, "title": title}
	for i := 0; i+1 < len(extra); i += 2 {
		meta[extra[i]] = extra[i+1]
	}
	res := tusCreate(t, srv, cookie, meta, len(data))
	if res.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("tus create %s = %d %s", name, res.StatusCode, body)
	}
	url := res.Header.Get("Location")
	if len(data) > 0 {
		tusPatch(t, url, cookie, 0, string(data))
	}
	return url[strings.LastIndex(url, "/")+1:]
}

func waitStatus(t *testing.T, srv *httptest.Server, cookie *http.Cookie, id string, statuses ...string) jobJSON {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var last jobJSON
	for time.Now().Before(deadline) {
		last = decode[jobJSON](t, do(t, http.MethodGet, srv.URL+"/api/jobs/"+id, "", cookie))
		if slices.Contains(statuses, last.Status) {
			return last
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %s stuck in %s (want %v)", id, last.Status, statuses)
	return last
}

func jobFiles(t *testing.T, srv *httptest.Server, cookie *http.Cookie, id string) []fileJSON {
	t.Helper()
	return decode[[]fileJSON](t, do(t, http.MethodGet, srv.URL+"/api/jobs/"+id+"/files", "", cookie))
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(files[name]))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jsonBody(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// commitBody is a commit request: one entry per file (path → fields).
func commitBody(t *testing.T, files ...map[string]any) string {
	t.Helper()
	return jsonBody(t, map[string]any{"files": files})
}

func call(t *testing.T, srv *httptest.Server, cookie *http.Cookie, method, path, body string) *http.Response {
	t.Helper()
	return do(t, method, srv.URL+path, body, cookie)
}

func post(t *testing.T, srv *httptest.Server, cookie *http.Cookie, path, body string) *http.Response {
	t.Helper()
	return call(t, srv, cookie, http.MethodPost, path, body)
}

func get(t *testing.T, srv *httptest.Server, cookie *http.Cookie, path string, headers ...string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(cookie)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func body(t *testing.T, res *http.Response) string {
	t.Helper()
	b, _ := io.ReadAll(res.Body)
	return string(b)
}

func wantStatus(t *testing.T, res *http.Response, status int, what string) {
	t.Helper()
	if res.StatusCode != status {
		t.Fatalf("%s = %d, want %d: %s", what, res.StatusCode, status, body(t, res))
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// writeSettled writes a file that looks copied a while ago: the unassigned
// section takes files modified in the last minute as still being copied.
func writeSettled(t *testing.T, path, content string) {
	t.Helper()
	writeFile(t, path, content)
	old := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func id(n int64) string { return strconv.FormatInt(n, 10) }

func gameOf(t *testing.T, srv *httptest.Server, cookie *http.Cookie, game int64) (gameJSON, int) {
	t.Helper()
	res := get(t, srv, cookie, "/api/games/"+id(game))
	if res.StatusCode != http.StatusOK {
		return gameJSON{}, res.StatusCode
	}
	return decode[gameJSON](t, res), res.StatusCode
}

// store uploads a raw file for a console and commits it; it returns the game id.
func store(t *testing.T, srv *httptest.Server, cookie *http.Cookie, console, title, name, content string, file map[string]any, extra ...string) int64 {
	t.Helper()
	job := upload(t, srv, cookie, name, console, title, []byte(content), extra...)
	waitStatus(t, srv, cookie, job, "confirm")
	file["path"] = name
	res := post(t, srv, cookie, "/api/jobs/"+job+"/commit", commitBody(t, file))
	wantStatus(t, res, http.StatusOK, "commit "+name)
	return decode[commitJSON](t, res).GameID
}

// unassignedList is every file of the unassigned section's entries.
func unassignedList(t *testing.T, srv *httptest.Server, cookie *http.Cookie) []unassignedJSON {
	t.Helper()
	var out []unassignedJSON
	for _, e := range unassignedEntries(t, srv, cookie) {
		out = append(out, e.Files...)
	}
	return out
}

func unassignedEntries(t *testing.T, srv *httptest.Server, cookie *http.Cookie) []entryJSON {
	t.Helper()
	return decode[[]entryJSON](t, get(t, srv, cookie, "/api/unassigned"))
}

// entryNamed finds an entry of the unassigned section by name.
func entryNamed(t *testing.T, srv *httptest.Server, cookie *http.Cookie, name string) entryJSON {
	t.Helper()
	entries := unassignedEntries(t, srv, cookie)
	for _, e := range entries {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("no entry %q in %+v", name, entries)
	return entryJSON{}
}

func trashList(t *testing.T, srv *httptest.Server, cookie *http.Cookie) []trashJSON {
	t.Helper()
	return decode[[]trashJSON](t, get(t, srv, cookie, "/api/trash"))
}

func readZip(t *testing.T, res *http.Response) map[string]string {
	t.Helper()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("zip download = %d %s", res.StatusCode, raw)
	}
	if res.ContentLength != int64(len(raw)) {
		t.Fatalf("Content-Length %d, body %d", res.ContentLength, len(raw))
	}
	r, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		out[f.Name] = string(b)
	}
	return out
}
