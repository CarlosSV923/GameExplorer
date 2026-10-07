// Package libraryfs implements the catalog's file system port on the library
// dataset. Staging and trash live inside the library, so moves are renames;
// across file systems (EXDEV, e.g. a console folder that is its own dataset)
// a move becomes copy + delete.
package libraryfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
)

// partialSuffix marks a cross-device copy in progress; the destination name
// only appears once the copy is complete.
const partialSuffix = ".gameexplorer-partial"

// FS implements application.Files.
type FS struct {
	root    string
	trash   string
	scratch string
}

var _ application.Files = (*FS)(nil)

// New builds the file system for a library root; trash and scratch must be
// inside it.
func New(root, trash, scratch string) *FS {
	return &FS{root: filepath.Clean(root), trash: filepath.Clean(trash), scratch: filepath.Clean(scratch)}
}

// LibraryPath implements application.Files.
func (f *FS) LibraryPath(elem ...string) string {
	return filepath.Join(append([]string{f.root}, elem...)...)
}

// TrashPath implements application.Files.
func (f *FS) TrashPath(elem ...string) string {
	return filepath.Join(append([]string{f.trash}, elem...)...)
}

// ScratchPath implements application.Files.
func (f *FS) ScratchPath(elem ...string) string {
	return filepath.Join(append([]string{f.scratch}, elem...)...)
}

// List implements application.Files.
func (f *FS) List(dir string) ([]string, error) {
	if err := f.inside(dir); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, nil
}

// RemoveAll implements application.Files.
func (f *FS) RemoveAll(p string) error {
	if err := f.inside(p); err != nil {
		return err
	}
	return os.RemoveAll(p)
}

// inside rejects paths outside the library: every path is built by the
// server, so this is a last line of defence against a naming bug.
func (f *FS) inside(paths ...string) error {
	for _, p := range paths {
		rel, err := filepath.Rel(f.root, p)
		if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || !filepath.IsAbs(p) {
			return fmt.Errorf("libraryfs: refusing %q outside the library", p)
		}
	}
	return nil
}

// Exists implements application.Files.
func (f *FS) Exists(p string) (bool, error) {
	if err := f.inside(p); err != nil {
		return false, err
	}
	_, err := os.Lstat(p)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	}
	return false, err
}

// MkdirAll implements application.Files.
func (f *FS) MkdirAll(dir string) ([]string, error) {
	if err := f.inside(dir); err != nil {
		return nil, err
	}
	var missing []string
	for d := dir; d != f.root; d = filepath.Dir(d) {
		if _, err := os.Lstat(d); err == nil {
			break
		}
		missing = append([]string{d}, missing...)
	}
	var created []string
	for _, d := range missing {
		if err := os.Mkdir(d, 0o775); err != nil && !errors.Is(err, fs.ErrExist) { //nolint:gosec // library folders must stay editable over SMB
			return created, err
		}
		created = append(created, d)
	}
	return created, nil
}

// Move implements application.Files.
func (f *FS) Move(from, to string) error {
	if err := f.inside(from, to); err != nil {
		return err
	}
	if _, err := os.Lstat(to); err == nil {
		return fmt.Errorf("%w: %s", fs.ErrExist, to)
	}
	err := os.Rename(from, to)
	if !errors.Is(err, syscall.EXDEV) {
		return err
	}
	partial := to + partialSuffix
	_ = os.RemoveAll(partial)
	if err := copyTree(from, partial); err != nil {
		_ = os.RemoveAll(partial)
		return fmt.Errorf("copy across file systems: %w", err)
	}
	if err := os.Rename(partial, to); err != nil {
		_ = os.RemoveAll(partial)
		return err
	}
	return os.RemoveAll(from)
}

func copyTree(from, to string) error {
	info, err := os.Lstat(from)
	if err != nil {
		return err
	}
	switch {
	case info.Mode().IsRegular():
		return copyFile(from, to)
	case !info.IsDir():
		return fmt.Errorf("%s: not a regular file or directory", from)
	}
	if err := os.Mkdir(to, 0o775); err != nil { //nolint:gosec // idem
		return err
	}
	entries, err := os.ReadDir(from)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if err := copyTree(filepath.Join(from, e.Name()), filepath.Join(to, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from) //nolint:gosec // paths are checked by inside()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o664) //nolint:gosec // idem
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// RemoveEmptyDir implements application.Files.
func (f *FS) RemoveEmptyDir(dir string) error {
	if err := f.inside(dir); err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil || len(entries) > 0 {
		return err
	}
	return os.Remove(dir)
}

// ReadFile implements application.Files.
func (f *FS) ReadFile(p string, limit int64) ([]byte, error) {
	if err := f.inside(p); err != nil {
		return nil, err
	}
	file, err := os.Open(p) //nolint:gosec // checked by inside()
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s is larger than %d bytes", filepath.Base(p), limit)
	}
	return data, nil
}

// WriteFile implements application.Files.
func (f *FS) WriteFile(p string, data []byte) error {
	if err := f.inside(p); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o664) //nolint:gosec // library files must stay editable over SMB
}

// Tree implements application.Files.
func (f *FS) Tree(p string) ([]application.TreeFile, error) {
	if err := f.inside(p); err != nil {
		return nil, err
	}
	info, err := os.Lstat(p)
	if err != nil {
		return nil, err
	}
	if info.Mode().IsRegular() {
		return []application.TreeFile{{Path: p, Size: info.Size(), Modified: info.ModTime()}}, nil
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s: not a regular file or directory", p)
	}
	var out []application.TreeFile
	err = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.Type().IsRegular() {
			return err // links and devices are skipped
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(p, path)
		if err != nil {
			return err
		}
		out = append(out, application.TreeFile{Path: path, Rel: filepath.ToSlash(rel), Size: info.Size(), Modified: info.ModTime()})
		return nil
	})
	return out, err
}

// Open implements application.Files.
func (f *FS) Open(p string) (io.ReadSeekCloser, error) {
	if err := f.inside(p); err != nil {
		return nil, err
	}
	return os.Open(p) //nolint:gosec // checked by inside()
}
