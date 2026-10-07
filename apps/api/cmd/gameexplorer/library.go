package main

import (
	"context"
	"errors"
	"fmt"

	catalogapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	catalogdomain "github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	ingestionapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	ingestiondomain "github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	metadataapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/application"
	metadatadomain "github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
)

// gameDirectory adapts the metadata context to the catalog's port.
type gameDirectory struct{ svc *metadataapp.Service }

func (d gameDirectory) GameByID(ctx context.Context, id int64) (catalogapp.GameInfo, error) {
	g, err := d.svc.GameByID(ctx, id)
	switch {
	case errors.Is(err, metadatadomain.ErrNotFound):
		return catalogapp.GameInfo{}, catalogapp.ErrUnknownGame
	case errors.Is(err, metadatadomain.ErrNotConfigured):
		return catalogapp.GameInfo{}, catalogapp.ErrMetadataNotConfigured
	case errors.Is(err, metadatadomain.ErrUpstream):
		return catalogapp.GameInfo{}, fmt.Errorf("%w: %w", catalogapp.ErrMetadataUnavailable, err)
	case err != nil:
		return catalogapp.GameInfo{}, err
	}
	return catalogapp.GameInfo{
		IGDBID: g.ID, Name: g.Name, ReleaseYear: g.ReleaseYear,
		CoverImageID: g.CoverImageID, Summary: g.Summary, Genres: g.Genres,
	}, nil
}

// libraryPort adapts the catalog's library service to ingestion's port.
type libraryPort struct{ svc *catalogapp.LibraryService }

func (l libraryPort) Plan(ctx context.Context, req ingestionapp.LibraryRequest) (ingestionapp.CommitPlan, error) {
	p, err := l.svc.Plan(ctx, storeRequest(req))
	if err != nil {
		return ingestionapp.CommitPlan{}, libraryError(err)
	}
	out := ingestionapp.CommitPlan{Console: p.Console, Title: p.Title, Folder: p.Folder, GameID: int64(p.GameID)}
	for _, e := range p.Existing {
		out.Existing = append(out.Existing, existingItem(e))
	}
	for _, it := range p.Items {
		planned := ingestionapp.PlannedItem{Path: it.Ref, Files: it.Files, Action: string(it.Action)}
		if it.Duplicate != nil {
			d := existingItem(*it.Duplicate)
			planned.Duplicate = &d
		}
		out.Items = append(out.Items, planned)
	}
	return out, nil
}

func (l libraryPort) Store(ctx context.Context, req ingestionapp.LibraryRequest) (ingestionapp.CommitResult, error) {
	res, err := l.svc.Store(ctx, storeRequest(req))
	out := ingestionapp.CommitResult{
		GameID: int64(res.GameID), Path: res.Path, Stored: res.Stored, Replaced: res.Replaced, Skipped: res.Skipped,
	}
	if err != nil {
		return out, libraryError(err)
	}
	return out, nil
}

func (l libraryPort) Recover(ctx context.Context) (int, error) { return l.svc.Recover(ctx) }

func (l libraryPort) HasItemsFrom(ctx context.Context, source string) (bool, error) {
	return l.svc.HasItemsFrom(ctx, source)
}

func storeRequest(req ingestionapp.LibraryRequest) catalogapp.StoreRequest {
	out := catalogapp.StoreRequest{Source: req.Source, Console: req.Console, IGDBGameID: req.IGDBGameID}
	for _, it := range req.Items {
		out.Items = append(out.Items, catalogapp.NewItem{
			Ref: it.Ref, Shape: shape(it.Shape), Root: it.Root, Parts: it.Parts, Size: it.Size,
			Kind: catalogdomain.ItemKind(it.Kind), Label: it.Label, DiscNumber: it.DiscNumber, TitleID: it.TitleID,
			OnDuplicate: catalogdomain.DuplicateAction(it.OnDuplicate),
		})
	}
	return out
}

func shape(s ingestiondomain.ItemShape) catalogdomain.Shape {
	switch s {
	case ingestiondomain.ShapeDisc:
		return catalogdomain.ShapeDisc
	case ingestiondomain.ShapeFolder:
		return catalogdomain.ShapeFolder
	default:
		return catalogdomain.ShapeFile
	}
}

func existingItem(e catalogdomain.GameItem) ingestionapp.ExistingItem {
	return ingestionapp.ExistingItem{
		ID: int64(e.ID), Kind: string(e.Kind), Label: e.Label, DiscNumber: e.DiscNumber, Files: e.Files, Size: e.Size,
	}
}

func libraryError(err error) error {
	var rej *catalogapp.Rejection
	switch {
	case errors.As(err, &rej):
		return &ingestionapp.CommitRejected{Reason: ingestionapp.RejectReason(rej.Reason), Message: rej.Message}
	case errors.Is(err, catalogapp.ErrUndoFailed):
		return fmt.Errorf("%w: %w", ingestionapp.ErrLibraryInconsistent, err)
	}
	return err
}
