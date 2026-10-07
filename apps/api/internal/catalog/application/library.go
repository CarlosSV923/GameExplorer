package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
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
// inside the library (staging, trash and scratch live there too, so moves
// are renames).
type Files interface {
	// LibraryPath joins elem under the library root.
	LibraryPath(elem ...string) string
	// TrashPath joins elem under the trash directory.
	TrashPath(elem ...string) string
	// ScratchPath joins elem under the directory for operations' scratch files.
	ScratchPath(elem ...string) string
	Exists(path string) (bool, error)
	// MkdirAll creates dir and returns the directories it created, outermost first.
	MkdirAll(dir string) ([]string, error)
	// Move renames from to to; it fails if to exists.
	Move(from, to string) error
	// RemoveEmptyDir deletes dir only if it is empty.
	RemoveEmptyDir(dir string) error
	// RemoveAll deletes path and everything below it.
	RemoveAll(path string) error
	// List returns the names in a directory (none if it does not exist).
	List(dir string) ([]string, error)
	ReadFile(path string, limit int64) ([]byte, error)
	WriteFile(path string, data []byte) error
	// Tree lists the regular files at path: the file itself, or every file
	// under a directory (sorted, Rel with "/" separators).
	Tree(path string) ([]TreeFile, error)
	Open(path string) (io.ReadSeekCloser, error)
}

// TreeFile is a regular file found by Files.Tree.
type TreeFile struct {
	Path     string // absolute
	Rel      string // relative to the listed directory; "" for a file
	Size     int64
	Modified time.Time
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

// Rejection is a refused request; nothing was changed. Message is shown to
// the user.
type Rejection struct {
	Reason  RejectReason
	Message string
}

func (r *Rejection) Error() string { return string(r.Reason) + ": " + r.Message }

func reject(reason RejectReason, format string, args ...any) *Rejection {
	return &Rejection{Reason: reason, Message: fmt.Sprintf(format, args...)}
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

// maxCueSize bounds the .cue sheets read for rewriting.
const maxCueSize = 1 << 20

// LibraryService changes the library: stores uploads, re-matches games and
// manages the trash and the integrity check (RF-09..RF-11, RF-24..RF-30).
// Every change goes through a journaled operation (engine.go).
type LibraryService struct {
	consoles domain.ConsoleRepository
	repo     domain.LibraryRepository
	games    GameDirectory
	files    Files
	log      *slog.Logger
	now      func() time.Time
	newID    func() string

	// mu serializes changes: two of them must not race for a folder or a name.
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

func (s *LibraryService) console(ctx context.Context, match func(domain.Console) bool, what string) (domain.Console, error) {
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return domain.Console{}, err
	}
	for _, c := range consoles {
		if match(c) {
			return c, nil
		}
	}
	return domain.Console{}, reject(RejectInvalid, "La consola %s no existe.", what)
}

func (s *LibraryService) consoleBySlug(ctx context.Context, slug string) (domain.Console, error) {
	return s.console(ctx, func(c domain.Console) bool { return string(c.Slug) == slug }, fmt.Sprintf("%q", slug))
}

func (s *LibraryService) consoleByID(ctx context.Context, id domain.ConsoleID) (domain.Console, error) {
	return s.console(ctx, func(c domain.Console) bool { return c.ID == id }, fmt.Sprint(id))
}

// gameInfo asks IGDB for a game, turning failures into rejections.
func (s *LibraryService) gameInfo(ctx context.Context, igdbID int64) (GameInfo, error) {
	info, err := s.games.GameByID(ctx, igdbID)
	switch {
	case errors.Is(err, ErrUnknownGame):
		return info, reject(RejectInvalid, "El juego %d no existe en IGDB.", igdbID)
	case errors.Is(err, ErrMetadataNotConfigured):
		return info, reject(RejectUnavailable, "IGDB no está configurado (IGDB_CLIENT_ID / IGDB_CLIENT_SECRET).")
	case errors.Is(err, ErrMetadataUnavailable):
		return info, reject(RejectUpstream, "IGDB no responde; inténtalo de nuevo.")
	}
	return info, err
}

// newFolder picks a folder name for a game (RF-11a). self is the game being
// renamed (0 for a new one): its own folder never counts as taken, even when
// only the letter case changes.
func (s *LibraryService) newFolder(ctx context.Context, console domain.Console, info GameInfo, self *domain.Game) (string, error) {
	var except domain.GameID
	if self != nil {
		except = self.ID
	}
	folder, err := domain.GameFolder(info.Name, info.ReleaseYear, info.IGDBID, func(f string) (bool, error) {
		if taken, err := s.repo.FolderTaken(ctx, console.ID, f, except); err != nil || taken {
			return taken, err
		}
		if self != nil && strings.EqualFold(f, self.Folder) {
			return false, nil
		}
		return s.files.Exists(s.files.LibraryPath(string(console.Slug), f))
	})
	if err != nil {
		return "", nameRejection(err)
	}
	return folder, nil
}

// itemNames names every file of an item: one name for files and folder
// games; the .cue sheet and its tracks for discs. parts are the current
// names (only their extensions and count matter).
func itemNames(title string, ref string, shape domain.Shape, n domain.ItemName, parts []string) ([]string, error) {
	if !n.Kind.Valid() {
		return nil, reject(RejectInvalid, "%q: tipo %q desconocido.", ref, n.Kind)
	}
	n.Title = title
	switch shape {
	case domain.ShapeFile, domain.ShapeFolder:
		if len(parts) != 1 {
			return nil, fmt.Errorf("item %q: %s with %d parts", ref, shape, len(parts))
		}
		ext := ""
		if shape == domain.ShapeFile {
			ext = path.Ext(parts[0])
		}
		f, err := n.FileName(ext)
		if err != nil {
			return nil, itemRejection(ref, err)
		}
		return []string{f}, nil
	case domain.ShapeDisc:
		if len(parts) == 0 || !strings.EqualFold(path.Ext(parts[0]), ".cue") {
			return nil, fmt.Errorf("item %q: disc without a .cue sheet", ref)
		}
		stem, err := n.Stem()
		if err != nil {
			return nil, itemRejection(ref, err)
		}
		cue, err := n.FileName(".cue")
		if err != nil {
			return nil, itemRejection(ref, err)
		}
		exts := make([]string, 0, len(parts)-1)
		for _, p := range parts[1:] {
			exts = append(exts, path.Ext(p))
		}
		tracks, err := domain.TrackNames(stem, exts)
		if err != nil {
			return nil, itemRejection(ref, err)
		}
		return append([]string{cue}, tracks...), nil
	}
	return nil, fmt.Errorf("item %q: unknown shape %q", ref, shape)
}

// storedNames renames a stored item for a (new) game title.
func storedNames(title string, it domain.GameItem) ([]string, error) {
	return itemNames(title, it.Files[0], it.Shape, it.Name(title), it.Files)
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

// nameSet detects two items that would get the same file name.
type nameSet map[string]string

func (n nameSet) add(ref string, files []string) error {
	for _, f := range files {
		key := strings.ToLower(f)
		if other, ok := n[key]; ok {
			return reject(RejectInvalid, "%q y %q tendrían el mismo nombre: %s.", other, ref, f)
		}
		n[key] = ref
	}
	return nil
}

// HasItemsFrom reports whether an upload's items are in the library.
func (s *LibraryService) HasItemsFrom(ctx context.Context, source string) (bool, error) {
	return s.repo.HasItemsFrom(ctx, source)
}
