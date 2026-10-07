// Package sqlite implements the ingestion repositories on SQLite via sqlc.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sqlite/sqlcgen"
)

// timeLayout is fixed-width UTC so that SQL string comparison equals time order.
const timeLayout = "2006-01-02T15:04:05.000000000Z"

// JobRepository implements domain.JobRepository.
type JobRepository struct {
	q *sqlcgen.Queries
}

var _ domain.JobRepository = (*JobRepository)(nil)

// NewJobRepository builds the repository.
func NewJobRepository(db *sql.DB) *JobRepository {
	return &JobRepository{q: sqlcgen.New(db)}
}

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// Create implements domain.JobRepository.
func (r *JobRepository) Create(ctx context.Context, j *domain.UploadJob) error {
	var origin sql.NullString
	if j.OriginConsole != nil {
		origin = sql.NullString{String: *j.OriginConsole, Valid: true}
	}
	return r.q.CreateJob(ctx, sqlcgen.CreateJobParams{
		ID: string(j.ID), FileName: j.FileName, Size: j.Size, Received: j.Received,
		Status: string(j.Status), Error: j.Error, OriginConsole: origin, StoragePath: j.StoragePath,
		CreatedAt: formatTime(j.CreatedAt), UpdatedAt: formatTime(j.UpdatedAt),
	})
}

// Get implements domain.JobRepository.
func (r *JobRepository) Get(ctx context.Context, id domain.JobID) (*domain.UploadJob, error) {
	row, err := r.q.GetJob(ctx, string(id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrJobNotFound
	}
	if err != nil {
		return nil, err
	}
	return toDomain(row)
}

// Save implements domain.JobRepository.
func (r *JobRepository) Save(ctx context.Context, j *domain.UploadJob) error {
	return r.q.SaveJob(ctx, sqlcgen.SaveJobParams{
		ID: string(j.ID), Received: j.Received, Status: string(j.Status), Error: j.Error,
		StoragePath: j.StoragePath, UpdatedAt: formatTime(j.UpdatedAt),
	})
}

// List implements domain.JobRepository.
func (r *JobRepository) List(ctx context.Context, limit int) ([]*domain.UploadJob, error) {
	rows, err := r.q.ListJobs(ctx, int64(limit))
	if err != nil {
		return nil, err
	}
	return toDomainAll(rows)
}

// ListStale implements domain.JobRepository.
func (r *JobRepository) ListStale(ctx context.Context, status domain.Status, before time.Time) ([]*domain.UploadJob, error) {
	rows, err := r.q.ListStaleJobs(ctx, sqlcgen.ListStaleJobsParams{Status: string(status), UpdatedAt: formatTime(before)})
	if err != nil {
		return nil, err
	}
	return toDomainAll(rows)
}

// Exists implements domain.JobRepository.
func (r *JobRepository) Exists(ctx context.Context, id domain.JobID) (bool, error) {
	return r.q.JobExists(ctx, string(id))
}

func toDomainAll(rows []sqlcgen.UploadJob) ([]*domain.UploadJob, error) {
	out := make([]*domain.UploadJob, 0, len(rows))
	for _, row := range rows {
		j, err := toDomain(row)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, nil
}

func toDomain(row sqlcgen.UploadJob) (*domain.UploadJob, error) {
	created, err1 := time.Parse(timeLayout, row.CreatedAt)
	updated, err2 := time.Parse(timeLayout, row.UpdatedAt)
	if err := errors.Join(err1, err2); err != nil {
		return nil, fmt.Errorf("job %s: %w", row.ID, err)
	}
	j := &domain.UploadJob{
		ID: domain.JobID(row.ID), FileName: row.FileName, Size: row.Size, Received: row.Received,
		Status: domain.Status(row.Status), Error: row.Error, StoragePath: row.StoragePath,
		CreatedAt: created, UpdatedAt: updated,
	}
	if row.OriginConsole.Valid {
		o := row.OriginConsole.String
		j.OriginConsole = &o
	}
	return j, nil
}
