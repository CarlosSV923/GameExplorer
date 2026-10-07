// Package application holds the ingestion use cases and their ports.
package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
)

// UploadStore is where uploaded bytes live (the tus store).
type UploadStore interface {
	// Delete removes an upload's data; deleting a missing upload is not an error.
	Delete(ctx context.Context, id domain.JobID) error
	// IDs lists every upload present in the store.
	IDs(ctx context.Context) ([]domain.JobID, error)
}

// Publisher broadcasts job changes (server-sent events).
type Publisher interface {
	Publish(job domain.UploadJob)
}

// AbandonedAfter is how long an upload may stay idle before it is purged (RF-12).
const AbandonedAfter = 24 * time.Hour

// ErrCannotCancel means the job is already finished or committing.
var ErrCannotCancel = errors.New("job can no longer be cancelled")

// Service implements the ingestion use cases.
type Service struct {
	repo  domain.JobRepository
	store UploadStore
	pub   Publisher
	log   *slog.Logger
	now   func() time.Time
}

// NewService builds the service. now may be nil (time.Now).
func NewService(repo domain.JobRepository, store UploadStore, pub Publisher, log *slog.Logger, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, store: store, pub: pub, log: log, now: now}
}

var slugShape = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// UploadCreated registers a job for an upload the browser just started.
// origin is the console slug of the screen the upload started from (RF-08).
func (s *Service) UploadCreated(ctx context.Context, id domain.JobID, fileName string, size int64, origin string) error {
	var originPtr *string
	if slugShape.MatchString(origin) {
		originPtr = &origin
	}
	job, err := domain.NewUploadJob(id, fileName, size, originPtr, s.now())
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, job); err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	s.pub.Publish(*job)
	return nil
}

// UploadProgress records received bytes.
func (s *Service) UploadProgress(ctx context.Context, id domain.JobID, received int64) error {
	return s.update(ctx, id, func(j *domain.UploadJob) error {
		j.RecordProgress(received, s.now())
		return nil
	})
}

// UploadFinished records that the upload is complete.
func (s *Service) UploadFinished(ctx context.Context, id domain.JobID, storagePath string) error {
	return s.update(ctx, id, func(j *domain.UploadJob) error {
		return j.MarkUploaded(storagePath, s.now())
	})
}

// UploadTerminated handles the browser deleting its upload (tus termination).
func (s *Service) UploadTerminated(ctx context.Context, id domain.JobID) error {
	return s.update(ctx, id, func(j *domain.UploadJob) error {
		if j.Status.Terminal() {
			return nil
		}
		return j.Cancel(s.now())
	})
}

// Cancel stops a job and deletes its uploaded data.
func (s *Service) Cancel(ctx context.Context, id domain.JobID) (*domain.UploadJob, error) {
	job, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !job.Status.CanTransitionTo(domain.StatusCancelled) {
		return nil, ErrCannotCancel
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("delete upload data: %w", err)
	}
	if err := job.Cancel(s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, job); err != nil {
		return nil, err
	}
	s.pub.Publish(*job)
	return job, nil
}

// Get returns one job.
func (s *Service) Get(ctx context.Context, id domain.JobID) (*domain.UploadJob, error) {
	return s.repo.Get(ctx, id)
}

// List returns recent jobs, newest first.
func (s *Service) List(ctx context.Context) ([]*domain.UploadJob, error) {
	return s.repo.List(ctx, 100)
}

// PurgeResult summarizes a purge run.
type PurgeResult struct {
	Abandoned int
	Orphans   int
}

// Purge fails uploads idle for longer than AbandonedAfter and deletes stored
// uploads that no job knows about (RF-12).
func (s *Service) Purge(ctx context.Context) (PurgeResult, error) {
	var res PurgeResult
	stale, err := s.repo.ListStale(ctx, domain.StatusUploading, s.now().Add(-AbandonedAfter))
	if err != nil {
		return res, err
	}
	for _, job := range stale {
		if err := s.store.Delete(ctx, job.ID); err != nil {
			return res, err
		}
		if err := job.Fail("Subida abandonada: sin actividad durante 24 h.", s.now()); err != nil {
			return res, err
		}
		if err := s.repo.Save(ctx, job); err != nil {
			return res, err
		}
		s.pub.Publish(*job)
		res.Abandoned++
	}

	ids, err := s.store.IDs(ctx)
	if err != nil {
		return res, err
	}
	for _, id := range ids {
		exists, err := s.repo.Exists(ctx, id)
		if err != nil {
			return res, err
		}
		if !exists {
			if err := s.store.Delete(ctx, id); err != nil {
				return res, err
			}
			res.Orphans++
		}
	}
	return res, nil
}

// RunPurge purges at start and then every interval until ctx ends.
func (s *Service) RunPurge(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		res, err := s.Purge(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			s.log.Warn("upload purge failed", "error", err)
		case res.Abandoned > 0 || res.Orphans > 0:
			s.log.Info("upload purge", "abandoned", res.Abandoned, "orphans", res.Orphans)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Service) update(ctx context.Context, id domain.JobID, fn func(*domain.UploadJob) error) error {
	job, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	before := *job
	if err := fn(job); err != nil {
		return err
	}
	if job.UpdatedAt.Equal(before.UpdatedAt) && job.Status == before.Status {
		return nil // nothing changed
	}
	if err := s.repo.Save(ctx, job); err != nil {
		return err
	}
	s.pub.Publish(*job)
	return nil
}
