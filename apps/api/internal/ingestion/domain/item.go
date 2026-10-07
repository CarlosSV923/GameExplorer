package domain

import (
	"context"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/detection"
)

// ItemShape is how a staged item is laid out on disk.
type ItemShape string

// Item shapes.
const (
	// ShapeFile is a single file (an .nsp, an .iso...).
	ShapeFile ItemShape = "file"
	// ShapeDisc is a .cue sheet plus the .bin tracks it references.
	ShapeDisc ItemShape = "disc"
	// ShapeFolder is a folder-format game (PS3 JB); never renamed inside.
	ShapeFolder ItemShape = "folder"
)

// Confidence says how a console was suggested.
type Confidence string

// Detection confidence levels.
const (
	ConfidenceHeader    Confidence = "header"
	ConfidenceExtension Confidence = "extension"
	ConfidenceNone      Confidence = "none"
)

// StagedItem is one reviewable unit found in an upload (RF-06, RF-07).
type StagedItem struct {
	JobID JobID
	Shape ItemShape
	// Path is relative to the job's staging directory ("." for a folder game
	// that is the whole upload). For discs it is the .cue file.
	Path string
	// Parts are every file the item consists of (relative paths).
	Parts []string
	Size  int64
	// Ignored items are junk (readme, .nfo...) listed only for transparency.
	Ignored bool

	Consoles       []string
	Confidence     Confidence
	SuggestedKind  detection.ItemKind
	TitleID        string
	VersionCode    string
	DisplayVersion string
	DiscNumber     int
}

// StagedItemRepository persists the items found in each job.
type StagedItemRepository interface {
	// Replace stores items as the full set for the job (re-extraction safe).
	Replace(ctx context.Context, id JobID, items []StagedItem) error
	List(ctx context.Context, id JobID) ([]StagedItem, error)
}
