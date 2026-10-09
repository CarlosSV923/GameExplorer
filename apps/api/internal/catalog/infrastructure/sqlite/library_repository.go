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
			ID: domain.GameID(row.ID), Console: domain.Slug(row.Console), IGDBID: int64Ptr(row.IgdbID),
			Title: row.Title, Folder: row.Folder, ItemCount: int(row.ItemCount), Size: row.Size,
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

// GameByFolder implements domain.LibraryRepository.
func (r *LibraryRepository) GameByFolder(ctx context.Context, console domain.Slug, folder string) (*domain.Game, error) {
	row, err := r.q.GetGameByFolder(ctx, sqlcgen.GetGameByFolderParams{Console: string(console), Folder: folder})
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

// LibraryFiles implements domain.LibraryRepository.
func (r *LibraryRepository) LibraryFiles(ctx context.Context) ([]domain.LibraryFile, error) {
	rows, err := r.q.ListLibraryFiles(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.LibraryFile, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.LibraryFile{
			Item: domain.ItemID(row.ID), Game: domain.GameID(row.GameID), Console: domain.Slug(row.Console),
			Folder: row.Folder, File: row.File,
		})
	}
	return out, nil
}

// HasItemsFrom implements domain.LibraryRepository.
func (r *LibraryRepository) HasItemsFrom(ctx context.Context, source string) (bool, error) {
	return r.q.HasItemsFromSource(ctx, source)
}

// UnassignedFiles implements domain.LibraryRepository.
func (r *LibraryRepository) UnassignedFiles(ctx context.Context) ([]domain.UnassignedFile, error) {
	rows, err := r.q.ListUnassigned(ctx)
	if err != nil {
		return nil, err
	}
	return unassignedToDomainAll(rows)
}

// UnassignedByID implements domain.LibraryRepository.
func (r *LibraryRepository) UnassignedByID(ctx context.Context, id domain.UnassignedID) (domain.UnassignedFile, error) {
	row, err := r.q.GetUnassigned(ctx, int64(id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.UnassignedFile{}, domain.ErrUnassignedNotFound
	}
	if err != nil {
		return domain.UnassignedFile{}, err
	}
	return unassignedToDomain(row)
}

// SetUnassignedJob implements domain.LibraryRepository.
func (r *LibraryRepository) SetUnassignedJob(ctx context.Context, id domain.UnassignedID, job string) error {
	return r.q.SetUnassignedJob(ctx, sqlcgen.SetUnassignedJobParams{JobID: job, ID: int64(id)})
}

// ReleaseUnassignedJob implements domain.LibraryRepository.
func (r *LibraryRepository) ReleaseUnassignedJob(ctx context.Context, job string) error {
	return r.q.ReleaseUnassignedJob(ctx, job)
}

// TrashEntries implements domain.LibraryRepository.
func (r *LibraryRepository) TrashEntries(ctx context.Context) ([]domain.TrashEntry, error) {
	rows, err := r.q.ListTrashEntries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.TrashEntry, 0, len(rows))
	for _, row := range rows {
		e, err := entryToDomain(row)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return r.withTrashFiles(ctx, out)
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
	out, err := r.withTrashFiles(ctx, []domain.TrashEntry{e})
	if err != nil {
		return nil, err
	}
	return &out[0], nil
}

// withTrashFiles fills the entries' game files and unassigned files.
func (r *LibraryRepository) withTrashFiles(ctx context.Context, entries []domain.TrashEntry) ([]domain.TrashEntry, error) {
	items, err := r.q.ListTrashItems(ctx)
	if err != nil {
		return nil, err
	}
	files, err := r.q.ListTrashUnassigned(ctx)
	if err != nil {
		return nil, err
	}
	byEntry := map[domain.TrashEntryID]*domain.TrashEntry{}
	for i := range entries {
		byEntry[entries[i].ID] = &entries[i]
	}
	for _, row := range items {
		e, ok := byEntry[domain.TrashEntryID(row.TrashEntryID.Int64)]
		if !ok {
			continue
		}
		it, err := itemToDomain(row)
		if err != nil {
			return nil, err
		}
		e.Items = append(e.Items, it)
	}
	for _, row := range files {
		e, ok := byEntry[domain.TrashEntryID(row.TrashEntryID.Int64)]
		if !ok {
			continue
		}
		f, err := unassignedToDomain(row)
		if err != nil {
			return nil, err
		}
		e.Files = append(e.Files, f)
	}
	return entries, nil
}

// PendingFiles implements domain.LibraryRepository.
func (r *LibraryRepository) PendingFiles(ctx context.Context) ([]domain.PendingFile, error) {
	rows, err := r.q.ListScanPending(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PendingFile, 0, len(rows))
	for _, row := range rows {
		t, err := time.Parse(time.RFC3339Nano, row.ModTime)
		if err != nil {
			return nil, fmt.Errorf("pending %q: %w", row.Path, err)
		}
		out = append(out, domain.PendingFile{Path: row.Path, Size: row.Size, ModTime: t})
	}
	return out, nil
}

// SetPendingFiles implements domain.LibraryRepository.
func (r *LibraryRepository) SetPendingFiles(ctx context.Context, files []domain.PendingFile) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	if err := q.ClearScanPending(ctx); err != nil {
		return err
	}
	for _, f := range files {
		if err := q.InsertScanPending(ctx, sqlcgen.InsertScanPendingParams{
			Path: f.Path, Size: f.Size, ModTime: f.ModTime.UTC().Format(time.RFC3339Nano),
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
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
		Console: string(g.Console), Title: g.Title, Folder: g.Folder, IgdbID: nullInt64(g.IGDBID),
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
		Console: string(g.Console), Title: g.Title, Folder: g.Folder, IgdbID: nullInt64(g.IGDBID),
		ReleaseYear: nullInt(g.ReleaseYear), CoverImageID: nullString(g.CoverImageID), Summary: nullString(g.Summary),
		Genres: string(genres), UpdatedAt: formatTime(now), ID: int64(g.ID),
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
	id, err := t.q.InsertGameItem(ctx, sqlcgen.InsertGameItemParams{
		GameID: int64(it.GameID), Kind: string(it.Kind), Label: it.Label, File: it.File, Size: it.Size,
		SourceJob: it.SourceJob, CreatedAt: formatTime(now),
	})
	return domain.ItemID(id), err
}

func (t libraryTx) UpdateItem(ctx context.Context, it domain.GameItem) error {
	return t.q.UpdateGameItem(ctx, sqlcgen.UpdateGameItemParams{
		GameID: int64(it.GameID), Kind: string(it.Kind), Label: it.Label, File: it.File, ID: int64(it.ID),
	})
}

func (t libraryTx) DeleteItem(ctx context.Context, id domain.ItemID) error {
	return t.q.DeleteGameItem(ctx, int64(id))
}

func (t libraryTx) MoveGameContents(ctx context.Context, from, to domain.GameID) error {
	if err := t.q.MoveGameItems(ctx, sqlcgen.MoveGameItemsParams{ToGame: int64(to), FromGame: int64(from)}); err != nil {
		return err
	}
	return t.q.MoveTrashEntries(ctx, sqlcgen.MoveTrashEntriesParams{
		ToGame: sql.NullInt64{Int64: int64(to), Valid: true}, FromGame: sql.NullInt64{Int64: int64(from), Valid: true},
	})
}

func (t libraryTx) InsertTrashEntry(ctx context.Context, e domain.TrashEntry) (domain.TrashEntryID, error) {
	whole := int64(0)
	if e.WholeGame {
		whole = 1
	}
	var game sql.NullInt64
	if e.GameID != nil {
		game = sql.NullInt64{Int64: int64(*e.GameID), Valid: true}
	}
	id, err := t.q.InsertTrashEntry(ctx, sqlcgen.InsertTrashEntryParams{
		GameID: game, WholeGame: whole, Reason: string(e.Reason), Dir: e.Dir, TrashedAt: formatTime(e.TrashedAt),
	})
	return domain.TrashEntryID(id), err
}

func (t libraryTx) SetItemTrash(ctx context.Context, id domain.ItemID, entry *domain.TrashEntryID) error {
	return t.q.SetItemTrash(ctx, sqlcgen.SetItemTrashParams{TrashEntryID: nullEntry(entry), ID: int64(id)})
}

func (t libraryTx) DeleteTrashEntry(ctx context.Context, id domain.TrashEntryID) error {
	return t.q.DeleteTrashEntry(ctx, int64(id))
}

func (t libraryTx) InsertUnassigned(ctx context.Context, f domain.UnassignedFile) (domain.UnassignedID, error) {
	id, err := t.q.InsertUnassigned(ctx, sqlcgen.InsertUnassignedParams{
		Path: f.Path, Origin: f.Origin, Reason: string(f.Reason), Size: f.Size,
		ArrivedAt: formatTime(f.ArrivedAt), TrashEntryID: nullEntry(f.TrashEntry),
		Console: string(f.Console), IgdbID: nullInt64(f.IGDBID),
	})
	return domain.UnassignedID(id), err
}

func (t libraryTx) SetUnassignedPlace(ctx context.Context, id domain.UnassignedID, path string, entry *domain.TrashEntryID) error {
	return t.q.SetUnassignedPlace(ctx, sqlcgen.SetUnassignedPlaceParams{Path: path, TrashEntryID: nullEntry(entry), ID: int64(id)})
}

func (t libraryTx) DeleteUnassigned(ctx context.Context, id domain.UnassignedID) error {
	return t.q.DeleteUnassigned(ctx, int64(id))
}

func gameToDomain(row sqlcgen.Game) (*domain.Game, error) {
	g := &domain.Game{
		ID: domain.GameID(row.ID), Console: domain.Slug(row.Console), IGDBID: int64Ptr(row.IgdbID),
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
		Label: row.Label, File: row.File, Size: row.Size, SourceJob: row.SourceJob,
		TrashEntry: entryPtr(row.TrashEntryID),
	}
	var err error
	if it.CreatedAt, err = time.Parse(timeLayout, row.CreatedAt); err != nil {
		return it, fmt.Errorf("item %d: %w", row.ID, err)
	}
	return it, nil
}

func unassignedToDomain(row sqlcgen.UnassignedFile) (domain.UnassignedFile, error) {
	f := domain.UnassignedFile{
		ID: domain.UnassignedID(row.ID), Path: row.Path, Origin: row.Origin, Reason: domain.UnassignedReason(row.Reason),
		Size: row.Size, TrashEntry: entryPtr(row.TrashEntryID),
		Console: domain.Slug(row.Console), IGDBID: int64Ptr(row.IgdbID), Job: row.JobID,
	}
	var err error
	if f.ArrivedAt, err = time.Parse(timeLayout, row.ArrivedAt); err != nil {
		return f, fmt.Errorf("unassigned %d: %w", row.ID, err)
	}
	return f, nil
}

func unassignedToDomainAll(rows []sqlcgen.UnassignedFile) ([]domain.UnassignedFile, error) {
	out := make([]domain.UnassignedFile, 0, len(rows))
	for _, row := range rows {
		f, err := unassignedToDomain(row)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func entryToDomain(row sqlcgen.TrashEntry) (domain.TrashEntry, error) {
	t, err := time.Parse(timeLayout, row.TrashedAt)
	if err != nil {
		return domain.TrashEntry{}, fmt.Errorf("trash entry %d: %w", row.ID, err)
	}
	e := domain.TrashEntry{
		ID: domain.TrashEntryID(row.ID), WholeGame: row.WholeGame == 1,
		Reason: domain.TrashReason(row.Reason), Dir: row.Dir, TrashedAt: t,
	}
	if row.GameID.Valid {
		g := domain.GameID(row.GameID.Int64)
		e.GameID = &g
	}
	return e, nil
}

func entryPtr(v sql.NullInt64) *domain.TrashEntryID {
	if !v.Valid {
		return nil
	}
	e := domain.TrashEntryID(v.Int64)
	return &e
}

func nullEntry(e *domain.TrashEntryID) sql.NullInt64 {
	if e == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*e), Valid: true}
}

func intPtr(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

func int64Ptr(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
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

func nullInt64(v *int64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: *v, Valid: true}
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
