package tus_test

import (
	"context"
	"encoding/base64"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/tus"
)

func newServer(t *testing.T, free uint64) *httptest.Server {
	t.Helper()
	a, err := tus.New(t.TempDir(), "/files/", slog.New(slog.NewTextHandler(io.Discard, nil)),
		func(string) (uint64, error) { return free, nil })
	if err != nil {
		t.Fatal(err)
	}
	// tusd blocks on its event channels until someone consumes them.
	go a.Run(t.Context(), noopHooks{})
	srv := httptest.NewServer(http.StripPrefix("/files/", a.Handler()))
	t.Cleanup(srv.Close)
	return srv
}

type noopHooks struct{}

func (noopHooks) UploadCreated(context.Context, domain.JobID, string, int64, string) error {
	return nil
}

func (noopHooks) UploadProgress(context.Context, domain.JobID, int64) error { return nil }

func (noopHooks) UploadFinished(context.Context, domain.JobID, string) error { return nil }

func (noopHooks) UploadTerminated(context.Context, domain.JobID) error { return nil }

func create(t *testing.T, url string, size int64, metadata string) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, url+"/files/", nil)
	req.Header.Set("Tus-Resumable", "1.0.0")
	req.Header.Set("Upload-Length", strconv.FormatInt(size, 10))
	if metadata != "" {
		req.Header.Set("Upload-Metadata", metadata)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func meta(name string) string {
	return "filename " + base64.StdEncoding.EncodeToString([]byte(name))
}

func TestCreateValidatesFileNameAndSpace(t *testing.T) {
	t.Parallel()
	srv := newServer(t, 50<<30) // 50 GiB free

	if res := create(t, srv.URL, 10, meta("INSIDE.nsp")); res.StatusCode != http.StatusCreated {
		t.Fatalf("valid upload status = %d", res.StatusCode)
	}
	if res := create(t, srv.URL, 10, ""); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing filename status = %d, want 400", res.StatusCode)
	}
	if res := create(t, srv.URL, 10, meta("..")); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad filename status = %d, want 400", res.StatusCode)
	}
	// 49.5 GiB + 1 GiB margin does not fit in 50 GiB.
	if res := create(t, srv.URL, 49<<30+512<<20, meta("big.rar")); res.StatusCode != http.StatusInsufficientStorage {
		t.Fatalf("too big status = %d, want 507", res.StatusCode)
	}
}
