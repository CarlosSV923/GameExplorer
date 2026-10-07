package application_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sqlite"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
)

type fakeStore struct {
	mu      sync.Mutex
	ids     []domain.JobID
	deleted []domain.JobID
}

func (f *fakeStore) Delete(_ context.Context, id domain.JobID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeStore) IDs(context.Context) ([]domain.JobID, error) { return f.ids, nil }

type recorder struct {
	mu   sync.Mutex
	jobs []domain.UploadJob
}

func (r *recorder) Publish(j domain.UploadJob) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, j)
}

func (r *recorder) statuses() []domain.Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Status, len(r.jobs))
	for i, j := range r.jobs {
		out[i] = j.Status
	}
	return out
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newService(t *testing.T, store *fakeStore) (*application.Service, *recorder, *clock) {
	t.Helper()
	conn, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	rec := &recorder{}
	clk := &clock{t: time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)}
	svc := application.NewService(sqlite.NewJobRepository(conn), store, rec,
		slog.New(slog.NewTextHandler(io.Discard, nil)), clk.now)
	return svc, rec, clk
}

func TestUploadEventsDriveTheJob(t *testing.T) {
	t.Parallel()
	svc, rec, clk := newService(t, &fakeStore{})
	ctx := t.Context()

	if err := svc.UploadCreated(ctx, "u1", "INSIDE [0100D2D009028000][v0].nsp", 1000, "switch"); err != nil {
		t.Fatal(err)
	}
	clk.t = clk.t.Add(time.Second)
	_ = svc.UploadProgress(ctx, "u1", 400)
	clk.t = clk.t.Add(time.Second)
	if err := svc.UploadFinished(ctx, "u1", "/lib/.gameexplorer/staging/uploads/u1"); err != nil {
		t.Fatal(err)
	}

	job, err := svc.Get(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != domain.StatusUploaded || job.Received != 1000 || job.OriginConsole == nil || *job.OriginConsole != "switch" ||
		job.StoragePath != "/lib/.gameexplorer/staging/uploads/u1" {
		t.Fatalf("job = %+v", job)
	}
	want := []domain.Status{domain.StatusUploading, domain.StatusUploading, domain.StatusUploaded}
	if got := rec.statuses(); !slices.Equal(got, want) {
		t.Fatalf("published %v, want %v", got, want)
	}
}

func TestUnknownOriginIsIgnored(t *testing.T) {
	t.Parallel()
	svc, _, _ := newService(t, &fakeStore{})

	_ = svc.UploadCreated(t.Context(), "u1", "a.rar", 1, "../../etc")
	job, _ := svc.Get(t.Context(), "u1")
	if job.OriginConsole != nil {
		t.Fatalf("origin = %q, want nil for an invalid slug", *job.OriginConsole)
	}
}

func TestCancelDeletesDataAndIsFinal(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	svc, _, _ := newService(t, store)
	ctx := t.Context()
	_ = svc.UploadCreated(ctx, "u1", "a.rar", 10, "")

	job, err := svc.Cancel(ctx, "u1")
	if err != nil || job.Status != domain.StatusCancelled || !slices.Equal(store.deleted, []domain.JobID{"u1"}) {
		t.Fatalf("cancel = %+v, %v, deleted %v", job, err, store.deleted)
	}
	if _, err := svc.Cancel(ctx, "u1"); !errors.Is(err, application.ErrCannotCancel) {
		t.Fatalf("second cancel err = %v", err)
	}
	if _, err := svc.Cancel(ctx, "nope"); !errors.Is(err, domain.ErrJobNotFound) {
		t.Fatalf("missing job err = %v", err)
	}
}

func TestTerminationFromTheBrowserCancels(t *testing.T) {
	t.Parallel()
	svc, _, _ := newService(t, &fakeStore{})
	ctx := t.Context()
	_ = svc.UploadCreated(ctx, "u1", "a.rar", 10, "")

	if err := svc.UploadTerminated(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	job, _ := svc.Get(ctx, "u1")
	if job.Status != domain.StatusCancelled {
		t.Fatalf("status = %s", job.Status)
	}
}

func TestPurgeAbandonedAndOrphans(t *testing.T) {
	t.Parallel()
	store := &fakeStore{ids: []domain.JobID{"old", "fresh", "done", "orphan"}}
	svc, _, clk := newService(t, store)
	ctx := t.Context()

	_ = svc.UploadCreated(ctx, "old", "old.rar", 10, "")
	_ = svc.UploadCreated(ctx, "done", "done.rar", 10, "")
	_ = svc.UploadFinished(ctx, "done", "/x")
	clk.t = clk.t.Add(application.AbandonedAfter - time.Hour)
	_ = svc.UploadCreated(ctx, "fresh", "fresh.rar", 10, "")
	clk.t = clk.t.Add(2 * time.Hour) // "old" is now idle for 25 h, "fresh" for 2 h

	res, err := svc.Purge(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if res.Abandoned != 1 || res.Orphans != 1 {
		t.Fatalf("purge = %+v, want 1 abandoned and 1 orphan", res)
	}
	slices.Sort(store.deleted)
	if !slices.Equal(store.deleted, []domain.JobID{"old", "orphan"}) {
		t.Fatalf("deleted = %v", store.deleted)
	}
	old, _ := svc.Get(ctx, "old")
	if old.Status != domain.StatusFailed || old.Error == "" {
		t.Fatalf("old = %+v", old)
	}
	if fresh, _ := svc.Get(ctx, "fresh"); fresh.Status != domain.StatusUploading {
		t.Fatalf("fresh = %+v", fresh)
	}
}
