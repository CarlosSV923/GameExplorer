package domain

import (
	"context"
	"errors"
	"strings"
	"time"
)

// GameID identifies a game in the library.
type GameID int64

// ItemID identifies a stored file.
type ItemID int64

// TrashEntryID identifies a trash entry.
type TrashEntryID int64

// UnassignedID identifies a file of the unassigned section.
type UnassignedID int64

// Not-found errors.
var (
	ErrGameNotFound       = errors.New("game not found")
	ErrItemNotFound       = errors.New("item not found")
	ErrTrashEntryNotFound = errors.New("trash entry not found")
	ErrUnassignedNotFound = errors.New("unassigned file not found")
)

// Game is a name within a console, optionally linked to IGDB (aggregate root).
type Game struct {
	ID      GameID
	Console Slug
	Title   string
	// Folder is the game's directory name inside the console folder.
	Folder       string
	IGDBID       *int64
	ReleaseYear  *int
	CoverImageID *string
	Summary      *string
	Genres       []string
	Items        []GameItem
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// GameItem is one stored file: a base game, an update, a DLC or the game.
type GameItem struct {
	ID     ItemID
	GameID GameID
	Kind   ItemKind
	// Label is the update version (without "v") or the DLC name.
	Label string
	// File is the name inside the game folder (or the trash entry's
	// directory while trashed).
	File string
	Size int64
	// SourceJob is the upload the file came from.
	SourceJob string
	CreatedAt time.Time
	// TrashEntry is set while the file is in the trash.
	TrashEntry *TrashEntryID
}

// Live reports whether the file is in the library (not in the trash).
func (i GameItem) Live() bool { return i.TrashEntry == nil }

// Name describes the file for naming under a game title.
func (i GameItem) Name(title string) ItemName {
	return ItemName{Title: title, Kind: i.Kind, Label: i.Label}
}

// LiveItems returns the files that are not in the trash.
func (g *Game) LiveItems() []GameItem {
	var out []GameItem
	for _, it := range g.Items {
		if it.Live() {
			out = append(out, it)
		}
	}
	return out
}

// FindDuplicate applies the duplicate policy (RF-09): a live file of the
// game with the same final name, ignoring case. It returns nil when there
// is none.
func FindDuplicate(existing []GameItem, file string) *GameItem {
	for i := range existing {
		if existing[i].Live() && strings.EqualFold(existing[i].File, file) {
			return &existing[i]
		}
	}
	return nil
}

// DuplicateAction is the user's decision for a duplicate.
type DuplicateAction string

// Duplicate decisions.
const (
	// Replace sends the existing file to the trash.
	Replace DuplicateAction = "replace"
	// Skip keeps the existing file and drops the other one.
	Skip DuplicateAction = "skip"
)

// UnassignedReason says how a file reached the unassigned section.
type UnassignedReason string

// Unassigned reasons.
const (
	UnassignedSamba  UnassignedReason = "samba"  // found by the scan
	UnassignedUpload UnassignedReason = "upload" // an upload that did not fit its console
	UnassignedManual UnassignedReason = "manual" // moved here by the user
)

// UnassignedFile is a file of the unassigned section (RF-27).
type UnassignedFile struct {
	ID UnassignedID
	// Path is relative to the unassigned folder, "/" separators; while
	// trashed, relative to the trash entry's directory.
	Path      string
	Origin    string
	Reason    UnassignedReason
	Size      int64
	ArrivedAt time.Time
	// TrashEntry is set while the file is in the trash.
	TrashEntry *TrashEntryID
	// Console is the console it came from, "" when unknown (prefills Asignar).
	Console Slug
	// IGDBID is the IGDB game picked when it was uploaded without console.
	IGDBID *int64
	// Job is the assignment in progress using the file ("" when free).
	Job string
}

// Name is the file's base name.
func (u UnassignedFile) Name() string {
	if i := strings.LastIndex(u.Path, "/"); i >= 0 {
		return u.Path[i+1:]
	}
	return u.Path
}

// EntryKey is the entry of the section the file belongs to (RF-27): its
// first-level folder, or the file itself when it is loose.
func (u UnassignedFile) EntryKey() string {
	return EntryKeyOf(u.Path)
}

// EntryKeyOf is the entry of a path relative to the unassigned folder.
func EntryKeyOf(rel string) string {
	if i := strings.Index(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return rel
}

// TrashReason says why something went to the trash.
type TrashReason string

// Trash reasons.
const (
	TrashDeleted  TrashReason = "deleted"  // sent by the user
	TrashReplaced TrashReason = "replaced" // replaced by a duplicate
)

// TrashEntry is what was sent to the trash together: one file, a whole
// game, or unassigned files (GameID nil). Its files live under Dir.
type TrashEntry struct {
	ID        TrashEntryID
	GameID    *GameID
	WholeGame bool
	Reason    TrashReason
	Dir       string
	TrashedAt time.Time
	Items     []GameItem
	Files     []UnassignedFile
}

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
	Console      Slug
	IGDBID       *int64
	Title        string
	Folder       string
	ReleaseYear  *int
	CoverImageID *string
	ItemCount    int
	Size         int64
}

// LibraryFile is a live file of the library with where it is (read model
// for the scan).
type LibraryFile struct {
	Item    ItemID
	Game    GameID
	Console Slug
	Folder  string
	File    string
}

// PendingFile is an unknown file seen by a scan, waiting to be unchanged on
// the next one (RF-26).
type PendingFile struct {
	// Path is relative to the library root, "/" separators.
	Path    string
	Size    int64
	ModTime time.Time
}

// LibraryTx records the catalog side of one library change, atomically.
type LibraryTx interface {
	InsertGame(ctx context.Context, g Game, now time.Time) (GameID, error)
	// UpdateGame stores console, title, folder, IGDB link and metadata.
	UpdateGame(ctx context.Context, g Game, now time.Time) error
	DeleteGame(ctx context.Context, id GameID) error
	// DeleteGameIfEmpty deletes the game when no file (not even a trashed one) is left.
	DeleteGameIfEmpty(ctx context.Context, id GameID) error
	InsertItem(ctx context.Context, it GameItem, now time.Time) (ItemID, error)
	// UpdateItem stores the item's game, kind, label and file name.
	UpdateItem(ctx context.Context, it GameItem) error
	DeleteItem(ctx context.Context, id ItemID) error
	// MoveGameContents moves every file and trash entry of one game to another.
	MoveGameContents(ctx context.Context, from, to GameID) error
	InsertTrashEntry(ctx context.Context, e TrashEntry) (TrashEntryID, error)
	SetItemTrash(ctx context.Context, id ItemID, entry *TrashEntryID) error
	DeleteTrashEntry(ctx context.Context, id TrashEntryID) error
	InsertUnassigned(ctx context.Context, f UnassignedFile) (UnassignedID, error)
	// SetUnassignedPlace stores the file's path and trash entry.
	SetUnassignedPlace(ctx context.Context, id UnassignedID, path string, entry *TrashEntryID) error
	DeleteUnassigned(ctx context.Context, id UnassignedID) error
}

// LibraryRepository persists games, files, the unassigned section, the
// trash and the operation journal.
type LibraryRepository interface {
	// ListGames returns the games with files outside the trash, by title.
	ListGames(ctx context.Context) ([]GameSummary, error)
	// GameByID returns a game with every file (ErrGameNotFound).
	GameByID(ctx context.Context, id GameID) (*Game, error)
	// GameByFolder returns the console's game with that folder, ignoring
	// case, with every file (ErrGameNotFound).
	GameByFolder(ctx context.Context, console Slug, folder string) (*Game, error)
	// ItemByID returns one file (ErrItemNotFound).
	ItemByID(ctx context.Context, id ItemID) (GameItem, error)
	// LibraryFiles returns every live file with its place.
	LibraryFiles(ctx context.Context) ([]LibraryFile, error)
	// HasItemsFrom reports whether any file came from the upload.
	HasItemsFrom(ctx context.Context, source string) (bool, error)

	// UnassignedFiles returns the files of the section (not in the trash), newest first.
	UnassignedFiles(ctx context.Context) ([]UnassignedFile, error)
	// UnassignedByID returns one file of the section, even while trashed (ErrUnassignedNotFound).
	UnassignedByID(ctx context.Context, id UnassignedID) (UnassignedFile, error)
	// SetUnassignedJob marks a file as used by an assignment ("" frees it).
	SetUnassignedJob(ctx context.Context, id UnassignedID, job string) error
	// ReleaseUnassignedJob frees every file the assignment was using.
	ReleaseUnassignedJob(ctx context.Context, job string) error

	// TrashEntries returns every entry with its files, newest first.
	TrashEntries(ctx context.Context) ([]TrashEntry, error)
	// TrashEntry returns one entry with its files (ErrTrashEntryNotFound).
	TrashEntry(ctx context.Context, id TrashEntryID) (*TrashEntry, error)

	// PendingFiles returns the unknown files seen by the last scan.
	PendingFiles(ctx context.Context) ([]PendingFile, error)
	// SetPendingFiles replaces them.
	SetPendingFiles(ctx context.Context, files []PendingFile) error

	SaveOperation(ctx context.Context, op Operation) error
	Operations(ctx context.Context) ([]Operation, error)
	DeleteOperation(ctx context.Context, id string) error
	// Apply runs fn in one transaction and, when operationID is not empty,
	// deletes that journal entry in the same transaction.
	Apply(ctx context.Context, operationID string, fn func(LibraryTx) error) error
}
