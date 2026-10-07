package application

import (
	"context"
	"errors"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// RematchRequest matches a game to another IGDB entry (RF-24).
type RematchRequest struct {
	GameID     domain.GameID
	IGDBGameID int64
	// Decisions are needed when the new IGDB game is already in the library
	// (the games merge) and an item is a duplicate there: Replace sends the
	// target's item to the trash, Skip sends this game's item to the trash.
	Decisions map[domain.ItemID]domain.DuplicateAction
}

// RematchItem is what happens to one item of the game.
type RematchItem struct {
	Item      domain.GameItem
	Files     []string // new names
	Duplicate *domain.GameItem
	Action    Action
}

// RematchPlan previews a re-match.
type RematchPlan struct {
	Console string
	Title   string
	Folder  string
	// MergeInto is the game that already has the new IGDB id (0 if none).
	MergeInto domain.GameID
	Items     []RematchItem

	game    *domain.Game
	target  *domain.Game
	console domain.Console
	info    GameInfo
}

// RematchResult says where the game ended up.
type RematchResult struct {
	GameID domain.GameID
	Path   string
	Merged bool
}

// PlanRematch previews a re-match without touching anything.
func (s *LibraryService) PlanRematch(ctx context.Context, req RematchRequest) (*RematchPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planRematch(ctx, req)
}

func (s *LibraryService) planRematch(ctx context.Context, req RematchRequest) (*RematchPlan, error) {
	g, err := s.repo.GameByID(ctx, req.GameID)
	if err != nil {
		return nil, err
	}
	items := g.LiveItems()
	if len(items) == 0 {
		return nil, domain.ErrGameNotFound
	}
	console, err := s.consoleByID(ctx, g.ConsoleID)
	if err != nil {
		return nil, err
	}
	info, err := s.gameInfo(ctx, req.IGDBGameID)
	if err != nil {
		return nil, err
	}
	p := &RematchPlan{Console: string(console.Slug), game: g, console: console, info: info}

	var existing []domain.GameItem
	target, err := s.repo.FindGame(ctx, console.ID, req.IGDBGameID)
	switch {
	case err == nil && target.ID != g.ID:
		p.target, p.MergeInto = target, target.ID
		p.Title, p.Folder = target.Title, target.Folder
		existing = target.LiveItems()
	case err == nil, errors.Is(err, domain.ErrGameNotFound):
		p.Title = info.Name
		if p.Folder, err = s.newFolder(ctx, console, info, g); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	names := nameSet{}
	replaced := map[domain.ItemID]bool{}
	for _, it := range items {
		files, err := storedNames(p.Title, it)
		if err != nil {
			return nil, err
		}
		ri := RematchItem{Item: it, Files: files, Action: ActionStore}
		ri.Duplicate = domain.FindDuplicate(existing, domain.Candidate{Kind: it.Kind, Label: it.Label, DiscNumber: it.DiscNumber, Files: files})
		if ri.Duplicate != nil {
			ri.Action = decide(req.Decisions[it.ID])
			if ri.Action == ActionReplace {
				if replaced[ri.Duplicate.ID] {
					return nil, reject(RejectInvalid, "Dos elementos reemplazarían a %q.", ri.Duplicate.Files[0])
				}
				replaced[ri.Duplicate.ID] = true
			}
		}
		if ri.Action != ActionSkip {
			if err := names.add(it.Files[0], files); err != nil {
				return nil, err
			}
		}
		p.Items = append(p.Items, ri)
	}
	return p, nil
}

// Rematch renames the game's folder and files for the new IGDB entry, or
// merges it into the game that already has it. Reversible like a commit.
func (s *LibraryService) Rematch(ctx context.Context, req RematchRequest) (RematchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.planRematch(ctx, req)
	if err != nil {
		return RematchResult{}, err
	}
	for _, it := range p.Items {
		if it.Action == ActionUndecided {
			return RematchResult{}, reject(RejectConflict, "%q ya existe en %q: elige Reemplazar u Omitir.", it.Item.Files[0], p.Folder)
		}
	}
	if p.target != nil {
		return s.merge(ctx, p)
	}
	return s.rename(ctx, p)
}

// rename moves the folder (if its name changes) and renames the files inside.
func (s *LibraryService) rename(ctx context.Context, p *RematchPlan) (RematchResult, error) {
	g := p.game
	b := s.newOp("rematch")
	oldDir := s.files.LibraryPath(p.Console, g.Folder)
	newDir := s.files.LibraryPath(p.Console, p.Folder)
	exists, err := s.files.Exists(oldDir)
	if err != nil {
		return RematchResult{}, err
	}
	if exists {
		b.move(oldDir, newDir)
	} else {
		b.mkdir(newDir)
	}
	for _, it := range p.Items {
		if err := b.placeAfter(oldDir, it.Item.Shape, newDir, it.Item.Files, newDir, it.Files, false); err != nil {
			return RematchResult{}, err
		}
	}
	updated := *g
	updated.IGDBID, updated.Title, updated.Folder = p.info.IGDBID, p.Title, p.Folder
	updated.ReleaseYear, updated.CoverImageID, updated.Summary, updated.Genres = p.info.ReleaseYear, p.info.CoverImageID, p.info.Summary, p.info.Genres
	now := s.now()
	err = s.run(ctx, b, func(tx domain.LibraryTx) error {
		if err := tx.UpdateGame(ctx, updated, now); err != nil {
			return err
		}
		for _, it := range p.Items {
			if err := tx.PlaceItem(ctx, it.Item.ID, g.ID, it.Files); err != nil {
				return err
			}
		}
		return nil
	})
	return RematchResult{GameID: g.ID, Path: p.Console + "/" + p.Folder}, err
}

// merge moves the items into the target game; the emptied game is deleted
// and its trash entries now belong to the target.
func (s *LibraryService) merge(ctx context.Context, p *RematchPlan) (RematchResult, error) {
	g, target := p.game, p.target
	b := s.newOp("rematch")
	oldDir := s.files.LibraryPath(p.Console, g.Folder)
	targetDir := s.files.LibraryPath(p.Console, target.Folder)
	b.mkdir(targetDir)

	type trashed struct {
		dir   string
		items []domain.ItemID
	}
	var replaced, skipped []trashed
	for _, it := range p.Items {
		switch it.Action {
		case ActionReplace:
			dir, err := b.trash(targetDir, *it.Duplicate)
			if err != nil {
				return RematchResult{}, err
			}
			replaced = append(replaced, trashed{dir, []domain.ItemID{it.Duplicate.ID}})
		case ActionSkip:
			dir, err := b.trash(oldDir, it.Item)
			if err != nil {
				return RematchResult{}, err
			}
			skipped = append(skipped, trashed{dir, []domain.ItemID{it.Item.ID}})
		}
	}
	for _, it := range p.Items {
		if it.Action == ActionStore || it.Action == ActionReplace {
			if err := b.place(it.Item.Shape, oldDir, it.Item.Files, targetDir, it.Files, false); err != nil {
				return RematchResult{}, err
			}
		}
	}
	b.prune = append(b.prune, oldDir)

	updated := *target // keep its title and folder; refresh what IGDB says
	updated.ReleaseYear, updated.CoverImageID, updated.Summary, updated.Genres = p.info.ReleaseYear, p.info.CoverImageID, p.info.Summary, p.info.Genres
	now := s.now()
	err := s.run(ctx, b, func(tx domain.LibraryTx) error {
		if err := tx.UpdateGame(ctx, updated, now); err != nil {
			return err
		}
		for _, t := range replaced {
			if err := trashItems(ctx, tx, target.ID, domain.TrashReplaced, false, t.dir, now, t.items...); err != nil {
				return err
			}
		}
		for _, t := range skipped {
			if err := trashItems(ctx, tx, target.ID, domain.TrashReplaced, false, t.dir, now, t.items...); err != nil {
				return err
			}
		}
		for _, it := range p.Items {
			if it.Action == ActionStore || it.Action == ActionReplace {
				if err := tx.PlaceItem(ctx, it.Item.ID, target.ID, it.Files); err != nil {
					return err
				}
			}
		}
		if err := tx.MoveGameContents(ctx, g.ID, target.ID); err != nil {
			return err
		}
		return tx.DeleteGame(ctx, g.ID)
	})
	return RematchResult{GameID: target.ID, Path: p.Console + "/" + target.Folder, Merged: true}, err
}
