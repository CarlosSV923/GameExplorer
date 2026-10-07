package application_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sqlite"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/staging"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
)

// recordedLibrary says which uploads have items in the library.
type recordedLibrary struct {
	application.Library
	stored    map[string]bool
	recovered bool
}

func (l *recordedLibrary) Recover(context.Context) (int, error) {
	l.recovered = true
	return 0, nil
}

func (l *recordedLibrary) HasItemsFrom(_ context.Context, source string) (bool, error) {
	return l.stored[source], nil
}

func TestRecoverFinishesOrReopensInterruptedCommits(t *testing.T) {
	t.Parallel()
	conn, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	jobs := sqlite.NewJobRepository(conn)
	area := staging.New(t.TempDir(), t.TempDir())
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

	for _, id := range []domain.JobID{"recorded", "unrecorded"} {
		job := &domain.UploadJob{ID: id, FileName: "x.nsp", Status: domain.StatusCommitting, CreatedAt: now, UpdatedAt: now}
		if err := jobs.Create(t.Context(), job); err != nil {
			t.Fatal(err)
		}
		if err := area.Reset(id); err != nil {
			t.Fatal(err)
		}
	}
	lib := &recordedLibrary{stored: map[string]bool{"recorded": true}}
	c := application.NewCommitter(jobs, sqlite.NewItemRepository(conn), area, lib, &recorder{},
		slog.New(slog.NewTextHandler(io.Discard, nil)), func() time.Time { return now })

	if err := c.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !lib.recovered {
		t.Fatal("the library must undo its journal first")
	}
	done, _ := jobs.Get(t.Context(), "recorded")
	if done.Status != domain.StatusDone {
		t.Fatalf("recorded job = %s, want done", done.Status)
	}
	if _, err := os.Stat(area.Dir("recorded")); !os.IsNotExist(err) {
		t.Fatalf("staging of a finished commit must be removed: %v", err)
	}
	back, _ := jobs.Get(t.Context(), "unrecorded")
	if back.Status != domain.StatusReview || back.Error == "" {
		t.Fatalf("unrecorded job = %s %q, want review with a reason", back.Status, back.Error)
	}
	if _, err := os.Stat(area.Dir("unrecorded")); err != nil {
		t.Fatalf("staging of an undone commit must stay: %v", err)
	}
}
