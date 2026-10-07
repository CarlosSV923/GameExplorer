package web_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/web"
)

func get(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return rec
}

func TestSPAHandler(t *testing.T) {
	t.Parallel()

	h := web.NewHandler(fstest.MapFS{
		"index.html":       {Data: []byte("<html>app</html>")},
		"assets/app-1.js":  {Data: []byte("console.log(1)")},
		"favicon.svg":      {Data: []byte("<svg/>")},
		"assets/nested/xx": {Data: []byte("x")},
	})

	tests := []struct {
		path, wantBody, wantCache string
	}{
		{"/", "<html>app</html>", "no-cache"},
		{"/consoles/ps2", "<html>app</html>", "no-cache"}, // client-side route
		{"/assets/app-1.js", "console.log(1)", "public, max-age=31536000, immutable"},
		{"/favicon.svg", "<svg/>", ""},
		{"/assets/missing.js", "<html>app</html>", "no-cache"},
		{"/../../etc/passwd", "<html>app</html>", "no-cache"},
	}
	for _, tt := range tests {
		rec := get(t, h, tt.path)
		body, _ := io.ReadAll(rec.Body)
		if rec.Code != http.StatusOK || string(body) != tt.wantBody {
			t.Errorf("%s: %d %q, want 200 %q", tt.path, rec.Code, body, tt.wantBody)
		}
		if got := rec.Header().Get("Cache-Control"); got != tt.wantCache {
			t.Errorf("%s: Cache-Control %q, want %q", tt.path, got, tt.wantCache)
		}
	}
}

func TestSPAHandlerWithoutBuild(t *testing.T) {
	t.Parallel()

	rec := get(t, web.NewHandler(fstest.MapFS{".gitkeep": {}}), "/anything")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "task dev:web") {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestSPAHandlerRejectsWrites(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	web.NewHandler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST status = %d, want 405", rec.Code)
	}
}
