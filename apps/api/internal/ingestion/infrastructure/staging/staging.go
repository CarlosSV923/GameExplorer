// Package staging implements the per-job working area: one directory per
// job inside the library dataset (LIBRARY_PATH/.gameexplorer/staging/
// extracted/<job>), so the final move into the library is a rename.
package staging

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/diskspace"
)

// Area implements application.Staging.
type Area struct {
	root    string
	volumes string
}

var _ application.Staging = (*Area)(nil)

// New builds the area: job directories under root, multi-volume parts under
// volumes (both created on first use).
func New(root, volumes string) *Area {
	return &Area{root: root, volumes: volumes}
}

// VolumeDir implements application.Staging. The set name comes from a file
// name, so it is hashed into a safe directory name.
func (a *Area) VolumeDir(set string) string {
	sum := sha256.Sum256([]byte(set))
	return filepath.Join(a.volumes, hex.EncodeToString(sum[:8]))
}

// RemoveVolumes implements application.Staging.
func (a *Area) RemoveVolumes(set string) error {
	return os.RemoveAll(a.VolumeDir(set))
}

// RemovePart implements application.Staging; it only deletes inside the
// volumes area.
func (a *Area) RemovePart(path string) error {
	rel, err := filepath.Rel(a.volumes, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("staging: refusing to delete %s outside the volumes area", path)
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Dir implements application.Staging. Job ids are tus ids (hex), validated by
// the API, so joining them is safe.
func (a *Area) Dir(id domain.JobID) string {
	return filepath.Join(a.root, filepath.Base(string(id)))
}

// Reset implements application.Staging.
func (a *Area) Reset(id domain.JobID) error {
	if err := os.RemoveAll(a.Dir(id)); err != nil {
		return err
	}
	return os.MkdirAll(a.Dir(id), 0o750)
}

// Remove implements application.Staging.
func (a *Area) Remove(id domain.JobID) error {
	return os.RemoveAll(a.Dir(id))
}

// CheckTree implements application.Staging.
func (a *Area) CheckTree(id domain.JobID) error {
	root := a.Dir(id)
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Type().IsRegular() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		return fmt.Errorf("%w: %s (%s)", application.ErrUnsafeEntry, rel, d.Type())
	})
}

// FS implements application.Staging.
func (a *Area) FS(id domain.JobID) fs.FS {
	return os.DirFS(a.Dir(id))
}

// FreeSpace implements application.Staging.
func (a *Area) FreeSpace() (uint64, error) {
	dir := a.root
	for {
		if _, err := os.Stat(dir); err == nil {
			return diskspace.Free(dir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return 0, errors.New("staging: no existing parent directory")
		}
		dir = parent
	}
}

// ReadHeader implements application.Staging.
func (a *Area) ReadHeader(path string, n int) ([]byte, error) {
	f, err := os.Open(path) //nolint:gosec // the upload path comes from our own tus store
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	read, err := io.ReadFull(f, buf)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, err
	}
	return buf[:read], nil
}
