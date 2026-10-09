package domain

import (
	"context"
	"time"
)

// JobRepository persists upload jobs.
type JobRepository interface {
	Create(ctx context.Context, job *UploadJob) error
	// Get returns ErrJobNotFound when the job does not exist.
	Get(ctx context.Context, id JobID) (*UploadJob, error)
	Save(ctx context.Context, job *UploadJob) error
	// List returns the most recent jobs, newest first.
	List(ctx context.Context, limit int) ([]*UploadJob, error)
	// ListStale returns jobs in status whose last update is older than before.
	ListStale(ctx context.Context, status Status, before time.Time) ([]*UploadJob, error)
	// Exists reports whether a job with this id exists.
	Exists(ctx context.Context, id JobID) (bool, error)
	// ListGroup returns the jobs of a multi-volume group, oldest first.
	ListGroup(ctx context.Context, group string) ([]*UploadJob, error)
}
