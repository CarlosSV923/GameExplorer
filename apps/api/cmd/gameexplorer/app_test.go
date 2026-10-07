package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	identityinfra "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/infrastructure"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

const testPassword = "correct horse battery staple"

func newTestServer(t *testing.T, mutate func(*config.Config)) *httptest.Server {
	t.Helper()
	cfg := config.Config{
		Port:          8080,
		LibraryPath:   t.TempDir(),
		DataPath:      t.TempDir(),
		Password:      testPassword,
		SessionSecret: strings.Repeat("s", config.MinSessionSecretBytes),
		SessionTTL:    config.Config{}.SessionTTL + 3600e9,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	a, err := newApp(t.Context(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("newApp: %v", err)
	}
	srv := httptest.NewServer(a.handler)
	t.Cleanup(func() {
		srv.Close()
		_ = a.Close()
	})
	return srv
}

func do(t *testing.T, method, url, body string, cookies ...*http.Cookie) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func decode[T any](t *testing.T, res *http.Response) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return v
}

func login(t *testing.T, srv *httptest.Server) *http.Cookie {
	t.Helper()
	res := do(t, http.MethodPost, srv.URL+"/api/auth/login", `{"password":"`+testPassword+`"}`)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("login status = %d", res.StatusCode)
	}
	for _, c := range res.Cookies() {
		if c.Name == "ge_session" {
			return c
		}
	}
	t.Fatal("no session cookie")
	return nil
}

func TestHealthIsPublicAndReportsChecks(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)

	res := do(t, http.MethodGet, srv.URL+"/api/health", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", res.StatusCode)
	}
	h := decode[struct {
		Status string `json:"status"`
		Checks []struct {
			Name string `json:"name"`
			Ok   bool   `json:"ok"`
		} `json:"checks"`
	}](t, res)
	if h.Status != "ok" || len(h.Checks) != 2 {
		t.Fatalf("health = %+v", h)
	}
}

func TestHealthDegradedWhenLibraryIsNotWritable(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, func(c *config.Config) {
		c.LibraryPath = filepath.Join(t.TempDir(), "does-not-exist")
	})

	h := decode[struct {
		Status string `json:"status"`
		Checks []struct {
			Name    string `json:"name"`
			Ok      bool   `json:"ok"`
			Message string `json:"message"`
		} `json:"checks"`
	}](t, do(t, http.MethodGet, srv.URL+"/api/health", ""))

	if h.Status != "degraded" || h.Checks[0].Ok || !strings.Contains(h.Checks[0].Message, "ACL") {
		t.Fatalf("health = %+v", h)
	}
}

func TestProtectedOperationsRequireSession(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)

	for _, path := range []string{"/api/consoles", "/api/auth/session"} {
		res := do(t, http.MethodGet, srv.URL+path, "")
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s status = %d, want 401", path, res.StatusCode)
		}
		if ct := res.Header.Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("%s content-type = %q", path, ct)
		}
	}

	forged := &http.Cookie{Name: "ge_session", Value: "Zm9v.YmFy"}
	if res := do(t, http.MethodGet, srv.URL+"/api/consoles", "", forged); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("forged cookie status = %d, want 401", res.StatusCode)
	}
}

func TestLoginFlow(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)

	if res := do(t, http.MethodPost, srv.URL+"/api/auth/login", `{"password":"wrong"}`); res.StatusCode != http.StatusUnauthorized {
		t.Fatalf("wrong password status = %d", res.StatusCode)
	}

	cookie := login(t, srv)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.MaxAge <= 0 {
		t.Errorf("cookie attributes = %+v", cookie)
	}

	consoles := decode[[]struct {
		Slug        string `json:"slug"`
		DisplayName string `json:"displayName"`
	}](t, do(t, http.MethodGet, srv.URL+"/api/consoles", "", cookie))
	if len(consoles) != 6 || consoles[0].Slug != "switch" || consoles[0].DisplayName != "Nintendo Switch" {
		t.Fatalf("consoles = %+v", consoles)
	}

	if res := do(t, http.MethodGet, srv.URL+"/api/auth/session", "", cookie); res.StatusCode != http.StatusOK {
		t.Fatalf("session status = %d", res.StatusCode)
	}

	res := do(t, http.MethodPost, srv.URL+"/api/auth/logout", "", cookie)
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("logout status = %d", res.StatusCode)
	}
	cleared := res.Cookies()
	if len(cleared) != 1 || cleared[0].MaxAge >= 0 || cleared[0].Value != "" {
		t.Fatalf("logout cookie = %+v", cleared)
	}
}

func TestLoginIsRateLimited(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)

	var last int
	for range loginBurst + 1 {
		last = do(t, http.MethodPost, srv.URL+"/api/auth/login", `{"password":"wrong"}`).StatusCode
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("attempt %d status = %d, want 429", loginBurst+1, last)
	}
}

func TestPasswordHashConfig(t *testing.T) {
	t.Parallel()
	phc, err := identityinfra.HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	srv := newTestServer(t, func(c *config.Config) { c.Password, c.PasswordHash = "", phc })
	login(t, srv)
}

func TestBadRequestsAndUnknownRoutes(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)

	if res := do(t, http.MethodPost, srv.URL+"/api/auth/login", `{not json`); res.StatusCode != http.StatusBadRequest {
		t.Errorf("malformed JSON status = %d, want 400", res.StatusCode)
	}
	res := do(t, http.MethodGet, srv.URL+"/api/does-not-exist", "")
	if res.StatusCode != http.StatusNotFound || res.Header.Get("Content-Type") != "application/problem+json" {
		t.Errorf("unknown API route = %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	res = do(t, http.MethodGet, srv.URL+"/consoles/ps2", "")
	if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Errorf("SPA route = %d %q", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if res.Header.Get("Content-Security-Policy") == "" {
		t.Error("missing CSP header")
	}
}

func TestHashPasswordCommand(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := hashPassword(strings.NewReader("s3cret\n"), &out); err != nil {
		t.Fatal(err)
	}
	v, err := identityinfra.NewArgon2idVerifier(strings.TrimSpace(out.String()))
	if err != nil || !v.Verify("s3cret") {
		t.Fatalf("hash output unusable: %q %v", out.String(), err)
	}
	if err := hashPassword(strings.NewReader(""), &out); err == nil {
		t.Fatal("empty password must fail")
	}
}
