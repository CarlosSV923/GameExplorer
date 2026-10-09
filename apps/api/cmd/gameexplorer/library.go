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
	for _, r := range p.Renamed {
		out.Renamed = append(out.Renamed, ingestionapp.RenamedItem{Item: existingItem(r.Item), File: r.File})
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
		Reason: catalogdomain.UnassignedReason(req.Reason), Console: catalogdomain.Slug(req.Console), IGDBID: req.IGDBID,
		ToTrash: req.ToTrash,
	}))
}

func (l libraryPort) TakeEntry(ctx context.Context, id int64, job string) (ingestionapp.UnassignedEntry, error) {
	e, err := l.svc.TakeEntry(ctx, catalogdomain.UnassignedID(id), job)
	if err != nil {
		return ingestionapp.UnassignedEntry{}, libraryError(err)
	}
	out := ingestionapp.UnassignedEntry{Name: e.Name, Files: l.sources(e.Files)}
	for _, f := range out.Files {
		out.Size += f.Size
	}
	return out, nil
}

func (l libraryPort) JobFiles(ctx context.Context, job string) ([]ingestionapp.UnassignedSource, error) {
	files, err := l.svc.JobFiles(ctx, job)
	if err != nil {
		return nil, libraryError(err)
	}
	return l.sources(files), nil
}

func (l libraryPort) sources(files []catalogdomain.UnassignedFile) []ingestionapp.UnassignedSource {
	out := make([]ingestionapp.UnassignedSource, 0, len(files))
	for _, f := range files {
		out = append(out, ingestionapp.UnassignedSource{
			ID: int64(f.ID), Name: f.Name(), Path: f.Path, Abs: l.svc.UnassignedPath(f.Path), Size: f.Size,
		})
	}
	return out
}

func (l libraryPort) ReleaseJob(ctx context.Context, job string) error {
	return libraryError(l.svc.ReleaseJob(ctx, job))
}

func (l libraryPort) DeleteJobFiles(ctx context.Context, job string, ids []int64) error {
	out := make([]catalogdomain.UnassignedID, len(ids))
	for i, id := range ids {
		out[i] = catalogdomain.UnassignedID(id)
	}
	return libraryError(l.svc.DeleteJobFiles(ctx, job, out))
}

func (l libraryPort) UnassignedDir() string { return l.svc.UnassignedPath("") }

func storeRequest(req ingestionapp.LibraryRequest) catalogapp.StoreRequest {
	out := catalogapp.StoreRequest{
		Source: req.Source, Console: req.Console, Name: catalogapp.GameName{Title: req.Title, IGDBID: req.IGDBID},
	}
	for _, r := range req.Renumber {
		out.Renumber = append(out.Renumber, catalogapp.Renumbering{Item: catalogdomain.ItemID(r.Item), Label: r.Label})
	}
	for _, f := range req.Files {
		out.Files = append(out.Files, catalogapp.NewFile{
			Ref: f.Ref, Root: f.Root, Path: f.Path, Size: f.Size,
			Kind: catalogdomain.ItemKind(f.Kind), Label: f.Label, OnDuplicate: catalogdomain.DuplicateAction(f.OnDuplicate),
			Unassigned: catalogdomain.UnassignedID(f.Unassigned),
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
