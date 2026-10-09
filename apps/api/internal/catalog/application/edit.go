package application

import (
	"context"
	"errors"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// EditRequest renames a game and/or moves it to another console (RF-24).
type EditRequest struct {
	GameID  domain.GameID
	Console string
	Name    GameName
	// Decisions are needed when the game merges into another one with the
	// same name and a file collides there: Replace sends the other game's
	// file to the trash, Skip sends this game's file to the trash.
	Decisions map[domain.ItemID]domain.DuplicateAction
}

// EditItem is what happens to one file of the game.
type EditItem struct {
	Item      domain.GameItem
	File      string // new name
	Duplicate *domain.GameItem
	Action    Action
}

// EditPlan previews an edit.
type EditPlan struct {
	Console string
	Title   string
	Folder  string
	// MergeInto is the game that already has the name on that console (0 if none).
	MergeInto domain.GameID
	Items     []EditItem

	game    *domain.Game
	target  *domain.Game
	updated domain.Game
}

// EditResult says where the game ended up.
type EditResult struct {
	GameID domain.GameID
	Path   string
	Merged bool
}

// PlanEdit previews an edit without touching anything.
func (s *LibraryService) PlanEdit(ctx context.Context, req EditRequest) (*EditPlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planEdit(ctx, req)
}

func (s *LibraryService) planEdit(ctx context.Context, req EditRequest) (*EditPlan, error) {
	g, err := s.repo.GameByID(ctx, req.GameID)
	if err != nil {
		return nil, err
	}
	items := g.LiveItems()
	if len(items) == 0 {
		return nil, domain.ErrGameNotFound
	}
	console, err := s.console(ctx, domain.Slug(req.Console))
	if err != nil {
		return nil, err
	}
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, it := range items {
		ext := extensionOf(it.File, consoles)
		if !console.Accepts(ext) || !console.Allows(it.Kind) {
			return nil, reject(RejectInvalid, "%s no acepta %q: el juego no puede cambiar de consola.", console.DisplayName, it.File)
		}
	}

	named, err := s.renamed(ctx, g, console.Slug, req.Name)
	if err != nil {
		return nil, err
	}
	p := &EditPlan{Console: string(console.Slug), game: g}
	var existing []domain.GameItem
	target, err := s.repo.GameByFolder(ctx, console.Slug, named.Folder)
	switch {
	case err == nil && target.ID != g.ID:
		p.target, p.MergeInto = target, target.ID
		p.updated = joinGame(*target, named)
		existing = target.LiveItems()
	case err == nil, errors.Is(err, domain.ErrGameNotFound):
		p.updated = named
		p.updated.ID, p.updated.CreatedAt = g.ID, g.CreatedAt
	default:
		return nil, err
	}
	p.Title, p.Folder = p.updated.Title, p.updated.Folder

	names := nameSet{}
	replaced := map[domain.ItemID]bool{}
	for _, it := range items {
		name, err := fileName(p.Title, it.Kind, it.Label, extensionOf(it.File, consoles), it.File)
		if err != nil {
			return nil, err
		}
		ei := EditItem{Item: it, File: name, Action: ActionStore}
		if ei.Duplicate = domain.FindDuplicate(existing, name); ei.Duplicate != nil {
			ei.Action = decide(req.Decisions[it.ID])
			if ei.Action == ActionReplace {
				if replaced[ei.Duplicate.ID] {
					return nil, reject(RejectInvalid, "Dos archivos reemplazarían a %q.", ei.Duplicate.File)
				}
				replaced[ei.Duplicate.ID] = true
			}
		}
		if ei.Action != ActionSkip {
			if err := names.add(it.File, name); err != nil {
				return nil, err
			}
		}
		p.Items = append(p.Items, ei)
	}
	return p, nil
}

// renamed describes the game under its new name. Keeping the IGDB game it
// is linked to reuses the stored metadata (no request to IGDB); a name of
// the user's own unlinks it.
func (s *LibraryService) renamed(ctx context.Context, g *domain.Game, console domain.Slug, n GameName) (domain.Game, error) {
	if n.IGDBID != nil && g.IGDBID != nil && *n.IGDBID == *g.IGDBID {
		out := *g
		out.Console, out.Items = console, nil
		folder, err := domain.GameFolder(out.Title)
		if err != nil {
			return out, nameRejection(err)
		}
		out.Folder = folder
		return out, nil
	}
	return s.describe(ctx, console, n)
}

// Edit renames the game's folder and files for its new name and console, or
// merges it into the game that already has them. Reversible like a commit.
func (s *LibraryService) Edit(ctx context.Context, req EditRequest) (EditResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.planEdit(ctx, req)
	if err != nil {
		return EditResult{}, err
	}
	for _, it := range p.Items {
		if it.Action == ActionUndecided {
			return EditResult{}, reject(RejectConflict, "%q ya existe en %q: elige Reemplazar u Omitir.", it.File, p.Folder)
		}
	}
	if p.target != nil {
		return s.merge(ctx, p)
	}
	return s.rename(ctx, p)
}

// rename moves the folder (if its name or console changes) and renames the
// files inside.
func (s *LibraryService) rename(ctx context.Context, p *EditPlan) (EditResult, error) {
	g := p.game
	b := s.newOp("edit")
	oldDir := s.gameDir(g)
	newDir := s.files.LibraryPath(p.Console, p.Folder)
	exists, err := s.files.Exists(oldDir)
	if err != nil {
		return EditResult{}, err
	}
	if exists {
		b.mkdir(s.files.LibraryPath(p.Console))
		b.move(oldDir, newDir)
	} else {
		b.mkdir(newDir)
	}
	for _, it := range p.Items {
		if err := b.placeAfter(oldDir, newDir, it.Item.File, newDir, it.File, false); err != nil {
			return EditResult{}, err
		}
	}
	if p.Console != string(g.Console) {
		b.prune = append(b.prune, s.files.LibraryPath(string(g.Console)))
	}
	now := s.now()
	err = s.run(ctx, b, func(tx domain.LibraryTx) error {
		if err := tx.UpdateGame(ctx, p.updated, now); err != nil {
			return err
		}
		for _, it := range p.Items {
			moved := it.Item
			moved.File = it.File
			if err := tx.UpdateItem(ctx, moved); err != nil {
				return err
			}
		}
		return nil
	})
	return EditResult{GameID: g.ID, Path: p.Console + "/" + p.Folder}, err
}

// merge moves the files into the target game; the emptied game is deleted
// and its trash entries now belong to the target.
func (s *LibraryService) merge(ctx context.Context, p *EditPlan) (EditResult, error) {
	g, target := p.game, p.target
	b := s.newOp("edit")
	oldDir := s.gameDir(g)
	targetDir := s.gameDir(target)
	b.mkdir(targetDir)

	type trashed struct {
		dir  string
		item domain.ItemID
	}
	var toTrash []trashed
	for _, it := range p.Items {
		switch it.Action {
		case ActionReplace:
			dir, err := b.trash(targetDir, *it.Duplicate)
			if err != nil {
				return EditResult{}, err
			}
			toTrash = append(toTrash, trashed{dir, it.Duplicate.ID})
		case ActionSkip:
			dir, err := b.trash(oldDir, it.Item)
			if err != nil {
				return EditResult{}, err
			}
			toTrash = append(toTrash, trashed{dir, it.Item.ID})
		}
	}
	for _, it := range p.Items {
		if it.Action == ActionStore || it.Action == ActionReplace {
			if err := b.place(oldDir, it.Item.File, targetDir, it.File, false); err != nil {
				return EditResult{}, err
			}
		}
	}
	b.pruneGame(g.Console, g.Folder)

	now := s.now()
	err := s.run(ctx, b, func(tx domain.LibraryTx) error {
		if err := tx.UpdateGame(ctx, p.updated, now); err != nil {
			return err
		}
		for _, t := range toTrash {
			if err := trashItems(ctx, tx, target.ID, domain.TrashReplaced, false, t.dir, now, t.item); err != nil {
				return err
			}
		}
		for _, it := range p.Items {
			if it.Action == ActionStore || it.Action == ActionReplace {
				moved := it.Item
				moved.GameID, moved.File = target.ID, it.File
				if err := tx.UpdateItem(ctx, moved); err != nil {
					return err
				}
			}
		}
		if err := tx.MoveGameContents(ctx, g.ID, target.ID); err != nil {
			return err
		}
		return tx.DeleteGame(ctx, g.ID)
	})
	return EditResult{GameID: target.ID, Path: p.Console + "/" + target.Folder, Merged: true}, err
}

// ItemEditRequest changes a file's kind and label (RF-24).
type ItemEditRequest struct {
	ItemID      domain.ItemID
	Kind        domain.ItemKind
	Label       string
	OnDuplicate domain.DuplicateAction
}

// EditFile renames one file for its new kind and label. When another file
// of the game has that name, OnDuplicate must be Replace: the other one
// goes to the trash.
func (s *LibraryService) EditFile(ctx context.Context, req ItemEditRequest) (domain.GameID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, err := s.repo.ItemByID(ctx, req.ItemID)
	if err != nil {
		return 0, err
	}
	if !it.Live() {
		return 0, domain.ErrItemNotFound
	}
	g, err := s.repo.GameByID(ctx, it.GameID)
	if err != nil {
		return 0, err
	}
	console, err := s.console(ctx, g.Console)
	if err != nil {
		return 0, err
	}
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return 0, err
	}
	kind, label, err := itemFor(console, req.Kind, req.Label, it.File)
	if err != nil {
		return 0, err
	}
	name, err := fileName(g.Title, kind, label, extensionOf(it.File, consoles), it.File)
	if err != nil {
		return 0, err
	}
	var others []domain.GameItem
	for _, other := range g.LiveItems() {
		if other.ID != it.ID {
			others = append(others, other)
		}
	}
	dup := domain.FindDuplicate(others, name)
	if dup != nil && req.OnDuplicate != domain.Replace {
		return 0, reject(RejectConflict, "Ya existe %q: elige Reemplazar (el actual va a la papelera) o cancela.", dup.File)
	}

	b := s.newOp("edit-file")
	dir := s.gameDir(g)
	var trashDir string
	if dup != nil {
		if trashDir, err = b.trash(dir, *dup); err != nil {
			return 0, err
		}
	}
	if err := b.place(dir, it.File, dir, name, false); err != nil {
		return 0, err
	}
	updated := it
	updated.Kind, updated.Label, updated.File = kind, label, name
	now := s.now()
	err = s.run(ctx, b, func(tx domain.LibraryTx) error {
		if dup != nil {
			if err := trashItems(ctx, tx, g.ID, domain.TrashReplaced, false, trashDir, now, dup.ID); err != nil {
				return err
			}
		}
		return tx.UpdateItem(ctx, updated)
	})
	return g.ID, err
}
