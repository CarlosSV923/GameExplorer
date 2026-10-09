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

// consoleRules adapts the catalog's consoles to ingestion's validation.
type consoleRules struct{ svc *catalogapp.ConsoleService }

func (c consoleRules) Rules(ctx context.Context) (map[string]ingestiondomain.ConsoleRule, []string, error) {
	consoles, err := c.svc.List(ctx)
	if err != nil {
		return nil, nil, err
	}
	out := make(map[string]ingestiondomain.ConsoleRule, len(consoles))
	for _, con := range consoles {
		out[string(con.Slug)] = ingestiondomain.ConsoleRule{
			Slug: string(con.Slug), Extensions: con.AllExtensions(), MultipleFiles: con.MultipleFiles,
		}
	}
	return out, catalogapp.KnownExtensions(consoles), nil
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
	for _, f := range p.Files {
		planned := ingestionapp.PlannedFile{Path: f.Ref, File: f.File, Action: string(f.Action)}
		if f.Duplicate != nil {
			d := existingItem(*f.Duplicate)
			planned.Duplicate = &d
		}
		out.Files = append(out.Files, planned)
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

func (l libraryPort) PutAside(ctx context.Context, req ingestionapp.SetAsideRequest) error {
	return libraryError(l.svc.PutAside(ctx, catalogapp.SetAside{
		Source: req.Source, Root: req.Root, Files: req.Files, Folder: req.Folder, Origin: req.Origin,
		Reason: catalogdomain.UnassignedReason(req.Reason), ToTrash: req.ToTrash,
	}))
}

func (l libraryPort) UnassignedFile(ctx context.Context, id int64) (ingestionapp.UnassignedSource, error) {
	_, f, err := l.svc.UnassignedFile(ctx, catalogdomain.UnassignedID(id))
	if err != nil {
		return ingestionapp.UnassignedSource{}, libraryError(err)
	}
	return ingestionapp.UnassignedSource{
		Name: f.Name(), Path: f.Path, Size: f.Size, Origin: f.Origin, Reason: string(f.Reason),
	}, nil
}

func (l libraryPort) TakeUnassigned(ctx context.Context, id int64, dest string) (ingestionapp.UnassignedSource, error) {
	f, err := l.svc.TakeUnassigned(ctx, catalogdomain.UnassignedID(id), dest)
	if err != nil {
		return ingestionapp.UnassignedSource{}, libraryError(err)
	}
	return ingestionapp.UnassignedSource{
		Name: f.Name(), Path: f.Path, Size: f.Size, Origin: f.Origin, Reason: string(f.Reason),
	}, nil
}

func storeRequest(req ingestionapp.LibraryRequest) catalogapp.StoreRequest {
	out := catalogapp.StoreRequest{
		Source: req.Source, Console: req.Console, Name: catalogapp.GameName{Title: req.Title, IGDBID: req.IGDBID},
	}
	for _, f := range req.Files {
		out.Files = append(out.Files, catalogapp.NewFile{
			Ref: f.Ref, Root: f.Root, Path: f.Path, Size: f.Size,
			Kind: catalogdomain.ItemKind(f.Kind), Label: f.Label, OnDuplicate: catalogdomain.DuplicateAction(f.OnDuplicate),
		})
	}
	return out
}

func existingItem(e catalogdomain.GameItem) ingestionapp.ExistingItem {
	return ingestionapp.ExistingItem{
		ID: int64(e.ID), Kind: string(e.Kind), Label: e.Label, File: e.File, Size: e.Size, CreatedAt: e.CreatedAt,
	}
}

func libraryError(err error) error {
	var rej *catalogapp.Rejection
	switch {
	case err == nil:
		return nil
	case errors.As(err, &rej):
		return &ingestionapp.CommitRejected{Reason: ingestionapp.RejectReason(rej.Reason), Message: rej.Message}
	case errors.Is(err, catalogapp.ErrUndoFailed):
		return fmt.Errorf("%w: %w", ingestionapp.ErrLibraryInconsistent, err)
	case errors.Is(err, catalogdomain.ErrUnassignedNotFound):
		return fmt.Errorf("%w: %w", ingestionapp.ErrUnassignedNotFound, err)
	}
	return err
}
