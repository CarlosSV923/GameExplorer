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

// TrashEntryID identifies a trash entry.
type TrashEntryID int64

// Shape is how an item is laid out on disk.
type Shape string

// Item shapes.
const (
	ShapeFile   Shape = "file"   // one file
	ShapeDisc   Shape = "disc"   // a .cue sheet plus its tracks
	ShapeFolder Shape = "folder" // folder-format game (PS3), never renamed inside
)

// Not-found errors.
var (
	ErrGameNotFound       = errors.New("game not found")
	ErrItemNotFound       = errors.New("item not found")
	ErrTrashEntryNotFound = errors.New("trash entry not found")
)

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
	// Files are relative to the game folder (or to the trash entry's
	// directory while trashed); the first one is the item's main entry (the
	// .cue for discs, the directory for folder games).
	Files   []string
	Size    int64
	TitleID string
	// SourceJob is the upload the item came from.
	SourceJob string
	CreatedAt time.Time
	// TrashEntry is set while the item is in the trash.
	TrashEntry *TrashEntryID
	// MissingSince is set while a file of the item is missing from disk
	// (deleted over SMB), as found by the integrity check.
	MissingSince *time.Time
}

// Live reports whether the item is in the library (not in the trash).
func (i GameItem) Live() bool { return i.TrashEntry == nil }

// Name describes the item for naming.
func (i GameItem) Name(title string) ItemName {
	return ItemName{Title: title, Kind: i.Kind, Label: i.Label, DiscNumber: i.DiscNumber}
}

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

// TrashReason says why something went to the trash.
type TrashReason string

// Trash reasons.
const (
	TrashDeleted  TrashReason = "deleted"  // sent by the user
	TrashReplaced TrashReason = "replaced" // replaced by a duplicate
)

// TrashEntry is what was sent to the trash together: one item or a whole
// game. Its files live in the trash under Dir.
type TrashEntry struct {
	ID        TrashEntryID
	GameID    GameID
	WholeGame bool
	Reason    TrashReason
	Dir       string
	TrashedAt time.Time
	Items     []GameItem
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
	// Skip keeps the existing item and drops the other one.
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

// GameSummary is a game as listed in the library (read model).
type GameSummary struct {
	ID           GameID
	ConsoleID    ConsoleID
	Title        string
	Folder       string
	ReleaseYear  *int
	CoverImageID *string
	ItemCount    int
	MissingCount int
	Size         int64
}

// LibraryTx records the catalog side of one library change, atomically.
type LibraryTx interface {
	InsertGame(ctx context.Context, g Game, now time.Time) (GameID, error)
	// UpdateGame stores IGDB id, title, folder and metadata.
	UpdateGame(ctx context.Context, g Game, now time.Time) error
	DeleteGame(ctx context.Context, id GameID) error
	// DeleteGameIfEmpty deletes the game when no item (not even a trashed one) is left.
	DeleteGameIfEmpty(ctx context.Context, id GameID) error
	InsertItem(ctx context.Context, it GameItem, now time.Time) (ItemID, error)
	// PlaceItem sets the item's game and file names.
	PlaceItem(ctx context.Context, id ItemID, game GameID, files []string) error
	DeleteItem(ctx context.Context, id ItemID) error
	SetMissing(ctx context.Context, id ItemID, since *time.Time) error
	// MoveGameContents moves every item and trash entry of one game to another.
	MoveGameContents(ctx context.Context, from, to GameID) error
	InsertTrashEntry(ctx context.Context, e TrashEntry) (TrashEntryID, error)
	SetItemTrash(ctx context.Context, id ItemID, entry *TrashEntryID) error
	DeleteTrashEntry(ctx context.Context, id TrashEntryID) error
}

// LibraryRepository persists games, items, the trash and the operation journal.
type LibraryRepository interface {
	// ListGames returns the games with items outside the trash, by title.
	ListGames(ctx context.Context) ([]GameSummary, error)
	// GameByID returns a game with every item (ErrGameNotFound).
	GameByID(ctx context.Context, id GameID) (*Game, error)
	// ItemByID returns one item (ErrItemNotFound).
	ItemByID(ctx context.Context, id ItemID) (GameItem, error)
	// FindGame returns the console's game for an IGDB id, with every item.
	FindGame(ctx context.Context, console ConsoleID, igdbID int64) (*Game, error)
	// FolderTaken reports whether another game of the console (other than
	// except) uses the folder.
	FolderTaken(ctx context.Context, console ConsoleID, folder string, except GameID) (bool, error)
	// HasItemsFrom reports whether any item came from the upload.
	HasItemsFrom(ctx context.Context, source string) (bool, error)

	// TrashEntries returns every entry with its items, newest first.
	TrashEntries(ctx context.Context) ([]TrashEntry, error)
	// TrashEntry returns one entry with its items (ErrTrashEntryNotFound).
	TrashEntry(ctx context.Context, id TrashEntryID) (*TrashEntry, error)

	SaveOperation(ctx context.Context, op Operation) error
	Operations(ctx context.Context) ([]Operation, error)
	DeleteOperation(ctx context.Context, id string) error
	// Apply runs fn in one transaction and, when operationID is not empty,
	// deletes that journal entry in the same transaction.
	Apply(ctx context.Context, operationID string, fn func(LibraryTx) error) error
}
