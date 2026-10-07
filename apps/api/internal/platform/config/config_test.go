package config_test

import (
	"log/slog"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

func validEnv() map[string]string {
	return map[string]string{
		"LIBRARY_PATH":   "/library",
		"DATA_PATH":      "/data",
		"APP_PASSWORD":   "secret",
		"SESSION_SECRET": strings.Repeat("x", config.MinSessionSecretBytes),
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(validEnv())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
	if cfg.SessionTTL != 720*time.Hour {
		t.Errorf("SessionTTL = %v, want 720h", cfg.SessionTTL)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure should default to false (LAN over HTTP)")
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
}

func TestLoadRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(map[string]string)
		wantErr string
	}{
		{"missing library", func(e map[string]string) { delete(e, "LIBRARY_PATH") }, "LIBRARY_PATH"},
		{"missing data", func(e map[string]string) { delete(e, "DATA_PATH") }, "DATA_PATH"},
		{"no password", func(e map[string]string) { delete(e, "APP_PASSWORD") }, "APP_PASSWORD_HASH (recommended) or APP_PASSWORD"},
		{"both passwords", func(e map[string]string) { e["APP_PASSWORD_HASH"] = "$argon2id$..." }, "only one"},
		{"short secret", func(e map[string]string) { e["SESSION_SECRET"] = "short" }, "SESSION_SECRET must be at least"},
		{"zero ttl", func(e map[string]string) { e["SESSION_TTL"] = "0s" }, "SESSION_TTL must be positive"},
		{"bad port", func(e map[string]string) { e["PORT"] = "70000" }, "out of range"},
		{"half igdb credentials", func(e map[string]string) { e["IGDB_CLIENT_ID"] = "abc" }, "IGDB_CLIENT_SECRET"},
		{"zero extract concurrency", func(e map[string]string) { e["EXTRACT_CONCURRENCY"] = "0" }, "EXTRACT_CONCURRENCY"},
		{"bad log level", func(e map[string]string) { e["LOG_LEVEL"] = "loud" }, `"loud"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			env := maps.Clone(validEnv())
			tt.mutate(env)

			_, err := config.Load(env)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}
