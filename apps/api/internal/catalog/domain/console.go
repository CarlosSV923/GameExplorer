// Package domain holds the catalog model: consoles, games and their items.
// It is pure: no I/O, no SQL, no HTTP.
package domain

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ConsoleID identifies a console.
type ConsoleID int64

// Slug is the console folder name inside the library (ES-DE style, e.g. "ps2").
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

var extensionPattern = regexp.MustCompile(`^\.[a-z0-9]{1,10}$`)

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

// Console is a platform with its own folder in the library.
type Console struct {
	ID             ConsoleID
	Slug           Slug
	DisplayName    string
	IGDBPlatformID *int64
	ReleaseYear    *int
	LogoImageID    *string
	Extensions     []string
	// DetectorKey links built-in consoles to a header detector; "" for
	// consoles added by the user (detected by extension only).
	DetectorKey string
	SortOrder   int
}

// ConsoleRepository persists consoles.
type ConsoleRepository interface {
	// List returns every console in carousel order.
	List(ctx context.Context) ([]Console, error)
	// UpdatePlatformMetadata stores the logo and, when known, the release year.
	UpdatePlatformMetadata(ctx context.Context, id ConsoleID, logoImageID *string, releaseYear *int) error
}
