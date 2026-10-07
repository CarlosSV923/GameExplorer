package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite/sqlcgen"
)

// timeLayout is fixed-width UTC so that SQL string comparison equals time order.
const timeLayout = "2006-01-02T15:04:05.000000000Z"

func formatTime(t time.Time) string { return t.UTC().Format(timeLayout) }

// LibraryRepository implements domain.LibraryRepository.
type LibraryRepository struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

var _ domain.LibraryRepository = (*LibraryRepository)(nil)

// NewLibraryRepository builds the repository.
func NewLibraryRepository(db *sql.DB) *LibraryRepository {
	return &LibraryRepository{db: db, q: sqlcgen.New(db)}
}

// FindGame implements domain.LibraryRepository.
func (r *LibraryRepository) FindGame(ctx context.Context, console domain.ConsoleID, igdbID int64) (*domain.Game, error) {
	row, err := r.q.FindGameByIGDB(ctx, sqlcgen.FindGameByIGDBParams{ConsoleID: int64(console), IgdbID: igdbID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrGameNotFound
	}
	if err != nil {
		return nil, err
	}
	g, err := gameToDomain(row)
	if err != nil {
		return nil, err
	}
	items, err := r.q.ListGameItems(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		item, err := itemToDomain(it)
		if err != nil {
			return nil, err
		}
		g.Items = append(g.Items, item)
	}
	return g, nil
}

// FolderTaken implements domain.LibraryRepository.
func (r *LibraryRepository) FolderTaken(ctx context.Context, console domain.ConsoleID, folder string) (bool, error) {
	return r.q.FolderTaken(ctx, sqlcgen.FolderTakenParams{ConsoleID: int64(console), Folder: folder})
}

// HasItemsFrom implements domain.LibraryRepository.
func (r *LibraryRepository) HasItemsFrom(ctx context.Context, source string) (bool, error) {
	return r.q.HasItemsFromSource(ctx, source)
}

// SaveOperation implements domain.LibraryRepository.
func (r *LibraryRepository) SaveOperation(ctx context.Context, op domain.Operation) error {
	moves, err := json.Marshal(op.Moves)
	if err != nil {
		return err
	}
	dirs, err := json.Marshal(nonNil(op.CreatedDirs))
	if err != nil {
		return err
	}
	return r.q.InsertOperation(ctx, sqlcgen.InsertOperationParams{
		ID: op.ID, Source: op.Source, Moves: string(moves), CreatedDirs: string(dirs), CreatedAt: formatTime(op.CreatedAt),
	})
}

// Operations implements domain.LibraryRepository.
func (r *LibraryRepository) Operations(ctx context.Context) ([]domain.Operation, error) {
	rows, err := r.q.ListOperations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Operation, 0, len(rows))
	for _, row := range rows {
		op := domain.Operation{ID: row.ID, Source: row.Source}
		if err := json.Unmarshal([]byte(row.Moves), &op.Moves); err != nil {
			return nil, fmt.Errorf("operation %s: decode moves: %w", row.ID, err)
		}
		if err := json.Unmarshal([]byte(row.CreatedDirs), &op.CreatedDirs); err != nil {
			return nil, fmt.Errorf("operation %s: decode dirs: %w", row.ID, err)
		}
		if op.CreatedAt, err = time.Parse(timeLayout, row.CreatedAt); err != nil {
			return nil, fmt.Errorf("operation %s: %w", row.ID, err)
		}
		out = append(out, op)
	}
	return out, nil
}

// DeleteOperation implements domain.LibraryRepository.
func (r *LibraryRepository) DeleteOperation(ctx context.Context, id string) error {
	return r.q.DeleteOperation(ctx, id)
}

// Apply implements domain.LibraryRepository.
func (r *LibraryRepository) Apply(ctx context.Context, c domain.Changes) (domain.GameID, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	now := formatTime(c.Now)

	genres, err := json.Marshal(nonNil(c.Game.Genres))
	if err != nil {
		return 0, err
	}
	gameID := int64(c.Game.ID)
	if gameID == 0 {
		gameID, err = q.InsertGame(ctx, sqlcgen.InsertGameParams{
			ConsoleID: int64(c.Game.ConsoleID), IgdbID: c.Game.IGDBID, Title: c.Game.Title, Folder: c.Game.Folder,
			ReleaseYear: nullInt(c.Game.ReleaseYear), CoverImageID: nullString(c.Game.CoverImageID),
			Summary: nullString(c.Game.Summary), Genres: string(genres), CreatedAt: now, UpdatedAt: now,
		})
	} else {
		err = q.UpdateGameMetadata(ctx, sqlcgen.UpdateGameMetadataParams{
			Title: c.Game.Title, ReleaseYear: nullInt(c.Game.ReleaseYear), CoverImageID: nullString(c.Game.CoverImageID),
			Summary: nullString(c.Game.Summary), Genres: string(genres), UpdatedAt: now, ID: gameID,
		})
	}
	if err != nil {
		return 0, fmt.Errorf("save game: %w", err)
	}

	for _, t := range c.Trashed {
		if err := q.TrashGameItem(ctx, sqlcgen.TrashGameItemParams{
			TrashedAt: sql.NullString{String: now, Valid: true}, TrashDir: t.TrashDir, ID: int64(t.ID),
		}); err != nil {
			return 0, fmt.Errorf("trash item %d: %w", t.ID, err)
		}
	}
	for _, it := range c.New {
		files, err := json.Marshal(it.Files)
		if err != nil {
			return 0, err
		}
		if err := q.InsertGameItem(ctx, sqlcgen.InsertGameItemParams{
			GameID: gameID, Kind: string(it.Kind), Label: it.Label, DiscNumber: int64(it.DiscNumber),
			Shape: string(it.Shape), Files: string(files), Size: it.Size, TitleID: it.TitleID,
			SourceJob: it.SourceJob, CreatedAt: now,
		}); err != nil {
			return 0, fmt.Errorf("insert item: %w", err)
		}
	}
	if err := q.DeleteOperation(ctx, c.OperationID); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return domain.GameID(gameID), nil
}

func gameToDomain(row sqlcgen.Game) (*domain.Game, error) {
	g := &domain.Game{
		ID: domain.GameID(row.ID), ConsoleID: domain.ConsoleID(row.ConsoleID), IGDBID: row.IgdbID,
		Title: row.Title, Folder: row.Folder,
	}
	if row.ReleaseYear.Valid {
		y := int(row.ReleaseYear.Int64)
		g.ReleaseYear = &y
	}
	if row.CoverImageID.Valid {
		g.CoverImageID = &row.CoverImageID.String
	}
	if row.Summary.Valid {
		g.Summary = &row.Summary.String
	}
	if err := json.Unmarshal([]byte(row.Genres), &g.Genres); err != nil {
		return nil, fmt.Errorf("game %d: decode genres: %w", row.ID, err)
	}
	var err1, err2 error
	g.CreatedAt, err1 = time.Parse(timeLayout, row.CreatedAt)
	g.UpdatedAt, err2 = time.Parse(timeLayout, row.UpdatedAt)
	if err := errors.Join(err1, err2); err != nil {
		return nil, fmt.Errorf("game %d: %w", row.ID, err)
	}
	return g, nil
}

func itemToDomain(row sqlcgen.GameItem) (domain.GameItem, error) {
	it := domain.GameItem{
		ID: domain.ItemID(row.ID), GameID: domain.GameID(row.GameID), Kind: domain.ItemKind(row.Kind),
		Label: row.Label, DiscNumber: int(row.DiscNumber), Shape: domain.Shape(row.Shape), Size: row.Size,
		TitleID: row.TitleID, SourceJob: row.SourceJob, TrashDir: row.TrashDir,
	}
	if err := json.Unmarshal([]byte(row.Files), &it.Files); err != nil {
		return it, fmt.Errorf("item %d: decode files: %w", row.ID, err)
	}
	var err error
	if it.CreatedAt, err = time.Parse(timeLayout, row.CreatedAt); err != nil {
		return it, fmt.Errorf("item %d: %w", row.ID, err)
	}
	if row.TrashedAt.Valid {
		t, err := time.Parse(timeLayout, row.TrashedAt.String)
		if err != nil {
			return it, fmt.Errorf("item %d: %w", row.ID, err)
		}
		it.TrashedAt = &t
	}
	return it, nil
}

func nullInt(v *int) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func nullString(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}
