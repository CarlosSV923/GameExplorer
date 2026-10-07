package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/detection"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/infrastructure/sqlite/sqlcgen"
)

// ItemRepository implements domain.StagedItemRepository.
type ItemRepository struct {
	db *sql.DB
	q  *sqlcgen.Queries
}

var _ domain.StagedItemRepository = (*ItemRepository)(nil)

// NewItemRepository builds the repository.
func NewItemRepository(db *sql.DB) *ItemRepository {
	return &ItemRepository{db: db, q: sqlcgen.New(db)}
}

// Replace implements domain.StagedItemRepository atomically.
func (r *ItemRepository) Replace(ctx context.Context, id domain.JobID, items []domain.StagedItem) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	if err := q.DeleteItems(ctx, string(id)); err != nil {
		return err
	}
	for _, it := range items {
		parts, err1 := json.Marshal(it.Parts)
		consoles, err2 := json.Marshal(it.Consoles)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("encode item %s", it.Path)
		}
		ignored := int64(0)
		if it.Ignored {
			ignored = 1
		}
		if err := q.InsertItem(ctx, sqlcgen.InsertItemParams{
			JobID: string(id), Path: it.Path, Shape: string(it.Shape), Parts: string(parts), Size: it.Size,
			Ignored: ignored, Consoles: string(consoles), Confidence: string(it.Confidence),
			SuggestedKind: string(it.SuggestedKind), TitleID: it.TitleID, VersionCode: it.VersionCode,
			DisplayVersion: it.DisplayVersion, DiscNumber: int64(it.DiscNumber),
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// List implements domain.StagedItemRepository.
func (r *ItemRepository) List(ctx context.Context, id domain.JobID) ([]domain.StagedItem, error) {
	rows, err := r.q.ListItems(ctx, string(id))
	if err != nil {
		return nil, err
	}
	out := make([]domain.StagedItem, 0, len(rows))
	for _, row := range rows {
		it := domain.StagedItem{
			JobID: domain.JobID(row.JobID), Path: row.Path, Shape: domain.ItemShape(row.Shape), Size: row.Size,
			Ignored: row.Ignored != 0, Confidence: domain.Confidence(row.Confidence),
			SuggestedKind: detection.ItemKind(row.SuggestedKind), TitleID: row.TitleID, VersionCode: row.VersionCode,
			DisplayVersion: row.DisplayVersion, DiscNumber: int(row.DiscNumber),
		}
		if err := json.Unmarshal([]byte(row.Parts), &it.Parts); err != nil {
			return nil, fmt.Errorf("item %s parts: %w", row.Path, err)
		}
		if err := json.Unmarshal([]byte(row.Consoles), &it.Consoles); err != nil {
			return nil, fmt.Errorf("item %s consoles: %w", row.Path, err)
		}
		out = append(out, it)
	}
	return out, nil
}
