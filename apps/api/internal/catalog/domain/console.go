// Package domain holds the catalog model: consoles, games, their files and
// the unassigned section. It is pure: no I/O, no SQL, no HTTP.
package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Slug is the console folder name inside the library (ES-DE style, e.g. "psp").
type Slug string

var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// ErrInvalidSlug is returned for folder names that are unsafe or not ES-DE style.
var ErrInvalidSlug = errors.New("invalid console slug")

// NewSlug validates a console folder name.
func NewSlug(s string) (Slug, error) {
	if !slugPattern.MatchString(s) {
		return "", fmt.Errorf("%w: %q (lower-case letters, digits, '-' or '_', max 32)", ErrInvalidSlug, s)
	}
	return Slug(s), nil
}

// ErrInvalidExtension is returned for extensions that are not ".something".
var ErrInvalidExtension = errors.New("invalid file extension")

// An extension has one to three dot-separated parts: ".iso", ".nkit.iso".
var extensionPattern = regexp.MustCompile(`^(\.[a-z0-9]{1,10}){1,3}$`)

// NormalizeExtension lower-cases an extension and ensures the leading dot.
func NormalizeExtension(ext string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(ext))
	if !strings.HasPrefix(e, ".") {
		e = "." + e
	}
	if !extensionPattern.MatchString(e) {
		return "", fmt.Errorf("%w: %q", ErrInvalidExtension, ext)
	}
	return e, nil
}

// ExtensionOf returns the longest of known that the file name ends with
// ("Game.nkit.iso" is ".nkit.iso" when it is known, even if ".iso" is too),
// or "" when none does (RF-41). A name that is only the extension has none.
func ExtensionOf(name string, known []string) string {
	lower := strings.ToLower(name)
	best := ""
	for _, ext := range known {
		if len(ext) > len(best) && len(lower) > len(ext) && strings.HasSuffix(lower, ext) {
			best = ext
		}
	}
	return best
}

// ConsoleDefinition is a console as defined in code, with its rules (RF-40).
// Adding a console means adding one of these to the registry
// (internal/catalog/domain/consoles); see docs/adding-a-console.md.
type ConsoleDefinition struct {
	Slug Slug
	// Name is the default display name.
	Name           string
	IGDBPlatformID int64
	// ReleaseYear is shown until IGDB provides one.
	ReleaseYear int
	// ExtensionsEnv is the environment variable that overrides
	// DefaultExtensions (comma separated).
	ExtensionsEnv     string
	DefaultExtensions []string
	// Kinds a file of this console can have. A console with KindGame only
	// stores one file per game; the others use base, update and DLC.
	Kinds []ItemKind
	// MultipleFiles allows several game files in one upload.
	MultipleFiles bool
}

// Allows reports whether a file of this console may have kind k.
func (d ConsoleDefinition) Allows(k ItemKind) bool { return slices.Contains(d.Kinds, k) }

// SingleKind is the only kind a console with one kind uses ("" otherwise).
func (d ConsoleDefinition) SingleKind() ItemKind {
	if len(d.Kinds) == 1 {
		return d.Kinds[0]
	}
	return ""
}

// Console is a console with what the user and IGDB changed about it.
type Console struct {
	ConsoleDefinition
	DisplayName string
	SortOrder   int
	LogoImageID *string
	// Year is IGDB's release year, or the definition's.
	Year int
	// Extensions are the fixed ones (environment or code); Custom were added
	// from the app.
	Extensions []string
	Custom     []string
	// GameCount is how many games have files in the library (read model).
	GameCount int
}

// AllExtensions are the fixed and the custom extensions.
func (c Console) AllExtensions() []string {
	return append(slices.Clone(c.Extensions), c.Custom...)
}

// Accepts reports whether ext (as returned by ExtensionOf) is one of the
// console's extensions.
func (c Console) Accepts(ext string) bool {
	return ext != "" && slices.Contains(c.AllExtensions(), ext)
}

// ConsoleSettings is what the database keeps about a console.
type ConsoleSettings struct {
	// DisplayName is nil when the code's name is kept.
	DisplayName *string
	SortOrder   int
	LogoImageID *string
	ReleaseYear *int
}

// ConsoleRepository persists what the user and IGDB change about consoles.
type ConsoleRepository interface {
	// Settings returns the stored settings by slug (consoles never changed
	// have none).
	Settings(ctx context.Context) (map[Slug]ConsoleSettings, error)
	// SetOrder creates missing rows; the other setters only update existing ones.
	SetDisplayName(ctx context.Context, slug Slug, name *string) error
	// SetOrder stores the carousel order: slugs[0] first.
	SetOrder(ctx context.Context, slugs []Slug) error
	SetPlatformMetadata(ctx context.Context, slug Slug, logoImageID *string, releaseYear *int) error
	// CustomExtensions returns the extensions added from the app, by slug.
	CustomExtensions(ctx context.Context) (map[Slug][]string, error)
	AddExtension(ctx context.Context, slug Slug, ext string) error
	RemoveExtension(ctx context.Context, slug Slug, ext string) error
	// GameCounts returns how many games of each console have files in the library.
	GameCounts(ctx context.Context) (map[Slug]int, error)
	// ItemFiles returns the file names of every item of the console's games,
	// trashed ones included.
	ItemFiles(ctx context.Context, slug Slug) ([]string, error)
}
