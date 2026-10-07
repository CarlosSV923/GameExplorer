// Package config loads and validates the process configuration from
// environment variables. Nothing else in the codebase reads os.Getenv.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/caarlos0/env/v11"
)

// MinSessionSecretBytes is the minimum length of SESSION_SECRET.
const MinSessionSecretBytes = 32

// Config is the validated process configuration.
type Config struct {
	Port int `env:"PORT" envDefault:"8080"`

	// LibraryPath is the SMB dataset that holds the game library.
	LibraryPath string `env:"LIBRARY_PATH,required"`
	// DataPath holds the SQLite database and the image cache.
	DataPath string `env:"DATA_PATH,required"`

	// PasswordHash is the argon2id PHC string of the shared password.
	PasswordHash string `env:"APP_PASSWORD_HASH"`
	// Password is a plain-text alternative to PasswordHash, hashed in memory at
	// startup. Convenient on TrueNAS, where generating a hash needs a shell.
	Password string `env:"APP_PASSWORD"`

	SessionSecret string        `env:"SESSION_SECRET,required"`
	SessionTTL    time.Duration `env:"SESSION_TTL" envDefault:"720h"`
	// CookieSecure must be true only when the app is served over HTTPS.
	CookieSecure bool `env:"COOKIE_SECURE" envDefault:"false"`

	// IGDB credentials (Twitch developer app). Without them the app works, but
	// game and platform search answer 503.
	IGDBClientID     string `env:"IGDB_CLIENT_ID"`
	IGDBClientSecret string `env:"IGDB_CLIENT_SECRET"`
	// Endpoint overrides, only for tests; empty means the real IGDB services.
	IGDBAPIURL   string `env:"IGDB_API_URL"`
	IGDBTokenURL string `env:"IGDB_TOKEN_URL"`
	IGDBImageURL string `env:"IGDB_IMAGE_URL"`

	// ExtractConcurrency is how many archives are extracted at once. One is
	// right for spinning disks: parallel extractions only make them seek.
	ExtractConcurrency int `env:"EXTRACT_CONCURRENCY" envDefault:"1"`

	// TrashRetentionDays is how long trashed items are kept before they are
	// deleted for good (RF-30).
	TrashRetentionDays int `env:"TRASH_RETENTION_DAYS" envDefault:"30"`

	LogLevel slog.Level `env:"LOG_LEVEL" envDefault:"info"`
}

// Load reads the configuration from the given environment (nil means the
// process environment) and validates it.
func Load(environ map[string]string) (Config, error) {
	var cfg Config
	opts := env.Options{}
	if environ != nil {
		opts.Environment = environ
	}
	if err := env.ParseWithOptions(&cfg, opts); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return Config{}, fmt.Errorf("config: %w", err)
	}
	return cfg, nil
}

// IGDBConfigured reports whether IGDB credentials are present.
func (c Config) IGDBConfigured() bool { return c.IGDBClientID != "" && c.IGDBClientSecret != "" }

func (c Config) validate() error {
	var errs []error
	switch {
	case c.PasswordHash == "" && c.Password == "":
		errs = append(errs, errors.New("set APP_PASSWORD_HASH (recommended) or APP_PASSWORD"))
	case c.PasswordHash != "" && c.Password != "":
		errs = append(errs, errors.New("set only one of APP_PASSWORD_HASH and APP_PASSWORD"))
	}
	if len(c.SessionSecret) < MinSessionSecretBytes {
		errs = append(errs, fmt.Errorf("SESSION_SECRET must be at least %d bytes", MinSessionSecretBytes))
	}
	if (c.IGDBClientID == "") != (c.IGDBClientSecret == "") {
		errs = append(errs, errors.New("set both IGDB_CLIENT_ID and IGDB_CLIENT_SECRET, or neither"))
	}
	if c.SessionTTL <= 0 {
		errs = append(errs, errors.New("SESSION_TTL must be positive"))
	}
	if c.ExtractConcurrency < 1 || c.ExtractConcurrency > 8 {
		errs = append(errs, fmt.Errorf("EXTRACT_CONCURRENCY must be between 1 and 8, got %d", c.ExtractConcurrency))
	}
	if c.TrashRetentionDays < 1 || c.TrashRetentionDays > 3650 {
		errs = append(errs, fmt.Errorf("TRASH_RETENTION_DAYS must be between 1 and 3650, got %d", c.TrashRetentionDays))
	}
	if c.Port <= 0 || c.Port > 65535 {
		errs = append(errs, fmt.Errorf("PORT %d is out of range", c.Port))
	}
	return errors.Join(errs...)
}
