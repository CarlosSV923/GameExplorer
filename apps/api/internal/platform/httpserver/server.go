// Package httpserver builds the HTTP server shared by every bounded context.
package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

// New returns an HTTP server tuned for long-lived transfers.
//
// ReadTimeout and WriteTimeout stay at zero on purpose: uploads (tus chunks)
// and downloads of multi-gigabyte files can legitimately take a long time.
// Slow-client protection comes from ReadHeaderTimeout and IdleTimeout instead.
func New(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

type healthResponse struct {
	Status string `json:"status"`
}

// HealthHandler reports that the process is up.
func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{Status: "ok"})
	})
}
