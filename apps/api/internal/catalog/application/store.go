package application

import (
	"context"
	"errors"
	"strings"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// NewItem is an item to add to the library.
type NewItem struct {
	// Ref identifies the item for the caller (echoed in the plan).
	Ref   string
	Shape domain.Shape
	// Root is the absolute directory that Parts are relative to.
	Root string
	// Parts use "/" separators; for discs the .cue sheet comes first, then
	// its tracks in sheet order.
	Parts       []string
	Size        int64
	Kind        domain.ItemKind
	Label       string
	DiscNumber  int
	TitleID     string
	OnDuplicate domain.DuplicateAction
}

// StoreRequest adds the items of one upload to one game.
type StoreRequest struct {
	Source     string
	Console    string
	IGDBGameID int64
	Items      []NewItem
}

// PlannedItem is the outcome for one new item.
type PlannedItem struct {
	Ref       string
	Files     []string // relative to the game folder
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
	Items    []PlannedItem

	game  domain.Game
	items []NewItem
}

// StoreResult summarizes a stored request.
type StoreResult struct {
	GameID domain.GameID
	// Path is "<console>/<folder>".
	Path                      string
	Stored, Replaced, Skipped int
}

// Plan computes names, folder and duplicates for a request.
func (s *LibraryService) Plan(ctx context.Context, req StoreRequest) (*Plan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plan(ctx, req)
}

func (s *LibraryService) plan(ctx context.Context, req StoreRequest) (*Plan, error) {
	if len(req.Items) == 0 {
		return nil, reject(RejectInvalid, "No hay elementos para guardar.")
	}
	console, err := s.consoleBySlug(ctx, req.Console)
	if err != nil {
		return nil, err
	}
	info, err := s.gameInfo(ctx, req.IGDBGameID)
	if err != nil {
		return nil, err
	}

	game := domain.Game{
		ConsoleID: console.ID, IGDBID: info.IGDBID, Title: info.Name, ReleaseYear: info.ReleaseYear,
		CoverImageID: info.CoverImageID, Summary: info.Summary, Genres: info.Genres,
	}
	var existing []domain.GameItem
	found, err := s.repo.FindGame(ctx, console.ID, req.IGDBGameID)
	switch {
	case err == nil:
		// Keep the stored title and folder so new files match the old ones.
		game.ID, game.Title, game.Folder = found.ID, found.Title, found.Folder
		existing = found.LiveItems()
	case errors.Is(err, domain.ErrGameNotFound):
		if game.Folder, err = s.newFolder(ctx, console, info, nil); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	p := &Plan{
		Console: string(console.Slug), Title: game.Title, Folder: game.Folder, GameID: game.ID,
		Existing: existing, game: game, items: req.Items,
	}
	names := nameSet{}
	replaced := map[domain.ItemID]string{}
	for _, it := range req.Items {
		n := domain.ItemName{Kind: it.Kind, Label: it.Label, DiscNumber: it.DiscNumber}
		files, err := itemNames(game.Title, it.Ref, it.Shape, n, it.Parts)
		if err != nil {
			return nil, err
		}
		planned := PlannedItem{Ref: it.Ref, Files: files, Action: ActionStore}
		planned.Duplicate = domain.FindDuplicate(existing, domain.Candidate{
			Kind: it.Kind, Label: it.Label, DiscNumber: it.DiscNumber, Files: files,
		})
		if planned.Duplicate != nil {
			planned.Action = decide(it.OnDuplicate)
			if planned.Action == ActionReplace {
				if other, ok := replaced[planned.Duplicate.ID]; ok {
					return nil, reject(RejectInvalid, "%q y %q reemplazarían al mismo elemento.", other, it.Ref)
				}
				replaced[planned.Duplicate.ID] = it.Ref
			}
		}
		if planned.Action != ActionSkip {
			if err := names.add(it.Ref, files); err != nil {
				return nil, err
			}
		}
		p.Items = append(p.Items, planned)
	}
	return p, nil
}

func decide(a domain.DuplicateAction) Action {
	switch a {
	case domain.Replace:
		return ActionReplace
	case domain.Skip:
		return ActionSkip
	}
	return ActionUndecided
}

// Store moves the items into the library and records them. If anything
// fails, every move is undone and the staged files are back where they were.
func (s *LibraryService) Store(ctx context.Context, req StoreRequest) (StoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p, err := s.plan(ctx, req)
	if err != nil {
		return StoreResult{}, err
	}
	res := StoreResult{GameID: p.GameID, Path: p.Console + "/" + p.Folder}
	for _, it := range p.Items {
		switch it.Action {
		case ActionUndecided:
			return res, reject(RejectConflict, "%q ya existe en la biblioteca: elige Reemplazar u Omitir.", it.Ref)
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
		return res, nil // every item was a skipped duplicate
	}

	b := s.newOp(req.Source)
	gameDir := s.files.LibraryPath(p.Console, p.Folder)
	b.mkdir(gameDir)
	type replacement struct {
		item domain.ItemID
		dir  string
	}
	var replacements []replacement
	for _, it := range p.Items {
		if it.Action == ActionReplace {
			dir, err := b.trash(gameDir, *it.Duplicate)
			if err != nil {
				return res, err
			}
			replacements = append(replacements, replacement{it.Duplicate.ID, dir})
		}
	}
	var stored []domain.GameItem
	for i, it := range p.Items {
		if it.Action != ActionStore && it.Action != ActionReplace {
			continue
		}
		src := p.items[i]
		if err := b.place(src.Shape, src.Root, src.Parts, gameDir, it.Files, true); err != nil {
			return res, err
		}
		item := domain.GameItem{
			Kind: src.Kind, Label: strings.TrimSpace(src.Label), Shape: src.Shape,
			Files: it.Files, Size: src.Size, TitleID: src.TitleID, SourceJob: req.Source,
		}
		if src.Kind == domain.KindDisc {
			item.DiscNumber = src.DiscNumber
		}
		stored = append(stored, item)
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
		res.GameID = game.ID
		return nil
	})
	return res, err
}
