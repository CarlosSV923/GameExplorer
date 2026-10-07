// Package sqlite implements the catalog repositories on SQLite via sqlc.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite/sqlcgen"
)

// ConsoleRepository implements domain.ConsoleRepository.
type ConsoleRepository struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

var _ domain.ConsoleRepository = (*ConsoleRepository)(nil)

// NewConsoleRepository builds the repository.
func NewConsoleRepository(db *sql.DB) *ConsoleRepository {
	return &ConsoleRepository{db: db, q: sqlcgen.New(db)}
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

// Create implements domain.ConsoleRepository.
func (r *ConsoleRepository) Create(ctx context.Context, c domain.Console) (domain.ConsoleID, error) {
	exts, err := json.Marshal(c.Extensions)
	if err != nil {
		return 0, err
	}
	params := sqlcgen.InsertConsoleParams{
		Slug: string(c.Slug), DisplayName: c.DisplayName, Extensions: string(exts),
		ReleaseYear: nullInt(c.ReleaseYear), LogoImageID: nullString(c.LogoImageID),
	}
	if c.IGDBPlatformID != nil {
		params.IgdbPlatformID = sql.NullInt64{Int64: *c.IGDBPlatformID, Valid: true}
	}
	id, err := r.q.InsertConsole(ctx, params)
	if isUniqueViolation(err) {
		return 0, domain.ErrSlugTaken
	}
	return domain.ConsoleID(id), err
}

// Update implements domain.ConsoleRepository.
func (r *ConsoleRepository) Update(ctx context.Context, c domain.Console) error {
	exts, err := json.Marshal(c.Extensions)
	if err != nil {
		return err
	}
	err = r.q.UpdateConsole(ctx, sqlcgen.UpdateConsoleParams{
		Slug: string(c.Slug), DisplayName: c.DisplayName, Extensions: string(exts), ID: int64(c.ID),
	})
	if isUniqueViolation(err) {
		return domain.ErrSlugTaken
	}
	return err
}

// Delete implements domain.ConsoleRepository.
func (r *ConsoleRepository) Delete(ctx context.Context, id domain.ConsoleID) error {
	return r.q.DeleteConsole(ctx, int64(id))
}

// SetOrder implements domain.ConsoleRepository.
func (r *ConsoleRepository) SetOrder(ctx context.Context, ids []domain.ConsoleID) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	for i, id := range ids {
		if err := q.SetConsoleOrder(ctx, sqlcgen.SetConsoleOrderParams{SortOrder: int64(i + 1), ID: int64(id)}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HasGames implements domain.ConsoleRepository.
func (r *ConsoleRepository) HasGames(ctx context.Context, id domain.ConsoleID) (bool, error) {
	return r.q.ConsoleHasGames(ctx, int64(id))
}

// isUniqueViolation recognizes SQLite's UNIQUE constraint error.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "UNIQUE constraint failed")
}
