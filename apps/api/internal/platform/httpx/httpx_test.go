package httpx_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpx"
)

func TestWriteProblem(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	httpx.WriteProblem(rec, http.StatusTeapot, "Short", "Longer detail")

	if rec.Code != http.StatusTeapot {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("content-type = %q", ct)
	}
	var p httpx.Problem
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Status != http.StatusTeapot || p.Title != "Short" || p.Detail != "Longer detail" {
		t.Fatalf("problem = %+v", p)
	}
}

func TestWithClientIP(t *testing.T) {
	t.Parallel()

	var got string
	h := httpx.WithClientIP(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = httpx.ClientIP(r.Context())
	}))
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.8.20:51234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4") // must be ignored
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != "192.168.8.20" {
		t.Fatalf("ClientIP = %q, want 192.168.8.20", got)
	}
}

func TestRecoverReturnsProblem(t *testing.T) {
	t.Parallel()

	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := httpx.Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/x", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestSecurityHeadersNoStoreOnlyForAPI(t *testing.T) {
	t.Parallel()

	h := httpx.SecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	for path, wantCache := range map[string]string{"/api/consoles": "no-store", "/assets/app.js": ""} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if got := rec.Header().Get("Cache-Control"); got != wantCache {
			t.Errorf("%s Cache-Control = %q, want %q", path, got, wantCache)
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s missing nosniff", path)
		}
	}
}
