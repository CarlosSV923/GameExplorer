package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	catalogapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	catalogsqlite "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite"
	cataloghttp "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/interfaces/http"
	identityapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/application"
	identityinfra "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/infrastructure"
	identityhttp "github.com/CarlosSV923/GameExplorer/apps/api/internal/identity/interfaces/http"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
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
)

// api assembles the bounded contexts' handlers into the generated interface.
// Each embedded handler implements a disjoint subset of the operations.
type api struct {
	*systemAPI
	*identityAPI
	*catalogAPI
}

var _ httpapi.StrictServerInterface = api{}

// app is the fully wired application.
type app struct {
	handler http.Handler
	db      *sql.DB
}

func (a *app) Close() error { return a.db.Close() }

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

	consoles := catalogapp.NewConsoleService(catalogsqlite.NewConsoleRepository(db))

	server := api{
		systemAPI: system.NewHandler(
			system.WritableDir("library-writable", cfg.LibraryPath),
			system.WritableDir("data-writable", cfg.DataPath),
		),
		identityAPI: identityHandler,
		catalogAPI:  cataloghttp.NewHandler(consoles),
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
	return &app{handler: handler, db: db}, nil
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
