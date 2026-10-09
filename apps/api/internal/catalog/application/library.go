package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
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

// Consoles gives the library the consoles and their extensions.
type Consoles interface {
	List(ctx context.Context) ([]domain.Console, error)
	Console(ctx context.Context, slug string) (domain.Console, error)
}

// Files is the library's file system. Every path is absolute and must lie
// inside the library (staging, trash, scratch and the unassigned folder live
// there too, so moves are renames).
type Files interface {
	// LibraryPath joins elem under the library root.
	LibraryPath(elem ...string) string
	// TrashPath joins elem under the trash directory.
	TrashPath(elem ...string) string
	// ScratchPath joins elem under the directory for operations' scratch files.
	ScratchPath(elem ...string) string
	// UnassignedPath joins elem under the unassigned folder.
	UnassignedPath(elem ...string) string
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
	// Tree lists the regular files at path: the file itself, or every file
	// under a directory (sorted, Rel with "/" separators). Links and special
	// files are skipped.
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

// Action is what will happen to a planned file.
type Action string

// Planned actions.
const (
	ActionStore   Action = "store"
	ActionReplace Action = "replace"
	ActionSkip    Action = "skip"
	// ActionUndecided means the file is a duplicate and needs a decision.
	ActionUndecided Action = "undecided"
)

// LibraryService changes the library: stores uploads, edits games, manages
// the unassigned section, the trash and the scan of changes made over SMB
// (RF-07..RF-11, RF-24..RF-30). Every change goes through a journaled
// operation (engine.go).
type LibraryService struct {
	consoles Consoles
	repo     domain.LibraryRepository
	games    GameDirectory
	files    Files
	log      *slog.Logger
	now      func() time.Time
	newID    func() string

	// mu serializes changes: two of them must not race for a folder or a name.
	mu sync.Mutex
	// lastScan is the last scan report (guarded by mu).
	lastScan *ScanReport
	// unassignedSeen is the unassigned folder's unknown files as the last
	// read saw them (guarded by mu, RF-26a).
	unassignedSeen map[string]seenFile
}

// NewLibraryService builds the service. now may be nil (time.Now).
func NewLibraryService(consoles Consoles, repo domain.LibraryRepository, games GameDirectory, files Files, log *slog.Logger, now func() time.Time) *LibraryService {
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

func (s *LibraryService) console(ctx context.Context, slug domain.Slug) (domain.Console, error) {
	c, err := s.consoles.Console(ctx, string(slug))
	if errors.Is(err, ErrConsoleNotFound) {
		return c, reject(RejectInvalid, "La consola %q no existe.", slug)
	}
	return c, err
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

// GameName is the name the user gave a game, optionally picked from IGDB.
type GameName struct {
	Title  string
	IGDBID *int64
}

// describe turns a GameName into a game with its title, folder and, when
// linked to IGDB, its metadata. A name picked from IGDB uses IGDB's title.
func (s *LibraryService) describe(ctx context.Context, console domain.Slug, n GameName) (domain.Game, error) {
	g := domain.Game{Console: console, Title: strings.Join(strings.Fields(n.Title), " ")}
	if n.IGDBID != nil {
		info, err := s.gameInfo(ctx, *n.IGDBID)
		if err != nil {
			return g, err
		}
		id := info.IGDBID
		g.IGDBID, g.Title = &id, info.Name
		g.ReleaseYear, g.CoverImageID, g.Summary, g.Genres = info.ReleaseYear, info.CoverImageID, info.Summary, info.Genres
	}
	folder, err := domain.GameFolder(g.Title)
	if err != nil {
		return g, nameRejection(err)
	}
	g.Folder = folder
	return g, nil
}

// fileName names a file of a game; ext comes from the file's current name.
func fileName(title string, kind domain.ItemKind, label, ext, ref string) (string, error) {
	name, err := domain.ItemName{Title: title, Kind: kind, Label: label}.FileName(ext)
	if err != nil {
		return "", itemRejection(ref, err)
	}
	return name, nil
}

// extensionOf returns a stored or staged file's extension among every
// console's extensions; a file whose extension is no longer known keeps its
// last dot-suffix.
func extensionOf(name string, consoles []domain.Console) string {
	if ext := domain.ExtensionOf(name, KnownExtensions(consoles)); ext != "" {
		return ext
	}
	if i := strings.LastIndex(name, "."); i > 0 {
		return strings.ToLower(name[i:])
	}
	return ""
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
		return reject(RejectInvalid, "El nombre del juego no deja un nombre de archivo válido.")
	case ne.Field == "version":
		return reject(RejectInvalid, "La versión del update debe tener solo números y puntos (por ejemplo 1.0.4).")
	case ne.Field == "label":
		return reject(RejectInvalid, "Falta el nombre del DLC.")
	case ne.Field == "extension":
		return reject(RejectInvalid, "La extensión del archivo no es válida.")
	}
	return reject(RejectInvalid, "Tipo de archivo no válido.")
}

// itemFor checks a file's kind and label against its console and returns
// them clean.
func itemFor(c domain.Console, kind domain.ItemKind, label, ref string) (domain.ItemKind, string, error) {
	if kind == "" {
		kind = c.SingleKind()
	}
	if kind == "" && c.Allows(domain.KindGame) {
		kind = domain.KindGame // one disc: the game itself (spec §5)
	}
	if kind == "" {
		return "", "", reject(RejectInvalid, "%q: indica si es el juego base, un update o un DLC.", ref)
	}
	if !c.Allows(kind) {
		return "", "", reject(RejectInvalid, "%q: %s no admite el tipo %q.", ref, c.DisplayName, kind)
	}
	clean, err := domain.CleanLabel(kind, label)
	if err != nil {
		return "", "", itemRejection(ref, err)
	}
	return kind, clean, nil
}

// nameSet detects two files that would get the same name.
type nameSet map[string]string

func (n nameSet) add(ref, file string) error {
	key := strings.ToLower(file)
	if other, ok := n[key]; ok {
		return reject(RejectInvalid, "%q y %q tendrían el mismo nombre: %s.", other, ref, file)
	}
	n[key] = ref
	return nil
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

// HasItemsFrom reports whether an upload's files are in the library.
func (s *LibraryService) HasItemsFrom(ctx context.Context, source string) (bool, error) {
	return s.repo.HasItemsFrom(ctx, source)
}

// gameDir is a game's folder.
func (s *LibraryService) gameDir(g *domain.Game) string {
	return s.files.LibraryPath(string(g.Console), g.Folder)
}

// pruneGame removes a game's folder and then its console's folder when
// they are left empty (RF-25).
func (b *opBuilder) pruneGame(console domain.Slug, folder string) {
	b.prune = append(b.prune, b.s.files.LibraryPath(string(console), folder), b.s.files.LibraryPath(string(console)))
}
