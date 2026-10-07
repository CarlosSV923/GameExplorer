// Package system implements process-level operations: health and startup
// diagnostics.
package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpapi"
)

// Check is one named diagnostic.
type Check struct {
	Name string
	Run  func(ctx context.Context) error
}

// WritableDir checks that dir exists and that this process can create files
// in it. On TrueNAS this catches the most common install mistake: the app's
// user has no ACL entry on the dataset (see docs/spike-results.md).
func WritableDir(name, dir string) Check {
	return Check{Name: name, Run: func(context.Context) error {
		f, err := os.CreateTemp(dir, ".gameexplorer-write-check-*")
		if err != nil {
			return fmt.Errorf("no se puede escribir en %s como uid=%d gid=%d: %w. "+
				"Da permiso de modificación a ese usuario en el dataset (ACL)",
				filepath.Clean(dir), os.Getuid(), os.Getgid(), err)
		}
		name := f.Name()
		_ = f.Close()
		return os.Remove(name)
	}}
}

// Handler implements GetHealth.
type Handler struct {
	checks []Check
}

// NewHandler builds the handler.
func NewHandler(checks ...Check) *Handler {
	return &Handler{checks: checks}
}

// GetHealth implements httpapi.StrictServerInterface.
func (h *Handler) GetHealth(ctx context.Context, _ httpapi.GetHealthRequestObject) (httpapi.GetHealthResponseObject, error) {
	res := httpapi.GetHealth200JSONResponse{Status: httpapi.Ok, Checks: make([]httpapi.HealthCheck, 0, len(h.checks))}
	for _, c := range h.checks {
		hc := httpapi.HealthCheck{Name: c.Name, Ok: true}
		if err := c.Run(ctx); err != nil {
			msg := err.Error()
			hc.Ok, hc.Message = false, &msg
			res.Status = httpapi.Degraded
		}
		res.Checks = append(res.Checks, hc)
	}
	return res, nil
}
