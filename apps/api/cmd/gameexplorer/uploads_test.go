package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

type jobJSON struct {
	ID            string  `json:"id"`
	FileName      string  `json:"fileName"`
	Size          int64   `json:"size"`
	Received      int64   `json:"received"`
	Status        string  `json:"status"`
	OriginConsole *string `json:"originConsole"`
}

// tusCreate starts a tus upload and returns its URL.
func tusCreate(t *testing.T, srv *httptest.Server, cookie *http.Cookie, name, console string, size int) string {
	t.Helper()
	meta := "filename " + base64.StdEncoding.EncodeToString([]byte(name))
	if console != "" {
		meta += ",consoleSlug " + base64.StdEncoding.EncodeToString([]byte(console))
	}
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/uploads/", nil)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", strconv.Itoa(size))
	req.Header.Set("Upload-Metadata", meta)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("tus create status = %d", res.StatusCode)
	}
	return res.Header.Get("Location")
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

func waitForJob(t *testing.T, srv *httptest.Server, cookie *http.Cookie, id, status string) jobJSON {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		res := do(t, http.MethodGet, srv.URL+"/api/jobs/"+id, "", cookie)
		if res.StatusCode == http.StatusOK {
			j := decode[jobJSON](t, res)
			if j.Status == status {
				return j
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %s never reached %s", id, status)
	return jobJSON{}
}

func TestUploadFlowWithLiveEvents(t *testing.T) {
	t.Parallel()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)

	// Subscribe to the event stream first.
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/api/jobs/events", nil)
	req.AddCookie(cookie)
	stream, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if ct := stream.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("stream content-type = %q", ct)
	}
	events := make(chan jobJSON, 16)
	go func() {
		sc := bufio.NewScanner(stream.Body)
		for sc.Scan() {
			if data, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
				var j jobJSON
				if json.Unmarshal([]byte(data), &j) == nil {
					events <- j
				}
			}
		}
	}()

	const name = "INSIDE [0100D2D009028000][v0].nsp"
	url := tusCreate(t, srv, cookie, name, "switch", 10)
	id := url[strings.LastIndex(url, "/")+1:]
	tusPatch(t, url, cookie, 0, "0123")
	tusPatch(t, url, cookie, 4, "456789") // resumed in a second request

	job := waitForJob(t, srv, cookie, id, "uploaded")
	if job.FileName != name || job.Size != 10 || job.Received != 10 || job.OriginConsole == nil || *job.OriginConsole != "switch" {
		t.Fatalf("job = %+v", job)
	}
	data, err := os.ReadFile(filepath.Join(uploadsDir(library), id))
	if err != nil || string(data) != "0123456789" {
		t.Fatalf("stored bytes = %q, %v", data, err)
	}

	seen := map[string]bool{}
	timeout := time.After(5 * time.Second)
	for !seen["uploaded"] {
		select {
		case e := <-events:
			if e.ID == id {
				seen[e.Status] = true
			}
		case <-timeout:
			t.Fatalf("events seen = %v, want uploading and uploaded", seen)
		}
	}
	if !seen["uploading"] {
		t.Fatalf("events seen = %v, missing uploading", seen)
	}

	list := decode[[]jobJSON](t, do(t, http.MethodGet, srv.URL+"/api/jobs", "", cookie))
	if len(list) != 1 || list[0].ID != id {
		t.Fatalf("list = %+v", list)
	}
}

func TestCancelJobDeletesUpload(t *testing.T) {
	t.Parallel()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)

	url := tusCreate(t, srv, cookie, "game.rar", "", 4)
	id := url[strings.LastIndex(url, "/")+1:]
	waitForJob(t, srv, cookie, id, "uploading")

	res := do(t, http.MethodPost, srv.URL+"/api/jobs/"+id+"/cancel", "", cookie)
	if res.StatusCode != http.StatusOK || decode[jobJSON](t, res).Status != "cancelled" {
		t.Fatalf("cancel status = %d", res.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(uploadsDir(library), id)); !os.IsNotExist(err) {
		t.Fatalf("upload data still on disk: %v", err)
	}
	if res := do(t, http.MethodPost, srv.URL+"/api/jobs/"+id+"/cancel", "", cookie); res.StatusCode != http.StatusConflict {
		t.Fatalf("second cancel = %d, want 409", res.StatusCode)
	}
	if res := do(t, http.MethodGet, srv.URL+"/api/jobs/nope", "", cookie); res.StatusCode != http.StatusNotFound {
		t.Fatalf("missing job = %d, want 404", res.StatusCode)
	}
}

func TestUploadsRequireSession(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/uploads/", nil)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", "4")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("tus without session = %d, want 401", res.StatusCode)
	}
	if res := do(t, http.MethodGet, srv.URL+"/api/jobs/events", ""); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("events without session = %d, want 401", res.StatusCode)
	}
}

func TestUploadsDisabledWhenLibraryIsNotWritable(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "a-file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// A path below a regular file can never be created.
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = filepath.Join(blocker, "library") })
	cookie := login(t, srv)

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL+"/api/uploads/", nil)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.AddCookie(cookie)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("uploads status = %d, want 503", res.StatusCode)
	}
}
