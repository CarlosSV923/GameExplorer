package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sqlite/sqlcgen"
)

// FileRepository implements domain.StagedFileRepository.
type FileRepository struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

var _ domain.StagedFileRepository = (*FileRepository)(nil)

// NewFileRepository builds the repository.
func NewFileRepository(db *sql.DB) *FileRepository {
	return &FileRepository{db: db, q: sqlcgen.New(db)}
}

// Replace implements domain.StagedFileRepository.
func (r *FileRepository) Replace(ctx context.Context, id domain.JobID, files []domain.StagedFile) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	q := r.q.WithTx(tx)
	if err := q.DeleteFiles(ctx, string(id)); err != nil {
		return err
	}
	for _, f := range files {
		var un sql.NullInt64
		if f.Unassigned != nil {
			un = sql.NullInt64{Int64: *f.Unassigned, Valid: true}
		}
		if err := q.InsertFile(ctx, sqlcgen.InsertFileParams{JobID: string(id), Path: f.Path, Size: f.Size, UnassignedID: un}); err != nil {
			return fmt.Errorf("insert %q: %w", f.Path, err)
		}
	}
	return tx.Commit()
}

// List implements domain.StagedFileRepository.
func (r *FileRepository) List(ctx context.Context, id domain.JobID) ([]domain.StagedFile, error) {
	rows, err := r.q.ListFiles(ctx, string(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.StagedFile, 0, len(rows))
	for _, row := range rows {
		f := domain.StagedFile{JobID: id, Path: row.Path, Size: row.Size}
		if row.UnassignedID.Valid {
			un := row.UnassignedID.Int64
			f.Unassigned = &un
		}
		out = append(out, f)
	}
	return out, nil
}
