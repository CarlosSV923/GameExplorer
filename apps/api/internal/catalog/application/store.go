package application

import (
	"context"
	"errors"
	"path"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// NewFile is a file to add to the library.
type NewFile struct {
	// Ref identifies the file for the caller (echoed in the plan).
	Ref string
	// Root is the absolute directory Path is relative to ("/" separators).
	Root        string
	Path        string
	Size        int64
	Kind        domain.ItemKind
	Label       string
	OnDuplicate domain.DuplicateAction
	// Unassigned is set when the file comes straight from the unassigned
	// section (Root is its folder): it leaves the section when stored (RF-27a).
	Unassigned domain.UnassignedID
}

// StoreRequest adds the files of one upload to one game.
type StoreRequest struct {
	Source  string
	Console string
	Name    GameName
	Files   []NewFile
}

// PlannedFile is the outcome for one new file.
type PlannedFile struct {
	Ref       string
	File      string // name inside the game folder
	Duplicate *domain.GameItem
	Action    Action
}

// Plan previews a StoreRequest without touching anything.
type Plan struct {
	Console string
	Title   string
	// Folder is the game's folder inside the console folder.
	Folder   string
	GameID   domain.GameID // 0 for a new game
	Existing []domain.GameItem
	Files    []PlannedFile

	game  domain.Game
	items []domain.GameItem // per Files: kind, label and size, clean
	files []NewFile
}

// StoreResult summarizes a stored request.
type StoreResult struct {
	GameID domain.GameID
	// Path is "<console>/<folder>".
	Path                      string
	Stored, Replaced, Skipped int
}

// Plan computes names, folder and duplicates for a request (RF-08, RF-09).
func (s *LibraryService) Plan(ctx context.Context, req StoreRequest) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plan(ctx, req)
}

func (s *LibraryService) plan(ctx context.Context, req StoreRequest) (*Plan, error) {
	if len(req.Files) == 0 {
		return nil, reject(RejectInvalid, "No hay archivos para guardar.")
	}
	console, err := s.console(ctx, domain.Slug(req.Console))
	if err != nil {
		return nil, err
	}
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return nil, err
	}
	game, err := s.describe(ctx, console.Slug, req.Name)
	if err != nil {
		return nil, err
	}
	var existing []domain.GameItem
	found, err := s.repo.GameByFolder(ctx, console.Slug, game.Folder)
	switch {
	case err == nil:
		existing = found.LiveItems()
		game = joinGame(*found, game)
	case !errors.Is(err, domain.ErrGameNotFound):
		return nil, err
	}

	p := &Plan{
		Console: string(console.Slug), Title: game.Title, Folder: game.Folder, GameID: game.ID,
		Existing: existing, game: game, files: req.Files,
	}
	names := nameSet{}
	replaced := map[domain.ItemID]string{}
	for _, f := range req.Files {
		ext := domain.ExtensionOf(path.Base(f.Path), KnownExtensions(consoles))
		if !console.Accepts(ext) {
			return nil, reject(RejectInvalid, "%q no es un archivo de %s.", f.Ref, console.DisplayName)
		}
		kind, label, err := itemFor(console, f.Kind, f.Label, f.Ref)
		if err != nil {
			return nil, err
		}
		name, err := fileName(game.Title, kind, label, ext, f.Ref)
		if err != nil {
			return nil, err
		}
		planned := PlannedFile{Ref: f.Ref, File: name, Action: ActionStore}
		if planned.Duplicate = domain.FindDuplicate(existing, name); planned.Duplicate != nil {
			planned.Action = decide(f.OnDuplicate)
			if planned.Action == ActionReplace {
				if other, ok := replaced[planned.Duplicate.ID]; ok {
					return nil, reject(RejectInvalid, "%q y %q reemplazarían al mismo archivo.", other, f.Ref)
				}
				replaced[planned.Duplicate.ID] = f.Ref
			}
		}
		if planned.Action != ActionSkip {
			if err := names.add(f.Ref, name); err != nil {
				return nil, err
			}
		}
		p.Files = append(p.Files, planned)
		p.items = append(p.items, domain.GameItem{Kind: kind, Label: label, File: name, Size: f.Size, SourceJob: req.Source})
	}
	return p, nil
}

// joinGame is the game a new name lands in when the console already has a
// game with that folder: the stored one keeps its title and folder (its
// files are named after them) and gains the IGDB link if it had none.
func joinGame(stored, named domain.Game) domain.Game {
	g := stored
	if g.IGDBID == nil && named.IGDBID != nil {
		g.IGDBID = named.IGDBID
		g.ReleaseYear, g.CoverImageID, g.Summary, g.Genres = named.ReleaseYear, named.CoverImageID, named.Summary, named.Genres
	}
	return g
}

// Store moves the files into the library and records them. If anything
// fails, every move is undone and the staged files are back where they were.
func (s *LibraryService) Store(ctx context.Context, req StoreRequest) (StoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.plan(ctx, req)
	if err != nil {
		return StoreResult{}, err
	}
	res := StoreResult{GameID: p.GameID, Path: p.Console + "/" + p.Folder}
	for _, f := range p.Files {
		switch f.Action {
		case ActionUndecided:
			return res, reject(RejectConflict, "%q ya existe en la biblioteca: elige Reemplazar u Omitir.", f.Ref)
		case ActionSkip:
			res.Skipped++
		case ActionReplace:
			res.Replaced++
			res.Stored++
		case ActionStore:
			res.Stored++
		}
	}
	if res.Stored == 0 {
		return res, nil // every file was a skipped duplicate
	}

	b := s.newOp(req.Source)
	gameDir := s.files.LibraryPath(p.Console, p.Folder)
	b.mkdir(gameDir)
	type replacement struct {
		item domain.ItemID
		dir  string
	}
	var replacements []replacement
	var stored []domain.GameItem
	var taken []domain.UnassignedID
	for i, f := range p.Files {
		if f.Action != ActionStore && f.Action != ActionReplace {
			continue
		}
		if f.Action == ActionReplace {
			dir, err := b.trash(gameDir, *f.Duplicate)
			if err != nil {
				return res, err
			}
			replacements = append(replacements, replacement{f.Duplicate.ID, dir})
		}
		src := p.files[i]
		if err := b.place(src.Root, src.Path, gameDir, f.File, true); err != nil {
			return res, err
		}
		stored = append(stored, p.items[i])
		if src.Unassigned != 0 {
			taken = append(taken, src.Unassigned)
			b.pruneParents(src.Root, src.Path)
		}
	}

	now := s.now()
	err = s.run(ctx, b, func(tx domain.LibraryTx) error {
		game := p.game
		if game.ID == 0 {
			id, err := tx.InsertGame(ctx, game, now)
			if err != nil {
				return err
			}
			game.ID = id
		} else if err := tx.UpdateGame(ctx, game, now); err != nil {
			return err
		}
		for _, r := range replacements {
			if err := trashItems(ctx, tx, game.ID, domain.TrashReplaced, false, r.dir, now, r.item); err != nil {
				return err
			}
		}
		for _, it := range stored {
			it.GameID = game.ID
			if _, err := tx.InsertItem(ctx, it, now); err != nil {
				return err
			}
		}
		for _, id := range taken {
			if err := tx.DeleteUnassigned(ctx, id); err != nil {
				return err
			}
		}
		res.GameID = game.ID
		return nil
	})
	return res, err
}
