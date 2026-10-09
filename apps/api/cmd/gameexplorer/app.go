package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	catalogapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	catalogconsoles "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain/consoles"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/libraryfs"
	catalogsqlite "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite"
	cataloghttp "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/interfaces/http"
	identityapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/application"
	identityinfra "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/infrastructure"
	identityhttp "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/interfaces/http"
	ingestionapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	ingestiondomain "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/broker"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sevenzip"
	ingestionsqlite "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sqlite"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/staging"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/tus"
	ingestionhttp "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/interfaces/http"
	metadataapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/application"
	metadatadomain "github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/infrastructure/igdb"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/infrastructure/imagecache"
	metadatahttp "github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/interfaces/http"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/diskspace"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpapi"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpx"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/system"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/web"
)

// Login throttle: 5 attempts at once, then one more every 12 seconds (5/min).
const (
	loginBurst  = 5
	loginRefill = 12 * time.Second
)

// Aliases give each embedded handler its own field name (they are all "Handler").
type (
	systemAPI   = system.Handler
	identityAPI = identityhttp.Handler
	catalogAPI  = cataloghttp.Handler
	metadataAPI = metadatahttp.Handler
	jobsAPI     = ingestionhttp.Handler
)

// api assembles the bounded contexts' handlers into the generated interface.
// Each embedded handler implements a disjoint subset of the operations.
type api struct {
	*systemAPI
	*identityAPI
	*catalogAPI
	*metadataAPI
	*jobsAPI
}

var _ httpapi.StrictServerInterface = api{}

// app is the fully wired application.
type app struct {
	handler   http.Handler
	db        *sql.DB
	log       *slog.Logger
	consoles  *catalogapp.ConsoleService
	metadata  *metadataapp.Service
	ingestion *ingestionapp.Service
	processor *ingestionapp.Processor
	committer *ingestionapp.Committer
	library   *catalogapp.LibraryService
	uploads   *tus.Adapter // nil when the library is not writable
	cfg       config.Config

	shutdown     chan struct{}
	shutdownOnce sync.Once
}

func (a *app) Close() error {
	a.beginShutdown()
	return a.db.Close()
}

// beginShutdown ends long-lived responses (event streams) so the HTTP
// server's graceful shutdown does not wait for them.
func (a *app) beginShutdown() { a.shutdownOnce.Do(func() { close(a.shutdown) }) }

// start runs the background work: upload events, the hourly purge of
// abandoned uploads and the console metadata sync. Library changes that a
// restart interrupted are undone first, before anything else touches files.
func (a *app) start(ctx context.Context) {
	if err := a.committer.Recover(ctx); err != nil {
		a.log.Error("recover interrupted commits", "error", err)
	}
	if a.uploads != nil {
		go a.uploads.Run(ctx, a.ingestion)
		go a.ingestion.RunPurge(ctx, time.Hour)
		go a.processor.Run(ctx, a.cfg.ExtractConcurrency)
		go a.library.RunMaintenance(ctx, a.cfg.ScanInterval, a.trashRetention())
	}
	go a.syncConsoles(ctx, a.log)
}

func (a *app) trashRetention() time.Duration {
	return time.Duration(a.cfg.TrashRetentionDays) * 24 * time.Hour
}

// stagingDir holds each job's extracted files until they are committed.
func stagingDir(libraryPath string) string {
	return filepath.Join(libraryPath, ".gameexplorer", "staging", "extracted")
}

// trashDir keeps replaced and deleted items until they are purged (RF-30).
func trashDir(libraryPath string) string {
	return filepath.Join(libraryPath, ".gameexplorer", "trash")
}

// scratchDir holds the scratch files of library operations in progress.
func scratchDir(libraryPath string) string {
	return filepath.Join(libraryPath, ".gameexplorer", "ops")
}

// volumesDir gathers the parts of multi-volume archives until all arrive.
func volumesDir(libraryPath string) string {
	return filepath.Join(libraryPath, ".gameexplorer", "staging", "volumes")
}

// uploadsDir is where tus stores incoming files: inside the library dataset,
// so moving a finished game into place is a rename, not a copy.
func uploadsDir(libraryPath string) string {
	return filepath.Join(libraryPath, ".gameexplorer", "staging", "uploads")
}

// unassignedDir is the unassigned section: unknown files found by the scan
// and uploads that did not fit their console (RF-27).
func unassignedDir(libraryPath string) string {
	return filepath.Join(libraryPath, "_unassigned")
}

// sourcesDir holds the files that jobs took from the unassigned section.
func sourcesDir(libraryPath string) string {
	return filepath.Join(libraryPath, ".gameexplorer", "staging", "sources")
}

// newApp wires every adapter. It is the only place that knows them all.
func newApp(ctx context.Context, cfg config.Config, log *slog.Logger) (*app, error) {
	// Without its data folder the app cannot even keep sessions: say why
	// instead of SQLite's "unable to open database file".
	if err := system.WritableDir("data-writable", cfg.DataPath).Run(ctx); err != nil {
		return nil, fmt.Errorf("DATA_PATH: %w", err)
	}
	db, err := database.Open(ctx, cfg.DataPath)
	if err != nil {
		return nil, err
	}

	verifier, err := passwordVerifier(cfg)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	identity := identityapp.NewService(
		verifier,
		identityinfra.NewHMACTokenCodec([]byte(cfg.SessionSecret)),
		identityinfra.NewMemoryThrottle(loginRefill, loginBurst, nil),
		cfg.SessionTTL,
		nil,
	)
	identityHandler := identityhttp.NewHandler(identity, cfg.CookieSecure)

	var provider metadataapp.Provider // nil when IGDB is not configured
	if cfg.IGDBConfigured() {
		provider = igdb.New(igdb.Config{
			ClientID:     cfg.IGDBClientID,
			ClientSecret: cfg.IGDBClientSecret,
			APIURL:       cfg.IGDBAPIURL,
			TokenURL:     cfg.IGDBTokenURL,
		})
	}
	metadata := metadataapp.NewService(provider,
		imagecache.New(filepath.Join(cfg.DataPath, "cache", "images"), cfg.IGDBImageURL, nil))

	consoles, err := catalogapp.NewConsoleService(catalogconsoles.All(), catalogsqlite.NewConsoleRepository(db), cfg.Extensions)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	rules := consoleRules{consoles}

	events := broker.New()
	jobs := ingestionsqlite.NewJobRepository(db)
	stagedFiles := ingestionsqlite.NewFileRepository(db)
	var ingestion *ingestionapp.Service // the tus validator below runs only after it is set
	uploads, uploadsErr := tus.New(uploadsDir(cfg.LibraryPath), "/api/uploads/", log, diskspace.Free,
		func(ctx context.Context, m ingestionapp.UploadMeta) error {
			_, err := ingestion.SpecFromMeta(ctx, m)
			return err
		})
	var store ingestionapp.UploadStore = unavailableStore{}
	if uploadsErr != nil {
		log.Error("uploads disabled: library not writable", "error", uploadsErr)
	} else {
		store = uploads
	}
	extractor := sevenzip.New("")
	stagingArea := staging.New(stagingDir(cfg.LibraryPath), volumesDir(cfg.LibraryPath), sourcesDir(cfg.LibraryPath))
	ingestion = ingestionapp.NewService(jobs, store, rules, events, log, nil)
	libraryRepo := catalogsqlite.NewLibraryRepository(db)
	libraryFiles := libraryfs.New(cfg.LibraryPath, trashDir(cfg.LibraryPath), scratchDir(cfg.LibraryPath), unassignedDir(cfg.LibraryPath))
	library := catalogapp.NewLibraryService(consoles, libraryRepo, gameDirectory{metadata}, libraryFiles, log, nil)
	browse := catalogapp.NewBrowseService(consoles, libraryRepo, libraryFiles)
	committer := ingestionapp.NewCommitter(jobs, stagedFiles, stagingArea, libraryPort{library}, rules, events, log, nil)
	var processor *ingestionapp.Processor
	if uploadsErr == nil {
		processor = ingestionapp.NewProcessor(ingestionapp.ProcessorDeps{
			Jobs: jobs, Files: stagedFiles, Uploads: uploads, Extractor: extractor,
			Staging: stagingArea, Consoles: rules, Library: libraryPort{library},
			Publisher: events, Log: log,
		})
		ingestion.WithQueue(processor).WithUnassigned(libraryPort{library}, stagingArea)
	}
	shutdown := make(chan struct{})

	server := api{
		systemAPI: system.NewHandler(
			system.WritableDir("library-writable", cfg.LibraryPath),
			system.WritableDir("data-writable", cfg.DataPath),
			system.Check{Name: "extractor", Run: func(context.Context) error {
				if !extractor.Available() {
					return errors.New("7-Zip (7zz) no está instalado: los comprimidos no se pueden extraer")
				}
				return nil
			}},
		),
		identityAPI: identityHandler,
		catalogAPI: cataloghttp.NewHandler(consoles).WithLibrary(browse, log).
			WithManagement(library, time.Duration(cfg.TrashRetentionDays)*24*time.Hour),
		metadataAPI: metadatahttp.NewHandler(metadata, log),
		jobsAPI:     ingestionhttp.NewHandler(ingestion, passwordSubmitter{processor}, events, shutdown).WithCommits(committer),
	}

	mux := http.NewServeMux()
	strict := httpapi.NewStrictHandlerWithOptions(server,
		[]httpapi.StrictMiddlewareFunc{identityHandler.Middleware()},
		httpapi.StrictHTTPServerOptions{
			RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
				httpx.WriteProblem(w, http.StatusBadRequest, "Bad Request", err.Error())
			},
			ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
				log.ErrorContext(r.Context(), "handler error", "path", r.URL.Path, "error", err)
				httpx.WriteProblem(w, http.StatusInternalServerError, "Internal Server Error", "")
			},
		})
	httpapi.HandlerWithOptions(strict, httpapi.StdHTTPServerOptions{
		BaseURL:    "/api",
		BaseRouter: mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, err error) {
			httpx.WriteProblem(w, http.StatusBadRequest, "Bad Request", err.Error())
		},
	})
	if uploadsErr == nil {
		mux.Handle("/api/uploads/", identityHandler.RequireSession(
			http.StripPrefix("/api/uploads/", uploads.Handler())))
	} else {
		mux.Handle("/api/uploads/", identityHandler.RequireSession(http.HandlerFunc(
			func(w http.ResponseWriter, _ *http.Request) {
				httpx.WriteProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
					"No se puede escribir en la biblioteca; revisa /api/health. "+uploadsErr.Error())
			})))
	}
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteProblem(w, http.StatusNotFound, "Not Found", "")
	})
	mux.Handle("/", web.Handler())

	handler := httpx.Chain(mux,
		httpx.Recover(log),
		httpx.Logging(log),
		httpx.SecurityHeaders,
		httpx.WithClientIP,
		httpx.WithRequest,
	)
	a := &app{
		handler: handler, db: db, log: log, consoles: consoles, metadata: metadata,
		ingestion: ingestion, processor: processor, committer: committer, library: library, shutdown: shutdown, cfg: cfg,
	}
	if uploadsErr == nil {
		a.uploads = uploads
	}
	return a, nil
}

// passwordSubmitter answers 409 when uploads (and so processing) are disabled.
type passwordSubmitter struct{ p *ingestionapp.Processor }

func (s passwordSubmitter) SubmitPassword(ctx context.Context, id ingestiondomain.JobID, password string) (*ingestiondomain.UploadJob, error) {
	if s.p == nil {
		return nil, ingestionapp.ErrNotWaitingForPassword
	}
	return s.p.SubmitPassword(ctx, id, password)
}

// unavailableStore stands in for the tus store when the library is not writable.
type unavailableStore struct{}

func (unavailableStore) Delete(context.Context, ingestiondomain.JobID) error {
	return errors.New("uploads are disabled: the library is not writable")
}

func (unavailableStore) IDs(context.Context) ([]ingestiondomain.JobID, error) { return nil, nil }

// syncConsoles refreshes console logos and release years from IGDB. It runs in
// the background at startup; failures only log, the app keeps the seeded data.
func (a *app) syncConsoles(ctx context.Context, log *slog.Logger) {
	if !a.metadata.Configured() {
		log.Info("IGDB not configured: console logos will not be refreshed")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	n, err := a.consoles.SyncPlatformMetadata(ctx, platformDirectory{a.metadata})
	if err != nil {
		log.Warn("console metadata sync failed", "error", err)
		return
	}
	log.Info("console metadata synced from IGDB", "updated", n)
}

// platformDirectory adapts the metadata context to the catalog's port.
type platformDirectory struct{ svc *metadataapp.Service }

func (d platformDirectory) PlatformsByID(ctx context.Context, ids []int64) ([]catalogapp.PlatformInfo, error) {
	platforms, err := d.svc.PlatformsByID(ctx, ids)
	switch {
	case errors.Is(err, metadatadomain.ErrNotConfigured):
		return nil, catalogapp.ErrMetadataNotConfigured
	case errors.Is(err, metadatadomain.ErrUpstream):
		return nil, fmt.Errorf("%w: %w", catalogapp.ErrMetadataUnavailable, err)
	case err != nil:
		return nil, err
	}
	out := make([]catalogapp.PlatformInfo, 0, len(platforms))
	for _, p := range platforms {
		out = append(out, catalogapp.PlatformInfo{IGDBPlatformID: p.ID, LogoImageID: p.LogoImageID, ReleaseYear: p.ReleaseYear})
	}
	return out, nil
}

func passwordVerifier(cfg config.Config) (identityapp.PasswordVerifier, error) {
	if cfg.PasswordHash != "" {
		v, err := identityinfra.NewArgon2idVerifier(cfg.PasswordHash)
		if err != nil {
			return nil, fmt.Errorf("APP_PASSWORD_HASH: %w", err)
		}
		return v, nil
	}
	return identityinfra.NewArgon2idVerifierFromPlain(cfg.Password)
}
