package imagecache_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/infrastructure/imagecache"
)

func TestCacheDownloadsOnceAndServesFromDisk(t *testing.T) {
	t.Parallel()

	var hits atomic.Int32
	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(50 * time.Millisecond) // widen the race window for concurrent callers
		switch r.URL.Path {
		case "/t_logo_med/plgu.png":
			_, _ = w.Write([]byte("PNGDATA"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(cdn.Close)

	dir := t.TempDir()
	cache := imagecache.New(dir, cdn.URL, nil)
	ref, _ := domain.NewImageRef("logo_med", "plgu")

	var wg sync.WaitGroup
	for range 5 {
		wg.Go(func() {
			rc, n, err := cache.Open(t.Context(), ref)
			if err != nil {
				t.Errorf("Open: %v", err)
				return
			}
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			if string(b) != "PNGDATA" || n != 7 {
				t.Errorf("got %q (%d bytes)", b, n)
			}
		})
	}
	wg.Wait()
	if h := hits.Load(); h != 1 {
		t.Fatalf("CDN hits = %d, want 1 (shared download)", h)
	}
	if _, err := os.Stat(filepath.Join(dir, "logo_med", "plgu.png")); err != nil {
		t.Fatalf("not cached on disk: %v", err)
	}

	rc, _, err := cache.Open(t.Context(), ref)
	if err != nil {
		t.Fatal(err)
	}
	_ = rc.Close()
	if h := hits.Load(); h != 1 {
		t.Fatalf("CDN hits after cache = %d, want 1", h)
	}

	missing, _ := domain.NewImageRef("cover_big", "nope")
	if _, _, err := cache.Open(t.Context(), missing); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing image err = %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "cover_big"))
	if len(entries) != 0 {
		t.Fatalf("failed download left files behind: %v", entries)
	}
}

func TestCacheReportsUpstreamErrors(t *testing.T) {
	t.Parallel()

	cdn := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(cdn.Close)

	ref, _ := domain.NewImageRef("cover_big", "co213p")
	if _, _, err := imagecache.New(t.TempDir(), cdn.URL, nil).Open(t.Context(), ref); !errors.Is(err, domain.ErrUpstream) {
		t.Fatalf("err = %v", err)
	}
}
