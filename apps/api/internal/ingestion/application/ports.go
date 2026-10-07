package application

import (
	"context"
	"errors"
	"io/fs"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/detection"
)

// Extraction errors returned by an Extractor.
var (
	// ErrPasswordRequired means the archive is encrypted and no password was given.
	ErrPasswordRequired = errors.New("archive password required")
	// ErrWrongPassword means the given password does not open the archive.
	ErrWrongPassword = errors.New("wrong archive password")
	// ErrCorrupt means the archive is damaged or an extracted file failed verification.
	ErrCorrupt = errors.New("archive is corrupt or failed verification")
)

// ArchiveEntry is one entry of an archive listing.
type ArchiveEntry struct {
	Path      string
	Size      int64
	CRC       string // upper-case hex CRC32, "" when the format has none
	IsDir     bool
	Encrypted bool
}

// Listing is an archive's table of contents.
type Listing struct {
	Entries []ArchiveEntry
}

// TotalSize is the uncompressed size of every file.
func (l Listing) TotalSize() int64 {
	var n int64
	for _, e := range l.Entries {
		if !e.IsDir {
			n += e.Size
		}
	}
	return n
}

// Encrypted reports whether any entry needs a password.
func (l Listing) Encrypted() bool {
	for _, e := range l.Entries {
		if e.Encrypted {
			return true
		}
	}
	return false
}

// Extractor lists and extracts archives (7-Zip).
type Extractor interface {
	// List returns the archive's entries. Header-encrypted archives fail with
	// ErrPasswordRequired (no password) or ErrWrongPassword.
	List(ctx context.Context, archive, password string) (Listing, error)
	// Extract unpacks archive into dest and verifies every file against
	// listing (size + CRC32). The warning is non-fatal (e.g. file attributes
	// that the dataset's ACL does not allow to set).
	Extract(ctx context.Context, archive, dest, password string, listing Listing, progress func(pct int)) (warning string, err error)
}

// Staging is the per-job working area inside the library dataset.
type Staging interface {
	// Dir is the job's directory (extraction target).
	Dir(id domain.JobID) string
	// Reset empties (or creates) the job's directory.
	Reset(id domain.JobID) error
	// Remove deletes the job's directory.
	Remove(id domain.JobID) error
	// CheckTree rejects anything that is not a regular file or directory
	// (symlinks could point outside the library).
	CheckTree(id domain.JobID) error
	// FS is a read-only view of the job's directory.
	FS(id domain.JobID) fs.FS
	// FreeSpace is the free space of the library's file system.
	FreeSpace() (uint64, error)
	// ReadHeader reads the first n bytes of a file.
	ReadHeader(path string, n int) ([]byte, error)
}

// UploadAdopter moves a finished raw upload out of the upload store.
type UploadAdopter interface {
	// Take renames the upload's data to dest and forgets the upload.
	Take(ctx context.Context, id domain.JobID, dest string) error
}

// ProfileSource provides the consoles known to the catalog for detection.
type ProfileSource interface {
	Profiles(ctx context.Context) ([]detection.ConsoleProfile, error)
}
