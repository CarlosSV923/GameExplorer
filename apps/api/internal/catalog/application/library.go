package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// GameInfo is what the catalog needs to know about an IGDB game.
type GameInfo struct {
	IGDBID       int64
	Name         string
	ReleaseYear  *int
	CoverImageID *string
	Summary      *string
	Genres       []string
}

// Errors a GameDirectory returns.
var (
	ErrUnknownGame           = errors.New("game does not exist in IGDB")
	ErrMetadataNotConfigured = errors.New("IGDB is not configured")
	ErrMetadataUnavailable   = errors.New("IGDB failed or could not be reached")
)

// GameDirectory looks up games by IGDB id (the catalog's port to metadata).
type GameDirectory interface {
	GameByID(ctx context.Context, id int64) (GameInfo, error)
}

// Files is the library's file system. Every path is absolute and must lie
// inside the library (staging and trash live there too, so moves are renames).
type Files interface {
	// LibraryPath joins elem under the library root.
	LibraryPath(elem ...string) string
	// TrashPath joins elem under the trash directory.
	TrashPath(elem ...string) string
	Exists(path string) (bool, error)
	// MkdirAll creates dir and returns the directories it created, outermost first.
	MkdirAll(dir string) ([]string, error)
	// Move renames from to to; it fails if to exists.
	Move(from, to string) error
	// RemoveEmptyDir deletes dir only if it is empty.
	RemoveEmptyDir(dir string) error
	ReadFile(path string, limit int64) ([]byte, error)
	WriteFile(path string, data []byte) error
	Remove(path string) error
}

// RejectReason classifies why a request was refused.
type RejectReason string

// Reject reasons.
const (
	RejectInvalid     RejectReason = "invalid"     // the request is wrong
	RejectConflict    RejectReason = "conflict"    // undecided duplicates, files in the way
	RejectUnavailable RejectReason = "unavailable" // IGDB not configured
	RejectUpstream    RejectReason = "upstream"    // IGDB failed
)

// Rejection is a refused request; Message is shown to the user.
type Rejection struct {
	Reason  RejectReason
	Message string
}

func (r *Rejection) Error() string { return string(r.Reason) + ": " + r.Message }

func reject(reason RejectReason, format string, args ...any) *Rejection {
	return &Rejection{Reason: reason, Message: fmt.Sprintf(format, args...)}
}

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

// Action is what will happen to a planned item.
type Action string

// Planned actions.
const (
	ActionStore   Action = "store"
	ActionReplace Action = "replace"
	ActionSkip    Action = "skip"
	// ActionUndecided means the item is a duplicate and needs a decision.
	ActionUndecided Action = "undecided"
)

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

// maxCueSize bounds the .cue sheets read for rewriting.
const maxCueSize = 1 << 20

// LibraryService adds uploads to the library (RF-09..RF-11).
type LibraryService struct {
	consoles domain.ConsoleRepository
	repo     domain.LibraryRepository
	games    GameDirectory
	files    Files
	log      *slog.Logger
	now      func() time.Time
	newID    func() string

	// mu serializes changes: two uploads must not race for a folder name.
	mu sync.Mutex
}

// NewLibraryService builds the service. now may be nil (time.Now).
func NewLibraryService(consoles domain.ConsoleRepository, repo domain.LibraryRepository, games GameDirectory, files Files, log *slog.Logger, now func() time.Time) *LibraryService {
	if now == nil {
		now = time.Now
	}
	return &LibraryService{consoles: consoles, repo: repo, games: games, files: files, log: log, now: now, newID: randomID}
}

func randomID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
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
	console, err := s.console(ctx, req.Console)
	if err != nil {
		return nil, err
	}
	info, err := s.games.GameByID(ctx, req.IGDBGameID)
	switch {
	case errors.Is(err, ErrUnknownGame):
		return nil, reject(RejectInvalid, "El juego %d no existe en IGDB.", req.IGDBGameID)
	case errors.Is(err, ErrMetadataNotConfigured):
		return nil, reject(RejectUnavailable, "IGDB no está configurado (IGDB_CLIENT_ID / IGDB_CLIENT_SECRET).")
	case errors.Is(err, ErrMetadataUnavailable):
		return nil, reject(RejectUpstream, "IGDB no responde; inténtalo de nuevo.")
	case err != nil:
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
		game.Folder, err = domain.GameFolder(info.Name, info.ReleaseYear, info.IGDBID, func(f string) (bool, error) {
			if taken, err := s.repo.FolderTaken(ctx, console.ID, f); err != nil || taken {
				return taken, err
			}
			return s.files.Exists(s.files.LibraryPath(string(console.Slug), f))
		})
		if err != nil {
			return nil, nameRejection(err)
		}
	default:
		return nil, err
	}

	p := &Plan{
		Console: string(console.Slug), Title: game.Title, Folder: game.Folder, GameID: game.ID,
		Existing: existing, game: game, items: req.Items,
	}
	names := map[string]string{}
	replaced := map[domain.ItemID]string{}
	for _, it := range req.Items {
		files, err := itemFiles(game.Title, it)
		if err != nil {
			return nil, err
		}
		planned := PlannedItem{Ref: it.Ref, Files: files, Action: ActionStore}
		planned.Duplicate = domain.FindDuplicate(existing, domain.Candidate{
			Kind: it.Kind, Label: it.Label, DiscNumber: it.DiscNumber, Files: files,
		})
		if planned.Duplicate != nil {
			switch it.OnDuplicate {
			case domain.Replace:
				planned.Action = ActionReplace
				if other, ok := replaced[planned.Duplicate.ID]; ok {
					return nil, reject(RejectInvalid, "%q y %q reemplazarían al mismo elemento.", other, it.Ref)
				}
				replaced[planned.Duplicate.ID] = it.Ref
			case domain.Skip:
				planned.Action = ActionSkip
			default:
				planned.Action = ActionUndecided
			}
		}
		if planned.Action != ActionSkip {
			for _, f := range files {
				key := strings.ToLower(f)
				if other, ok := names[key]; ok {
					return nil, reject(RejectInvalid, "%q y %q tendrían el mismo nombre: %s.", other, it.Ref, f)
				}
				names[key] = it.Ref
			}
		}
		p.Items = append(p.Items, planned)
	}
	return p, nil
}

func (s *LibraryService) console(ctx context.Context, slug string) (domain.Console, error) {
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return domain.Console{}, err
	}
	for _, c := range consoles {
		if string(c.Slug) == slug {
			return c, nil
		}
	}
	return domain.Console{}, reject(RejectInvalid, "La consola %q no existe.", slug)
}

// itemFiles names every part of an item.
func itemFiles(title string, it NewItem) ([]string, error) {
	name := domain.ItemName{Title: title, Kind: it.Kind, Label: it.Label, DiscNumber: it.DiscNumber}
	if !it.Kind.Valid() {
		return nil, reject(RejectInvalid, "%q: tipo %q desconocido.", it.Ref, it.Kind)
	}
	switch it.Shape {
	case domain.ShapeFile, domain.ShapeFolder:
		if len(it.Parts) != 1 {
			return nil, fmt.Errorf("item %q: %s with %d parts", it.Ref, it.Shape, len(it.Parts))
		}
		ext := ""
		if it.Shape == domain.ShapeFile {
			ext = path.Ext(it.Parts[0])
		}
		f, err := name.FileName(ext)
		if err != nil {
			return nil, itemRejection(it.Ref, err)
		}
		return []string{f}, nil
	case domain.ShapeDisc:
		if len(it.Parts) == 0 || !strings.EqualFold(path.Ext(it.Parts[0]), ".cue") {
			return nil, fmt.Errorf("item %q: disc without a .cue sheet", it.Ref)
		}
		stem, err := name.Stem()
		if err != nil {
			return nil, itemRejection(it.Ref, err)
		}
		cue, err := name.FileName(".cue")
		if err != nil {
			return nil, itemRejection(it.Ref, err)
		}
		exts := make([]string, 0, len(it.Parts)-1)
		for _, p := range it.Parts[1:] {
			exts = append(exts, path.Ext(p))
		}
		tracks, err := domain.TrackNames(stem, exts)
		if err != nil {
			return nil, itemRejection(it.Ref, err)
		}
		return append([]string{cue}, tracks...), nil
	}
	return nil, fmt.Errorf("item %q: unknown shape %q", it.Ref, it.Shape)
}

func itemRejection(ref string, err error) error {
	r := nameRejection(err)
	var rej *Rejection
	if errors.As(r, &rej) {
		rej.Message = fmt.Sprintf("%q: %s", ref, rej.Message)
	}
	return r
}

func nameRejection(err error) error {
	var ne *domain.NameError
	if !errors.As(err, &ne) {
		return err
	}
	switch {
	case errors.Is(err, domain.ErrNameTooLong):
		return reject(RejectInvalid, "El nombre resultante supera los 255 bytes.")
	case ne.Field == "title":
		return reject(RejectInvalid, "El título no deja un nombre de archivo válido.")
	case ne.Field == "label":
		return reject(RejectInvalid, "Falta la versión del update o el nombre del DLC.")
	case ne.Field == "disc":
		return reject(RejectInvalid, "El número de disco debe estar entre 1 y 99.")
	case ne.Field == "extension":
		return reject(RejectInvalid, "La extensión del archivo no es válida.")
	}
	return reject(RejectInvalid, "Tipo de elemento no válido.")
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

	op, changes, temps, err := s.prepare(p, req.Source)
	defer func() {
		for _, t := range temps {
			if err := s.files.Remove(t); err != nil {
				s.log.Warn("remove rewritten cue", "path", t, "error", err)
			}
		}
	}()
	if err != nil {
		return res, err
	}

	if err := s.repo.SaveOperation(ctx, op); err != nil {
		s.removeDirs(op.CreatedDirs)
		return res, fmt.Errorf("save journal: %w", err)
	}
	for _, m := range op.Moves {
		if err := s.files.Move(m.From, m.To); err != nil {
			return res, s.undoAfter(ctx, op, fmt.Errorf("move %s: %w", filepath.Base(m.From), err))
		}
	}
	gameID, err := s.repo.Apply(ctx, changes)
	if err != nil {
		return res, s.undoAfter(ctx, op, fmt.Errorf("record items: %w", err))
	}
	res.GameID = gameID
	return res, nil
}

// prepare builds the journal and the catalog changes, creates the folders and
// writes rewritten .cue sheets next to the originals (returned as temps).
func (s *LibraryService) prepare(p *Plan, source string) (op domain.Operation, changes domain.Changes, temps []string, err error) {
	now := s.now()
	op = domain.Operation{ID: s.newID(), Source: source, CreatedAt: now}
	changes = domain.Changes{OperationID: op.ID, Game: p.game, Now: now}
	gameDir := s.files.LibraryPath(p.Console, p.Folder)

	// Replaced items go to the trash first, so a new item may reuse a name.
	dirs := []string{gameDir}
	for _, it := range p.Items {
		if it.Action != ActionReplace {
			continue
		}
		trashDir := op.ID + "-" + strconv.FormatInt(int64(it.Duplicate.ID), 10)
		dirs = append(dirs, s.files.TrashPath(trashDir))
		changes.Trashed = append(changes.Trashed, domain.TrashedItem{ID: it.Duplicate.ID, TrashDir: trashDir})
		for _, f := range it.Duplicate.Files {
			from := filepath.Join(gameDir, f)
			ok, err := s.files.Exists(from)
			if err != nil {
				return op, changes, temps, err
			}
			if ok { // a file deleted over SMB has nothing to move
				op.Moves = append(op.Moves, domain.Move{From: from, To: s.files.TrashPath(trashDir, f)})
			}
		}
	}

	for i, it := range p.Items {
		if it.Action != ActionStore && it.Action != ActionReplace {
			continue
		}
		src := p.items[i]
		for j, part := range src.Parts {
			from := filepath.Join(src.Root, filepath.FromSlash(part))
			if src.Shape == domain.ShapeDisc && j == 0 {
				rewritten, err := s.rewriteCue(src, it.Files)
				if err != nil {
					return op, changes, temps, err
				}
				temps = append(temps, rewritten)
				from = rewritten
			}
			op.Moves = append(op.Moves, domain.Move{From: from, To: filepath.Join(gameDir, it.Files[j])})
		}
		changes.New = append(changes.New, domain.GameItem{
			Kind: src.Kind, Label: strings.TrimSpace(src.Label), DiscNumber: src.DiscNumber, Shape: src.Shape,
			Files: it.Files, Size: src.Size, TitleID: src.TitleID, SourceJob: source,
		})
		if src.Kind != domain.KindDisc {
			changes.New[len(changes.New)-1].DiscNumber = 0
		}
	}

	// Nothing may be in the way, except files that an earlier move frees.
	freed := map[string]bool{}
	for _, m := range op.Moves {
		freed[m.From] = true
		if freed[m.To] {
			continue
		}
		taken, err := s.files.Exists(m.To)
		if err != nil {
			return op, changes, temps, err
		}
		if taken {
			return op, changes, temps, reject(RejectConflict,
				"Ya hay un archivo %q en la carpeta del juego que la app no conoce; muévelo o bórralo por SMB.", filepath.Base(m.To))
		}
	}

	for _, d := range dirs {
		created, err := s.files.MkdirAll(d)
		op.CreatedDirs = append(op.CreatedDirs, created...)
		if err != nil {
			s.removeDirs(op.CreatedDirs)
			return op, changes, temps, fmt.Errorf("create folder: %w", err)
		}
	}
	return op, changes, temps, nil
}

// rewriteCue writes the item's .cue sheet with FILE lines pointing at the
// new track names, next to the original, and returns its path.
func (s *LibraryService) rewriteCue(it NewItem, files []string) (string, error) {
	cuePath := filepath.Join(it.Root, filepath.FromSlash(it.Parts[0]))
	data, err := s.files.ReadFile(cuePath, maxCueSize)
	if err != nil {
		return "", fmt.Errorf("read cue: %w", err)
	}
	tracks := map[string]string{}
	for i, part := range it.Parts[1:] {
		tracks[strings.ToLower(part)] = files[i+1]
	}
	dir := path.Dir(it.Parts[0])
	sheet := domain.RewriteCue(string(data), func(ref string) (string, bool) {
		name, ok := tracks[strings.ToLower(path.Join(dir, strings.ReplaceAll(ref, `\`, "/")))]
		return name, ok
	})
	out := cuePath + ".gameexplorer-commit"
	if err := s.files.WriteFile(out, []byte(sheet)); err != nil {
		return "", fmt.Errorf("write cue: %w", err)
	}
	return out, nil
}

// ErrUndoFailed means a failed change could not be fully reverted: some
// files may be in the library without being recorded. The journal is kept and
// the next start retries.
var ErrUndoFailed = errors.New("library change could not be undone")

func (s *LibraryService) undoAfter(ctx context.Context, op domain.Operation, cause error) error {
	if !s.undo(ctx, op) {
		return fmt.Errorf("%w: %w", ErrUndoFailed, cause)
	}
	return cause
}

// undo reverts an operation's moves (newest first) and removes the folders
// it created. A move is reverted only when its destination exists and its
// source does not, so undo is safe to repeat after a crash at any point.
func (s *LibraryService) undo(ctx context.Context, op domain.Operation) bool {
	failed := false
	for i := len(op.Moves) - 1; i >= 0; i-- {
		m := op.Moves[i]
		fromExists, err1 := s.files.Exists(m.From)
		toExists, err2 := s.files.Exists(m.To)
		if err := errors.Join(err1, err2); err != nil {
			s.log.Error("undo move: stat", "from", m.From, "to", m.To, "error", err)
			failed = true
			continue
		}
		if fromExists || !toExists {
			continue
		}
		if err := s.files.Move(m.To, m.From); err != nil {
			s.log.Error("undo move", "from", m.To, "to", m.From, "error", err)
			failed = true
		}
	}
	s.removeDirs(op.CreatedDirs)
	if failed {
		return false // keep the journal so the next start tries again
	}
	if err := s.repo.DeleteOperation(ctx, op.ID); err != nil {
		s.log.Warn("delete journal", "operation", op.ID, "error", err)
	}
	return true
}

func (s *LibraryService) removeDirs(dirs []string) {
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := s.files.RemoveEmptyDir(dirs[i]); err != nil {
			s.log.Warn("remove created folder", "dir", dirs[i], "error", err)
		}
	}
}

// Recover undoes the operations interrupted by a restart. Call it before
// anything else touches the library.
func (s *LibraryService) Recover(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ops, err := s.repo.Operations(ctx)
	if err != nil {
		return 0, err
	}
	for _, op := range ops {
		s.log.Warn("undoing interrupted library change", "operation", op.ID, "source", op.Source, "moves", len(op.Moves))
		s.undo(ctx, op)
	}
	return len(ops), nil
}

// HasItemsFrom reports whether an upload's items are in the library.
func (s *LibraryService) HasItemsFrom(ctx context.Context, source string) (bool, error) {
	return s.repo.HasItemsFrom(ctx, source)
}
