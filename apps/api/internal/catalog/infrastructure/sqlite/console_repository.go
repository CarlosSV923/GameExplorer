// Package sqlite implements the catalog repositories on SQLite via sqlc.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite/sqlcgen"
)

// ConsoleRepository implements domain.ConsoleRepository.
type ConsoleRepository struct {
	q *sqlcgen.Queries
}

var _ domain.ConsoleRepository = (*ConsoleRepository)(nil)

// NewConsoleRepository builds the repository.
func NewConsoleRepository(db *sql.DB) *ConsoleRepository {
	return &ConsoleRepository{q: sqlcgen.New(db)}
}

// List implements domain.ConsoleRepository.
func (r *ConsoleRepository) List(ctx context.Context) ([]domain.Console, error) {
	rows, err := r.q.ListConsoles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Console, 0, len(rows))
	for _, row := range rows {
		c, err := toDomain(row)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

func toDomain(row sqlcgen.ListConsolesRow) (domain.Console, error) {
	var exts []string
	if err := json.Unmarshal([]byte(row.Extensions), &exts); err != nil {
		return domain.Console{}, fmt.Errorf("console %q: decode extensions: %w", row.Slug, err)
	}
	c := domain.Console{
		ID:          domain.ConsoleID(row.ID),
		Slug:        domain.Slug(row.Slug),
		DisplayName: row.DisplayName,
		Extensions:  exts,
		SortOrder:   int(row.SortOrder),
		GameCount:   int(row.GameCount),
	}
	if row.IgdbPlatformID.Valid {
		id := row.IgdbPlatformID.Int64
		c.IGDBPlatformID = &id
	}
	if row.ReleaseYear.Valid {
		y := int(row.ReleaseYear.Int64)
		c.ReleaseYear = &y
	}
	if row.DetectorKey.Valid {
		c.DetectorKey = row.DetectorKey.String
	}
	if row.LogoImageID.Valid {
		logo := row.LogoImageID.String
		c.LogoImageID = &logo
	}
	return c, nil
}

// UpdatePlatformMetadata implements domain.ConsoleRepository.
func (r *ConsoleRepository) UpdatePlatformMetadata(ctx context.Context, id domain.ConsoleID, logoImageID *string, releaseYear *int) error {
	params := sqlcgen.UpdateConsolePlatformMetadataParams{ID: int64(id)}
	if logoImageID != nil {
		params.LogoImageID = sql.NullString{String: *logoImageID, Valid: true}
	}
	if releaseYear != nil {
		params.ReleaseYear = sql.NullInt64{Int64: int64(*releaseYear), Valid: true}
	}
	return r.q.UpdateConsolePlatformMetadata(ctx, params)
}
