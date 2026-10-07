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

// ListGames implements domain.LibraryRepository.
func (r *LibraryRepository) ListGames(ctx context.Context) ([]domain.GameSummary, error) {
	rows, err := r.q.ListGameSummaries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.GameSummary, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.GameSummary{
			ID: domain.GameID(row.ID), ConsoleID: domain.ConsoleID(row.ConsoleID), Title: row.Title, Folder: row.Folder,
			ItemCount: int(row.ItemCount), MissingCount: int(row.MissingCount), Size: row.Size,
			ReleaseYear: intPtr(row.ReleaseYear), CoverImageID: stringPtr(row.CoverImageID),
		})
	}
	return out, nil
}

// GameByID implements domain.LibraryRepository.
func (r *LibraryRepository) GameByID(ctx context.Context, id domain.GameID) (*domain.Game, error) {
	row, err := r.q.GetGame(ctx, int64(id))
	return r.withItems(ctx, row, err)
}

// FindGame implements domain.LibraryRepository.
func (r *LibraryRepository) FindGame(ctx context.Context, console domain.ConsoleID, igdbID int64) (*domain.Game, error) {
	row, err := r.q.FindGameByIGDB(ctx, sqlcgen.FindGameByIGDBParams{ConsoleID: int64(console), IgdbID: igdbID})
	return r.withItems(ctx, row, err)
}

func (r *LibraryRepository) withItems(ctx context.Context, row sqlcgen.Game, err error) (*domain.Game, error) {
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

// ItemByID implements domain.LibraryRepository.
func (r *LibraryRepository) ItemByID(ctx context.Context, id domain.ItemID) (domain.GameItem, error) {
	row, err := r.q.GetGameItem(ctx, int64(id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.GameItem{}, domain.ErrItemNotFound
	}
	if err != nil {
		return domain.GameItem{}, err
	}
	return itemToDomain(row)
}

// FolderTaken implements domain.LibraryRepository.
func (r *LibraryRepository) FolderTaken(ctx context.Context, console domain.ConsoleID, folder string, except domain.GameID) (bool, error) {
	return r.q.FolderTaken(ctx, sqlcgen.FolderTakenParams{ConsoleID: int64(console), Folder: folder, ExceptID: int64(except)})
}

// HasItemsFrom implements domain.LibraryRepository.
func (r *LibraryRepository) HasItemsFrom(ctx context.Context, source string) (bool, error) {
	return r.q.HasItemsFromSource(ctx, source)
}

// TrashEntries implements domain.LibraryRepository.
func (r *LibraryRepository) TrashEntries(ctx context.Context) ([]domain.TrashEntry, error) {
	rows, err := r.q.ListTrashEntries(ctx)
	if err != nil {
		return nil, err
	}
	items, err := r.q.ListTrashItems(ctx)
	if err != nil {
		return nil, err
	}
	byEntry := map[int64][]domain.GameItem{}
	for _, row := range items {
		it, err := itemToDomain(row)
		if err != nil {
			return nil, err
		}
		byEntry[row.TrashEntryID.Int64] = append(byEntry[row.TrashEntryID.Int64], it)
	}
	out := make([]domain.TrashEntry, 0, len(rows))
	for _, row := range rows {
		e, err := entryToDomain(row)
		if err != nil {
			return nil, err
		}
		e.Items = byEntry[row.ID]
		out = append(out, e)
	}
	return out, nil
}

// TrashEntry implements domain.LibraryRepository.
func (r *LibraryRepository) TrashEntry(ctx context.Context, id domain.TrashEntryID) (*domain.TrashEntry, error) {
	row, err := r.q.GetTrashEntry(ctx, int64(id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrTrashEntryNotFound
	}
	if err != nil {
		return nil, err
	}
	e, err := entryToDomain(row)
	if err != nil {
		return nil, err
	}
	items, err := r.q.ListGameItems(ctx, row.GameID)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		if it.TrashEntryID.Valid && it.TrashEntryID.Int64 == row.ID {
			item, err := itemToDomain(it)
			if err != nil {
				return nil, err
			}
			e.Items = append(e.Items, item)
		}
	}
	return &e, nil
}

// SaveOperation implements domain.LibraryRepository.
func (r *LibraryRepository) SaveOperation(ctx context.Context, op domain.Operation) error {
	moves, err := json.Marshal(nonNil(op.Moves))
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
func (r *LibraryRepository) Apply(ctx context.Context, operationID string, fn func(domain.LibraryTx) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	if err := fn(libraryTx{q}); err != nil {
		return err
	}
	if operationID != "" {
		if err := q.DeleteOperation(ctx, operationID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// libraryTx implements domain.LibraryTx inside a transaction.
type libraryTx struct{ q *sqlcgen.Queries }

func (t libraryTx) InsertGame(ctx context.Context, g domain.Game, now time.Time) (domain.GameID, error) {
	genres, err := json.Marshal(nonNil(g.Genres))
	if err != nil {
		return 0, err
	}
	id, err := t.q.InsertGame(ctx, sqlcgen.InsertGameParams{
		ConsoleID: int64(g.ConsoleID), IgdbID: g.IGDBID, Title: g.Title, Folder: g.Folder,
		ReleaseYear: nullInt(g.ReleaseYear), CoverImageID: nullString(g.CoverImageID), Summary: nullString(g.Summary),
		Genres: string(genres), CreatedAt: formatTime(now), UpdatedAt: formatTime(now),
	})
	return domain.GameID(id), err
}

func (t libraryTx) UpdateGame(ctx context.Context, g domain.Game, now time.Time) error {
	genres, err := json.Marshal(nonNil(g.Genres))
	if err != nil {
		return err
	}
	return t.q.UpdateGame(ctx, sqlcgen.UpdateGameParams{
		IgdbID: g.IGDBID, Title: g.Title, Folder: g.Folder, ReleaseYear: nullInt(g.ReleaseYear),
		CoverImageID: nullString(g.CoverImageID), Summary: nullString(g.Summary), Genres: string(genres),
		UpdatedAt: formatTime(now), ID: int64(g.ID),
	})
}

func (t libraryTx) DeleteGame(ctx context.Context, id domain.GameID) error {
	return t.q.DeleteGame(ctx, int64(id))
}

func (t libraryTx) DeleteGameIfEmpty(ctx context.Context, id domain.GameID) error {
	n, err := t.q.CountGameItems(ctx, int64(id))
	if err != nil || n > 0 {
		return err
	}
	return t.q.DeleteGame(ctx, int64(id))
}

func (t libraryTx) InsertItem(ctx context.Context, it domain.GameItem, now time.Time) (domain.ItemID, error) {
	files, err := json.Marshal(it.Files)
	if err != nil {
		return 0, err
	}
	id, err := t.q.InsertGameItem(ctx, sqlcgen.InsertGameItemParams{
		GameID: int64(it.GameID), Kind: string(it.Kind), Label: it.Label, DiscNumber: int64(it.DiscNumber),
		Shape: string(it.Shape), Files: string(files), Size: it.Size, TitleID: it.TitleID,
		SourceJob: it.SourceJob, CreatedAt: formatTime(now),
	})
	return domain.ItemID(id), err
}

func (t libraryTx) PlaceItem(ctx context.Context, id domain.ItemID, game domain.GameID, files []string) error {
	b, err := json.Marshal(files)
	if err != nil {
		return err
	}
	return t.q.UpdateItemPlace(ctx, sqlcgen.UpdateItemPlaceParams{GameID: int64(game), Files: string(b), ID: int64(id)})
}

func (t libraryTx) DeleteItem(ctx context.Context, id domain.ItemID) error {
	return t.q.DeleteGameItem(ctx, int64(id))
}

func (t libraryTx) SetMissing(ctx context.Context, id domain.ItemID, since *time.Time) error {
	v := sql.NullString{}
	if since != nil {
		v = sql.NullString{String: formatTime(*since), Valid: true}
	}
	return t.q.SetItemMissing(ctx, sqlcgen.SetItemMissingParams{MissingSince: v, ID: int64(id)})
}

func (t libraryTx) MoveGameContents(ctx context.Context, from, to domain.GameID) error {
	if err := t.q.MoveGameItems(ctx, sqlcgen.MoveGameItemsParams{ToGame: int64(to), FromGame: int64(from)}); err != nil {
		return err
	}
	return t.q.MoveTrashEntries(ctx, sqlcgen.MoveTrashEntriesParams{ToGame: int64(to), FromGame: int64(from)})
}

func (t libraryTx) InsertTrashEntry(ctx context.Context, e domain.TrashEntry) (domain.TrashEntryID, error) {
	whole := int64(0)
	if e.WholeGame {
		whole = 1
	}
	id, err := t.q.InsertTrashEntry(ctx, sqlcgen.InsertTrashEntryParams{
		GameID: int64(e.GameID), WholeGame: whole, Reason: string(e.Reason), Dir: e.Dir, TrashedAt: formatTime(e.TrashedAt),
	})
	return domain.TrashEntryID(id), err
}

func (t libraryTx) SetItemTrash(ctx context.Context, id domain.ItemID, entry *domain.TrashEntryID) error {
	v := sql.NullInt64{}
	if entry != nil {
		v = sql.NullInt64{Int64: int64(*entry), Valid: true}
	}
	return t.q.SetItemTrash(ctx, sqlcgen.SetItemTrashParams{TrashEntryID: v, ID: int64(id)})
}

func (t libraryTx) DeleteTrashEntry(ctx context.Context, id domain.TrashEntryID) error {
	return t.q.DeleteTrashEntry(ctx, int64(id))
}

func gameToDomain(row sqlcgen.Game) (*domain.Game, error) {
	g := &domain.Game{
		ID: domain.GameID(row.ID), ConsoleID: domain.ConsoleID(row.ConsoleID), IGDBID: row.IgdbID,
		Title: row.Title, Folder: row.Folder, ReleaseYear: intPtr(row.ReleaseYear),
		CoverImageID: stringPtr(row.CoverImageID), Summary: stringPtr(row.Summary),
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
		TitleID: row.TitleID, SourceJob: row.SourceJob,
	}
	if err := json.Unmarshal([]byte(row.Files), &it.Files); err != nil {
		return it, fmt.Errorf("item %d: decode files: %w", row.ID, err)
	}
	var err error
	if it.CreatedAt, err = time.Parse(timeLayout, row.CreatedAt); err != nil {
		return it, fmt.Errorf("item %d: %w", row.ID, err)
	}
	if row.TrashEntryID.Valid {
		e := domain.TrashEntryID(row.TrashEntryID.Int64)
		it.TrashEntry = &e
	}
	if row.MissingSince.Valid {
		t, err := time.Parse(timeLayout, row.MissingSince.String)
		if err != nil {
			return it, fmt.Errorf("item %d: %w", row.ID, err)
		}
		it.MissingSince = &t
	}
	return it, nil
}

func entryToDomain(row sqlcgen.TrashEntry) (domain.TrashEntry, error) {
	t, err := time.Parse(timeLayout, row.TrashedAt)
	if err != nil {
		return domain.TrashEntry{}, fmt.Errorf("trash entry %d: %w", row.ID, err)
	}
	return domain.TrashEntry{
		ID: domain.TrashEntryID(row.ID), GameID: domain.GameID(row.GameID), WholeGame: row.WholeGame == 1,
		Reason: domain.TrashReason(row.Reason), Dir: row.Dir, TrashedAt: t,
	}, nil
}

func intPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func stringPtr(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	s := v.String
	return &s
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
