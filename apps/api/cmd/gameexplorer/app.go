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
	catalogsqlite "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite"
	cataloghttp "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/interfaces/http"
	identityapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/application"
	identityinfra "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/infrastructure"
	identityhttp "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/interfaces/http"
	ingestionapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	ingestiondomain "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/detection"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/broker"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sevenzip"
	ingestionsqlite "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sqlite"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/staging"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/tus"
	ingestionhttp "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/interfaces/http"
	metadataapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/application"
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
// abandoned uploads and the console metadata sync.
func (a *app) start(ctx context.Context) {
	if a.uploads != nil {
		go a.uploads.Run(ctx, a.ingestion)
		go a.ingestion.RunPurge(ctx, time.Hour)
		go a.processor.Run(ctx, a.cfg.ExtractConcurrency)
	}
	go a.syncConsoles(ctx, a.log)
}

// stagingDir holds each job's extracted files until they are committed.
func stagingDir(libraryPath string) string {
	return filepath.Join(libraryPath, ".gameexplorer", "staging", "extracted")
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

// newApp wires every adapter. It is the only place that knows them all.
func newApp(ctx context.Context, cfg config.Config, log *slog.Logger) (*app, error) {
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

	consoles := catalogapp.NewConsoleService(catalogsqlite.NewConsoleRepository(db))

	events := broker.New()
	uploads, uploadsErr := tus.New(uploadsDir(cfg.LibraryPath), "/api/uploads/", log, diskspace.Free)
	var store ingestionapp.UploadStore = unavailableStore{}
	if uploadsErr != nil {
		log.Error("uploads disabled: library not writable", "error", uploadsErr)
	} else {
		store = uploads
	}
	jobs := ingestionsqlite.NewJobRepository(db)
	items := ingestionsqlite.NewItemRepository(db)
	extractor := sevenzip.New("")
	ingestion := ingestionapp.NewService(jobs, store, events, log, nil).WithItems(items)
	var processor *ingestionapp.Processor
	if uploadsErr == nil {
		processor = ingestionapp.NewProcessor(ingestionapp.ProcessorDeps{
			Jobs: jobs, Items: items, Uploads: uploads, Extractor: extractor,
			Staging: staging.New(stagingDir(cfg.LibraryPath), volumesDir(cfg.LibraryPath)), Profiles: consoleProfiles{consoles},
			Publisher: events, Log: log,
		})
		ingestion.WithQueue(processor)
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
		catalogAPI:  cataloghttp.NewHandler(consoles),
		metadataAPI: metadatahttp.NewHandler(metadata, log),
		jobsAPI:     ingestionhttp.NewHandler(ingestion, passwordSubmitter{processor}, events, shutdown),
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
	)
	a := &app{
		handler: handler, db: db, log: log, consoles: consoles, metadata: metadata,
		ingestion: ingestion, processor: processor, shutdown: shutdown, cfg: cfg,
	}
	if uploadsErr == nil {
		a.uploads = uploads
	}
	return a, nil
}

// consoleProfiles adapts the catalog's consoles to ingestion's detection.
type consoleProfiles struct{ svc *catalogapp.ConsoleService }

func (c consoleProfiles) Profiles(ctx context.Context) ([]detection.ConsoleProfile, error) {
	consoles, err := c.svc.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]detection.ConsoleProfile, 0, len(consoles))
	for _, con := range consoles {
		out = append(out, detection.ConsoleProfile{Slug: string(con.Slug), Extensions: con.Extensions, DetectorKey: con.DetectorKey})
	}
	return out, nil
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
	if err != nil {
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
