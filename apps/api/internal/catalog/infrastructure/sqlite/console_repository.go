// Package sqlite implements the catalog repositories on SQLite via sqlc.
package sqlite

import (
	"context"
	"database/sql"

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

// Settings implements domain.ConsoleRepository.
func (r *ConsoleRepository) Settings(ctx context.Context) (map[domain.Slug]domain.ConsoleSettings, error) {
	rows, err := r.q.ListConsoleSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.Slug]domain.ConsoleSettings, len(rows))
	for _, row := range rows {
		out[domain.Slug(row.Slug)] = domain.ConsoleSettings{
			DisplayName: stringPtr(row.DisplayName), SortOrder: int(row.SortOrder),
			LogoImageID: stringPtr(row.LogoImageID), ReleaseYear: intPtr(row.ReleaseYear),
		}
	}
	return out, nil
}

// SetDisplayName implements domain.ConsoleRepository.
func (r *ConsoleRepository) SetDisplayName(ctx context.Context, slug domain.Slug, name *string) error {
	return r.q.SetConsoleDisplayName(ctx, sqlcgen.SetConsoleDisplayNameParams{DisplayName: nullString(name), Slug: string(slug)})
}

// SetOrder implements domain.ConsoleRepository.
func (r *ConsoleRepository) SetOrder(ctx context.Context, slugs []domain.Slug) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	for i, slug := range slugs {
		if err := q.UpsertConsoleOrder(ctx, sqlcgen.UpsertConsoleOrderParams{Slug: string(slug), SortOrder: int64((i + 1) * 10)}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SetPlatformMetadata implements domain.ConsoleRepository.
func (r *ConsoleRepository) SetPlatformMetadata(ctx context.Context, slug domain.Slug, logoImageID *string, releaseYear *int) error {
	return r.q.SetConsolePlatformMetadata(ctx, sqlcgen.SetConsolePlatformMetadataParams{
		LogoImageID: nullString(logoImageID), ReleaseYear: nullInt(releaseYear), Slug: string(slug),
	})
}

// CustomExtensions implements domain.ConsoleRepository.
func (r *ConsoleRepository) CustomExtensions(ctx context.Context) (map[domain.Slug][]string, error) {
	rows, err := r.q.ListConsoleExtensions(ctx)
	if err != nil {
		return nil, err
	}
	out := map[domain.Slug][]string{}
	for _, row := range rows {
		out[domain.Slug(row.Slug)] = append(out[domain.Slug(row.Slug)], row.Extension)
	}
	return out, nil
}

// AddExtension implements domain.ConsoleRepository.
func (r *ConsoleRepository) AddExtension(ctx context.Context, slug domain.Slug, ext string) error {
	return r.q.AddConsoleExtension(ctx, sqlcgen.AddConsoleExtensionParams{Slug: string(slug), Extension: ext})
}

// RemoveExtension implements domain.ConsoleRepository.
func (r *ConsoleRepository) RemoveExtension(ctx context.Context, slug domain.Slug, ext string) error {
	return r.q.RemoveConsoleExtension(ctx, sqlcgen.RemoveConsoleExtensionParams{Slug: string(slug), Extension: ext})
}

// GameCounts implements domain.ConsoleRepository.
func (r *ConsoleRepository) GameCounts(ctx context.Context) (map[domain.Slug]int, error) {
	rows, err := r.q.CountGamesByConsole(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.Slug]int, len(rows))
	for _, row := range rows {
		out[domain.Slug(row.Console)] = int(row.Games)
	}
	return out, nil
}

// ItemFiles implements domain.ConsoleRepository.
func (r *ConsoleRepository) ItemFiles(ctx context.Context, slug domain.Slug) ([]string, error) {
	return r.q.ListConsoleItemFiles(ctx, string(slug))
}
