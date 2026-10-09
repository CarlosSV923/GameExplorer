package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

func TestUploadNeedsItsForm(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)
	cookie := login(t, srv)

	cases := map[string]form{
		"no title":           {"filename": "a.nsp", "consoleSlug": "switch"},
		"unknown console":    {"filename": "a.nsp", "consoleSlug": "ps3", "title": "A"},
		"bad igdb id":        {"filename": "a.nsp", "consoleSlug": "switch", "title": "A", "igdbId": "x"},
		"group without size": {"filename": "a.part1.rar", "consoleSlug": "switch", "title": "A", "group": "g1"},
	}
	for name, meta := range cases {
		if res := tusCreate(t, srv, cookie, meta, 10); res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, res.StatusCode)
		}
	}
	if res := tusCreate(t, srv, nil, form{"filename": "a.nsp", "consoleSlug": "switch", "title": "A"}, 10); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("without session: status = %d, want 401", res.StatusCode)
	}
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

	const name = "Limbo [0100A8E005E7C000].nsp"
	res := tusCreate(t, srv, cookie, form{"filename": name, "consoleSlug": "switch", "title": "Limbo"}, 10)
	wantStatus(t, res, http.StatusCreated, "tus create")
	url := res.Header.Get("Location")
	id := url[strings.LastIndex(url, "/")+1:]
	tusPatch(t, url, cookie, 0, "0123")
	tusPatch(t, url, cookie, 4, "456789") // resumed in a second request

	// A raw (non-archive) file is moved into staging and validated.
	job := waitStatus(t, srv, cookie, id, "confirm")
	if job.FileName != name || job.Size != 10 || job.Received != 10 || job.Console != "switch" || job.Title != "Limbo" {
		t.Fatalf("job = %+v", job)
	}
	data, err := os.ReadFile(filepath.Join(stagingDir(library), id, name))
	if err != nil || string(data) != "0123456789" {
		t.Fatalf("stored bytes = %q, %v", data, err)
	}
	files := jobFiles(t, srv, cookie, id)
	if len(files) != 1 || files[0].Path != name || !files[0].Valid || len(files[0].Consoles) != 1 || files[0].Consoles[0] != "switch" {
		t.Fatalf("files = %+v", files)
	}

	seen := map[string]bool{}
	timeout := time.After(5 * time.Second)
	for !seen["confirm"] {
		select {
		case e := <-events:
			if e.ID == id {
				seen[e.Status] = true
			}
		case <-timeout:
			t.Fatalf("events seen = %v", seen)
		}
	}
	if !seen["uploading"] {
		t.Errorf("events seen = %v, want uploading too", seen)
	}
}

func TestRawFileOfAnotherConsoleIsInvalid(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)
	cookie := login(t, srv)

	id := upload(t, srv, cookie, "Okami.iso", "switch", "Ōkami", []byte("not a switch file"))
	job := waitStatus(t, srv, cookie, id, "invalid")
	if job.InvalidReason == nil || *job.InvalidReason != "none" {
		t.Fatalf("job = %+v", job)
	}
	files := jobFiles(t, srv, cookie, id)
	if len(files) != 1 || files[0].Valid || strings.Join(files[0].Consoles, ",") != "gc,ps2,psp,wii" {
		t.Fatalf("files = %+v (an .iso fits GameCube, PS2, PSP and Wii)", files)
	}
}
