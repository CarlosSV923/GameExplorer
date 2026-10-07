// Package tus adapts tusd (the reference tus server) to the ingestion
// context: resumable uploads of any size, streamed straight to disk.
package tus

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tus/tusd/v2/pkg/filelocker"
	"github.com/tus/tusd/v2/pkg/filestore"
	tusd "github.com/tus/tusd/v2/pkg/handler"
	expslog "golang.org/x/exp/slog"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
)

// Metadata keys the browser sends with Upload-Metadata.
const (
	MetaFileName = "filename"
	MetaConsole  = "consoleSlug"
)

// FreeSpaceMargin is kept free on top of every upload's size.
const FreeSpaceMargin = 1 << 30 // 1 GiB

// FreeSpaceFunc reports the free bytes of the filesystem holding path.
type FreeSpaceFunc func(path string) (uint64, error)

// Hooks receives upload lifecycle events (implemented by application.Service).
type Hooks interface {
	UploadCreated(ctx context.Context, id domain.JobID, fileName string, size int64, origin string) error
	UploadProgress(ctx context.Context, id domain.JobID, received int64) error
	UploadFinished(ctx context.Context, id domain.JobID, storagePath string) error
	UploadTerminated(ctx context.Context, id domain.JobID) error
}

// Adapter owns the tusd handler and its file store.
type Adapter struct {
	dir     string
	store   filestore.FileStore
	handler *tusd.Handler
	log     *slog.Logger
}

var _ application.UploadStore = (*Adapter)(nil)

// New creates the store in dir (inside the library dataset, so later moves
// are a rename) and the tus handler served under basePath.
func New(dir, basePath string, log *slog.Logger, freeSpace FreeSpaceFunc) (*Adapter, error) {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("tus: create upload dir: %w", err)
	}
	a := &Adapter{dir: dir, store: filestore.New(dir), log: log}

	composer := tusd.NewStoreComposer()
	a.store.UseIn(composer)
	filelocker.New(dir).UseIn(composer)

	h, err := tusd.NewHandler(tusd.Config{
		BasePath:                basePath,
		StoreComposer:           composer,
		NotifyCreatedUploads:    true,
		NotifyUploadProgress:    true,
		NotifyCompleteUploads:   true,
		NotifyTerminatedUploads: true,
		UploadProgressInterval:  time.Second,
		DisableDownload:         true, // finished files are served by the catalog, never by tus
		DisableConcatenation:    true,
		// tusd logs every request at info level (with its own slog type); keep only warnings.
		Logger: expslog.New(expslog.NewJSONHandler(os.Stdout, &expslog.HandlerOptions{Level: expslog.LevelWarn})),
		PreUploadCreateCallback: func(ev tusd.HookEvent) (tusd.HTTPResponse, tusd.FileInfoChanges, error) {
			return tusd.HTTPResponse{}, tusd.FileInfoChanges{}, validateNewUpload(ev.Upload, dir, freeSpace)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("tus: %w", err)
	}
	a.handler = h
	return a, nil
}

func validateNewUpload(info tusd.FileInfo, dir string, freeSpace FreeSpaceFunc) error {
	if _, err := domain.CleanFileName(info.MetaData[MetaFileName]); err != nil {
		return tusd.NewError("ERR_INVALID_FILENAME", "Upload-Metadata must include a valid filename", http.StatusBadRequest)
	}
	if info.SizeIsDeferred {
		return tusd.NewError("ERR_SIZE_REQUIRED", "the upload size must be declared up front", http.StatusBadRequest)
	}
	if freeSpace != nil {
		free, err := freeSpace(dir)
		if err == nil && uint64(info.Size)+FreeSpaceMargin > free { //nolint:gosec // size is non-negative (tusd validates it)
			return tusd.NewError("ERR_INSUFFICIENT_STORAGE",
				fmt.Sprintf("not enough free space: need %d bytes, %d available", uint64(info.Size)+FreeSpaceMargin, free), //nolint:gosec // see above
				http.StatusInsufficientStorage)
		}
	}
	return nil
}

// Handler is the tus HTTP endpoint; mount it at basePath with StripPrefix.
func (a *Adapter) Handler() http.Handler { return a.handler }

// Run forwards tusd events to hooks until ctx ends.
func (a *Adapter) Run(ctx context.Context, hooks Hooks) {
	for {
		var err error
		select {
		case <-ctx.Done():
			return
		case ev := <-a.handler.CreatedUploads:
			err = hooks.UploadCreated(ctx, domain.JobID(ev.Upload.ID), ev.Upload.MetaData[MetaFileName],
				ev.Upload.Size, ev.Upload.MetaData[MetaConsole])
		case ev := <-a.handler.UploadProgress:
			err = hooks.UploadProgress(ctx, domain.JobID(ev.Upload.ID), ev.Upload.Offset)
		case ev := <-a.handler.CompleteUploads:
			err = hooks.UploadFinished(ctx, domain.JobID(ev.Upload.ID), ev.Upload.Storage["Path"])
		case ev := <-a.handler.TerminatedUploads:
			err = hooks.UploadTerminated(ctx, domain.JobID(ev.Upload.ID))
		}
		if err != nil && ctx.Err() == nil {
			a.log.Warn("upload event", "error", err)
		}
	}
}

// Delete implements application.UploadStore.
func (a *Adapter) Delete(ctx context.Context, id domain.JobID) error {
	up, err := a.store.GetUpload(ctx, string(id))
	if errors.Is(err, tusd.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := a.store.AsTerminatableUpload(up).Terminate(ctx); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// IDs implements application.UploadStore.
func (a *Adapter) IDs(context.Context) ([]domain.JobID, error) {
	infos, err := filepath.Glob(filepath.Join(a.dir, "*.info"))
	if err != nil {
		return nil, err
	}
	ids := make([]domain.JobID, 0, len(infos))
	for _, p := range infos {
		ids = append(ids, domain.JobID(strings.TrimSuffix(filepath.Base(p), ".info")))
	}
	return ids, nil
}
