package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/detection"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

func require7zz(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("7zz"); err != nil {
		if os.Getenv("REQUIRE_7ZZ") != "" {
			t.Fatal("7zz not found but REQUIRE_7ZZ is set")
		}
		t.Skip("7zz not installed")
	}
}

type itemJSON struct {
	Path          string   `json:"path"`
	Shape         string   `json:"shape"`
	Ignored       bool     `json:"ignored"`
	Consoles      []string `json:"consoles"`
	Confidence    string   `json:"confidence"`
	SuggestedKind *string  `json:"suggestedKind"`
}

type fullJob struct {
	jobJSON
	Error   *string `json:"error"`
	Warning *string `json:"warning"`
}

// upload sends data through tus and returns the job id.
func upload(t *testing.T, srv *httptest.Server, cookie *http.Cookie, name string, data []byte) string {
	t.Helper()
	url := tusCreate(t, srv, cookie, name, "", len(data))
	tusPatch(t, url, cookie, 0, string(data))
	return url[strings.LastIndex(url, "/")+1:]
}

func waitStatus(t *testing.T, srv *httptest.Server, cookie *http.Cookie, id string, statuses ...string) fullJob {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var last fullJob
	for time.Now().Before(deadline) {
		last = decode[fullJob](t, do(t, http.MethodGet, srv.URL+"/api/jobs/"+id, "", cookie))
		if slices.Contains(statuses, last.Status) {
			return last
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("job %s stuck in %s (want %v)", id, last.Status, statuses)
	return last
}

func items(t *testing.T, srv *httptest.Server, cookie *http.Cookie, id string) []itemJSON {
	t.Helper()
	return decode[[]itemJSON](t, do(t, http.MethodGet, srv.URL+"/api/jobs/"+id+"/items", "", cookie))
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(content))
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestRawFileGoesToReview(t *testing.T) {
	t.Parallel()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)

	const name = "INSIDE [0100D2D009028000][v0].nsp"
	id := upload(t, srv, cookie, name, []byte("PFS0 raw switch package"))
	waitStatus(t, srv, cookie, id, "review")

	got := items(t, srv, cookie, id)
	if len(got) != 1 || got[0].Path != name || !slices.Equal(got[0].Consoles, []string{"switch"}) ||
		got[0].Confidence != "header" || got[0].SuggestedKind == nil || *got[0].SuggestedKind != "base" {
		t.Fatalf("items = %+v", got)
	}
	if _, err := os.Stat(filepath.Join(stagingDir(library), id, name)); err != nil {
		t.Fatalf("raw file not adopted into staging: %v", err)
	}
	if _, err := os.Stat(filepath.Join(uploadsDir(library), id)); !os.IsNotExist(err) {
		t.Fatalf("upload data should have moved out of the tus store: %v", err)
	}
}

func TestZipIsExtractedVerifiedAndScanned(t *testing.T) {
	t.Parallel()
	require7zz(t)
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)

	archive := zipOf(t, map[string]string{
		"Rogue Prince BASE GAME/The Rogue Prince of Persia [01008D9022462000] [v0].nsp":                  "PFS0 base",
		"Rogue Prince BASE GAME/The Rogue Prince of Persia [01008D9022462800] [v131072]Update 1.0.4.nsp": "PFS0 update",
		"Rogue Prince BASE GAME/leeme.txt": "hola",
	})
	id := upload(t, srv, cookie, "rogue-prince.zip", archive)
	job := waitStatus(t, srv, cookie, id, "review", "failed")
	if job.Status != "review" || job.Received != int64(len(archive)) {
		t.Fatalf("job = %+v (error %v)", job, job.Error)
	}

	got := items(t, srv, cookie, id)
	if len(got) != 3 {
		t.Fatalf("items = %+v", got)
	}
	kinds := map[string]string{}
	for _, it := range got {
		if it.Ignored {
			kinds[it.Path] = "ignored"
		} else if it.SuggestedKind != nil {
			kinds[it.Path] = *it.SuggestedKind
		}
	}
	if kinds["Rogue Prince BASE GAME/leeme.txt"] != "ignored" ||
		kinds["Rogue Prince BASE GAME/The Rogue Prince of Persia [01008D9022462800] [v131072]Update 1.0.4.nsp"] != "update" {
		t.Fatalf("kinds = %v", kinds)
	}
	if _, err := os.Stat(filepath.Join(uploadsDir(library), id)); !os.IsNotExist(err) {
		t.Fatalf("archive must be deleted after a successful extraction: %v", err)
	}
}

func TestEncryptedArchiveWaitsForPassword(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv := newTestServer(t, nil)
	cookie := login(t, srv)

	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "Metroid Prime.iso"), append(make([]byte, 0x1C), 0xC2, 0x33, 0x9F, 0x3D), 0o600)
	archive := filepath.Join(t.TempDir(), "mp.7z")
	cmd := exec.CommandContext(t.Context(), "7zz", "a", "-bso0", "-bsp0", "-t7z", "-pS3cret", "-mhe=on", archive, ".")
	cmd.Dir = src
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("7zz a: %v %s", err, out)
	}
	data, _ := os.ReadFile(archive)

	id := upload(t, srv, cookie, "metroid-prime.7z", data)
	job := waitStatus(t, srv, cookie, id, "needs_password")
	if job.Error == nil || !strings.Contains(*job.Error, "contraseña") {
		t.Fatalf("needs_password reason = %v", job.Error)
	}

	if res := do(t, http.MethodPost, srv.URL+"/api/jobs/"+id+"/password", `{"password":"wrong"}`, cookie); res.StatusCode != http.StatusAccepted {
		t.Fatalf("submit wrong password = %d", res.StatusCode)
	}
	job = waitStatus(t, srv, cookie, id, "needs_password")
	deadline := time.Now().Add(10 * time.Second)
	for (job.Error == nil || !strings.Contains(*job.Error, "incorrecta")) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		job = decode[fullJob](t, do(t, http.MethodGet, srv.URL+"/api/jobs/"+id, "", cookie))
	}
	if job.Error == nil || !strings.Contains(*job.Error, "incorrecta") {
		t.Fatalf("wrong password reason = %v", job.Error)
	}

	do(t, http.MethodPost, srv.URL+"/api/jobs/"+id+"/password", `{"password":"S3cret"}`, cookie)
	waitStatus(t, srv, cookie, id, "review")
	got := items(t, srv, cookie, id)
	if len(got) != 1 || !slices.Equal(got[0].Consoles, []string{"gc"}) || got[0].Confidence != "header" {
		t.Fatalf("items = %+v", got)
	}

	if res := do(t, http.MethodPost, srv.URL+"/api/jobs/"+id+"/password", `{"password":"S3cret"}`, cookie); res.StatusCode != http.StatusConflict {
		t.Fatalf("password for a job in review = %d, want 409", res.StatusCode)
	}
}

func TestZipSlipIsRejected(t *testing.T) {
	t.Parallel()
	require7zz(t)
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)

	id := upload(t, srv, cookie, "evil.zip", zipOf(t, map[string]string{"../evil.txt": "pwned", "ok/game.iso": "data"}))
	job := waitStatus(t, srv, cookie, id, "failed", "review")
	if job.Status != "failed" || job.Error == nil || !strings.Contains(*job.Error, "rutas peligrosas") {
		t.Fatalf("job = %+v (error %v)", job, job.Error)
	}
	if _, err := os.Stat(filepath.Join(stagingDir(library), "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("nothing may be written outside the job directory")
	}
}

func TestCancelInReviewRemovesStaging(t *testing.T) {
	t.Parallel()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) { c.LibraryPath = library })
	cookie := login(t, srv)

	id := upload(t, srv, cookie, "game.nsp", []byte("PFS0 x"))
	waitStatus(t, srv, cookie, id, "review")
	if res := do(t, http.MethodPost, srv.URL+"/api/jobs/"+id+"/cancel", "", cookie); res.StatusCode != http.StatusOK {
		t.Fatalf("cancel = %d", res.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(stagingDir(library), id)); !os.IsNotExist(err) {
		t.Fatalf("staging dir still present: %v", err)
	}
}

func TestHealthReportsExtractor(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv := newTestServer(t, nil)
	h := decode[struct {
		Status string `json:"status"`
		Checks []struct {
			Name string `json:"name"`
			Ok   bool   `json:"ok"`
		} `json:"checks"`
	}](t, do(t, http.MethodGet, srv.URL+"/api/health", ""))
	found := false
	for _, c := range h.Checks {
		if c.Name == "extractor" {
			found = c.Ok
		}
	}
	if !found || h.Status != "ok" {
		t.Fatalf("health = %+v", h)
	}
}

// TestSeedMatchesDetectionVectors runs the shared golden vectors against the
// consoles actually seeded in the database, so seed and detection rules can
// never drift apart.
func TestSeedMatchesDetectionVectors(t *testing.T) {
	t.Parallel()
	cfg := config.Config{
		Port: 8080, LibraryPath: t.TempDir(), DataPath: t.TempDir(), Password: testPassword,
		SessionSecret: strings.Repeat("s", config.MinSessionSecretBytes), SessionTTL: 3600e9, ExtractConcurrency: 1,
	}
	a, err := newApp(t.Context(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	profiles, err := consoleProfiles{a.consoles}.Profiles(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile("../../../../contracts/detection-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				FileName string `json:"fileName"`
			} `json:"input"`
			Expected struct {
				Consoles []string `json:"consoles"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	for _, c := range file.Cases {
		got := slices.Sorted(slices.Values(detection.FromName(c.Input.FileName, profiles).Consoles))
		want := slices.Sorted(slices.Values(c.Expected.Consoles))
		if !slices.Equal(got, want) {
			t.Errorf("%s: seeded consoles give %v, want %v", c.Name, got, want)
		}
	}
}
