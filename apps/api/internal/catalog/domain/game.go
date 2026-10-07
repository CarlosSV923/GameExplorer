package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// GameID identifies a game in the library.
type GameID int64

// ItemID identifies a stored item.
type ItemID int64

// Shape is how an item is laid out on disk.
type Shape string

// Item shapes.
const (
	ShapeFile   Shape = "file"   // one file
	ShapeDisc   Shape = "disc"   // a .cue sheet plus its tracks
	ShapeFolder Shape = "folder" // folder-format game (PS3), never renamed inside
)

// ErrGameNotFound is returned when a game does not exist in the library.
var ErrGameNotFound = errors.New("game not found")

// Game is a game of one console, matched to an IGDB entry (aggregate root).
type Game struct {
	ID        GameID
	ConsoleID ConsoleID
	IGDBID    int64
	Title     string
	// Folder is the game's directory name inside the console folder.
	Folder       string
	ReleaseYear  *int
	CoverImageID *string
	Summary      *string
	Genres       []string
	Items        []GameItem
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GameItem is one stored item: a base game, an update, a DLC or a disc.
type GameItem struct {
	ID         ItemID
	GameID     GameID
	Kind       ItemKind
	Label      string
	DiscNumber int
	Shape      Shape
	// Files are relative to the game folder; the first one is the item's
	// main entry (the .cue for discs, the directory for folder games).
	Files   []string
	Size    int64
	TitleID string
	// SourceJob is the upload the item came from.
	SourceJob string
	CreatedAt time.Time
	// TrashedAt is set while the item is in the trash; TrashDir is its
	// directory inside the trash.
	TrashedAt *time.Time
	TrashDir  string
}

// Live reports whether the item is in the library (not in the trash).
func (i GameItem) Live() bool { return i.TrashedAt == nil }

// LiveItems returns the items that are not in the trash.
func (g *Game) LiveItems() []GameItem {
	var out []GameItem
	for _, it := range g.Items {
		if it.Live() {
			out = append(out, it)
		}
	}
	return out
}

// Candidate is an item about to be stored, as the duplicate policy sees it.
type Candidate struct {
	Kind       ItemKind
	Label      string
	DiscNumber int
	Files      []string
}

// FindDuplicate applies the duplicate policy (RF-10): within the same game,
// a base, the same disc, an update with the same version or any item that
// would use one of the same file names is a duplicate. It returns nil when
// the candidate is new.
func FindDuplicate(existing []GameItem, c Candidate) *GameItem {
	names := map[string]bool{}
	for _, f := range c.Files {
		names[strings.ToLower(f)] = true
	}
	for i := range existing {
		it := &existing[i]
		if !it.Live() {
			continue
		}
		if it.Kind == c.Kind {
			switch c.Kind {
			case KindBase:
				return it
			case KindDisc:
				if it.DiscNumber == c.DiscNumber {
					return it
				}
			case KindUpdate:
				if sameVersion(it.Label, c.Label) {
					return it
				}
			case KindDLC:
			}
		}
		for _, f := range it.Files {
			if names[strings.ToLower(f)] {
				return it
			}
		}
	}
	return nil
}

// sameVersion compares update labels ignoring case, spaces and a leading "v".
func sameVersion(a, b string) bool {
	clean := func(s string) string {
		s = strings.ToLower(strings.Join(strings.Fields(s), ""))
		return strings.TrimPrefix(s, "v")
	}
	return clean(a) == clean(b)
}

// DuplicateAction is the user's decision for a duplicate.
type DuplicateAction string

// Duplicate decisions.
const (
	// Replace sends the existing item to the trash.
	Replace DuplicateAction = "replace"
	// Skip keeps the existing item and discards the new one.
	Skip DuplicateAction = "skip"
)

// Move is one rename of a library operation.
type Move struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Operation is the journal of a library change in progress: if the process
// stops before the change is recorded, the moves are undone on restart.
type Operation struct {
	ID          string
	Source      string
	Moves       []Move
	CreatedDirs []string
	CreatedAt   time.Time
}

// Changes are the catalog updates of one operation, applied atomically.
type Changes struct {
	OperationID string
	// Game is created when its ID is 0, otherwise its metadata is refreshed.
	Game    Game
	New     []GameItem
	Trashed []TrashedItem
	Now     time.Time
}

// TrashedItem is an item replaced by a new upload.
type TrashedItem struct {
	ID       ItemID
	TrashDir string
}

// LibraryRepository persists games, items and the operation journal.
type LibraryRepository interface {
	// FindGame returns the console's game for an IGDB id, with every item.
	FindGame(ctx context.Context, console ConsoleID, igdbID int64) (*Game, error)
	// FolderTaken reports whether another game of the console uses the folder.
	FolderTaken(ctx context.Context, console ConsoleID, folder string) (bool, error)
	// HasItemsFrom reports whether any item came from the upload.
	HasItemsFrom(ctx context.Context, source string) (bool, error)

	SaveOperation(ctx context.Context, op Operation) error
	Operations(ctx context.Context) ([]Operation, error)
	DeleteOperation(ctx context.Context, id string) error
	// Apply records the changes and deletes the operation, in one transaction.
	Apply(ctx context.Context, c Changes) (GameID, error)
}
