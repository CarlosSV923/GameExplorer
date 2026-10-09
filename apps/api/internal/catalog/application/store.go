package application

import (
	"context"
	"errors"
	"path"
	"slices"

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
	// Renumber gives files already in the game a disc number (RF-08a): the
	// game of one disc gets more.
	Renumber []Renumbering
}

// Renumbering is the disc number for a file already in the game.
type Renumbering struct {
	Item  domain.ItemID
	Label string
}

// RenamedItem is a file already in the game that the commit renames.
type RenamedItem struct {
	Item domain.GameItem
	File string
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
	// Renamed are files already in the game that become numbered discs.
	Renamed []RenamedItem

	game  domain.Game
	items []domain.GameItem // per Files: kind, label and size, clean
	// renamed is the game's files with the renumbering applied.
	renamed []domain.GameItem
	console domain.Console
	files   []NewFile
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
	// Renumbered files keep their place in the duplicate check under their new name.
	current := slices.Clone(existing)
	for _, rn := range req.Renumber {
		i := slices.IndexFunc(current, func(it domain.GameItem) bool { return it.ID == rn.Item })
		if i < 0 || !console.Allows(domain.KindDisc) {
			return nil, reject(RejectInvalid, "Solo los discos de un juego guardado se pueden numerar.")
		}
		label, err := domain.CleanLabel(domain.KindDisc, rn.Label)
		if err != nil {
			return nil, itemRejection(current[i].File, err)
		}
		ext := domain.ExtensionOf(current[i].File, KnownExtensions(consoles))
		name, err := fileName(game.Title, domain.KindDisc, label, ext, current[i].File)
		if err != nil {
			return nil, err
		}
		p.Renamed = append(p.Renamed, RenamedItem{Item: current[i], File: name})
		current[i].Kind, current[i].Label, current[i].File = domain.KindDisc, label, name
	}
	names := nameSet{}
	p.renamed = current
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
		if planned.Duplicate = domain.FindDuplicate(current, name); planned.Duplicate != nil {
			planned.Action = decide(f.OnDuplicate)
			if planned.Action == ActionReplace {
				if slices.ContainsFunc(p.Renamed, func(r RenamedItem) bool { return r.Item.ID == planned.Duplicate.ID }) {
					return nil, reject(RejectInvalid, "%q reemplazaría al disco que se está numerando: dale otro número.", f.Ref)
				}
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
	p.console = console
	return p, nil
}

// checkDiscs keeps a game of a console with discs coherent (spec §5): a
// game of several files has every one numbered.
func checkDiscs(console domain.Console, p *Plan) error {
	if !console.Allows(domain.KindDisc) {
		return nil
	}
	var final []domain.GameItem
	for _, it := range p.renamed {
		gone := slices.ContainsFunc(p.Files, func(f PlannedFile) bool {
			return f.Action == ActionReplace && f.Duplicate != nil && f.Duplicate.ID == it.ID
		})
		if !gone {
			final = append(final, it)
		}
	}
	for i, f := range p.Files {
		if f.Action == ActionStore || f.Action == ActionReplace {
			final = append(final, p.items[i])
		}
	}
	if len(final) < 2 {
		return nil
	}
	for _, it := range final {
		if it.Kind != domain.KindDisc {
			return reject(RejectInvalid, "Un juego de varios discos necesita el número de cada disco, también del que ya está guardado (%s).", it.File)
		}
	}
	return nil
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
	// The preview shows the game's files so the user can number them; storing
	// needs every disc numbered.
	if err := checkDiscs(p.console, p); err != nil {
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
	// The game's own disc becomes "(Disc N)" before the new ones arrive (RF-08a).
	for _, r := range p.Renamed {
		if err := b.place(gameDir, r.Item.File, gameDir, r.File, true); err != nil {
			return res, err
		}
	}
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
		for _, it := range p.renamed {
			if slices.ContainsFunc(p.Renamed, func(r RenamedItem) bool { return r.Item.ID == it.ID }) {
				if err := tx.UpdateItem(ctx, it); err != nil {
					return err
				}
			}
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
