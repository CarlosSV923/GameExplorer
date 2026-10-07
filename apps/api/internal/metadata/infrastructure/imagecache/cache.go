// Package imagecache stores IGDB images on disk (DATA_PATH/cache/images) so
// covers and logos are fetched from the CDN once and then served locally.
package imagecache

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
)

// DefaultBaseURL is the IGDB image CDN.
const DefaultBaseURL = "https://images.igdb.com/igdb/image/upload"

const maxImageBytes = 10 << 20

// Cache implements application.ImageStore.
type Cache struct {
	dir     string
	baseURL string
	http    *http.Client
	group   singleflight.Group
}

var _ application.ImageStore = (*Cache)(nil)

// New builds a cache rooted at dir. baseURL may be empty (IGDB CDN).
func New(dir, baseURL string, hc *http.Client) *Cache {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	if hc == nil {
		hc = &http.Client{Timeout: 20 * time.Second}
	}
	return &Cache{dir: dir, baseURL: baseURL, http: hc}
}

func (c *Cache) path(ref domain.ImageRef) string {
	// Size and id are validated by domain.NewImageRef ([a-z0-9_]), so this is safe.
	return filepath.Join(c.dir, string(ref.Size), ref.ID+ref.Extension())
}

// Open implements application.ImageStore.
func (c *Cache) Open(ctx context.Context, ref domain.ImageRef) (io.ReadCloser, int64, error) {
	if f, n, err := openFile(c.path(ref)); err == nil {
		return f, n, nil
	}
	// Concurrent requests for the same image share one download.
	_, err, _ := c.group.Do(c.path(ref), func() (any, error) {
		return nil, c.download(context.WithoutCancel(ctx), ref)
	})
	if err != nil {
		return nil, 0, err
	}
	return openFile(c.path(ref))
}

func openFile(p string) (io.ReadCloser, int64, error) {
	f, err := os.Open(p) //nolint:gosec // path built from a validated ImageRef
	if err != nil {
		return nil, 0, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, 0, err
	}
	return f, info.Size(), nil
}

func (c *Cache) download(ctx context.Context, ref domain.ImageRef) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/t_%s/%s%s", c.baseURL, ref.Size, ref.ID, ref.Extension())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return errors.Join(domain.ErrUpstream, err)
	}
	defer res.Body.Close()
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusForbidden:
		return domain.ErrNotFound
	case res.StatusCode != http.StatusOK:
		return fmt.Errorf("%w: image CDN returned %d", domain.ErrUpstream, res.StatusCode)
	}

	dest := c.path(ref)
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	// Write to a temp file in the same directory, then rename: readers never
	// see a half-written image, even if the process dies mid-download.
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	n, err := io.Copy(tmp, io.LimitReader(res.Body, maxImageBytes+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return errors.Join(domain.ErrUpstream, err)
	}
	if n > maxImageBytes {
		return fmt.Errorf("%w: image larger than %d bytes", domain.ErrUpstream, maxImageBytes)
	}
	return os.Rename(tmp.Name(), dest)
}
