// Package domain is the metadata model: our own view of games and platforms,
// independent of IGDB's schema (this context is an anti-corruption layer).
package domain

import (
	"cmp"
	"errors"
	"regexp"
	"slices"
	"strings"
)

var (
	// ErrNotConfigured means IGDB credentials are missing.
	ErrNotConfigured = errors.New("metadata provider not configured")
	// ErrUpstream means IGDB failed or could not be reached.
	ErrUpstream = errors.New("metadata provider error")
	// ErrNotFound means the requested image does not exist upstream.
	ErrNotFound = errors.New("not found")
	// ErrInvalidQuery is returned for search terms that are too short or long.
	ErrInvalidQuery = errors.New("invalid search query")
	// ErrInvalidImage is returned for unknown sizes or malformed image ids.
	ErrInvalidImage = errors.New("invalid image reference")
)

// Game is a game as GameExplorer needs it.
type Game struct {
	ID           int64
	Name         string
	ReleaseYear  *int
	CoverImageID *string
	Summary      *string
	Genres       []string
	PlatformIDs  []int64
}

// Platform is a gaming platform as GameExplorer needs it.
type Platform struct {
	ID           int64
	Name         string
	Abbreviation *string
	LogoImageID  *string
	ReleaseYear  *int
}

// ImageSize is an allowed image preset.
type ImageSize string

// Allowed image sizes.
const (
	CoverSmall    ImageSize = "cover_small"
	CoverBig      ImageSize = "cover_big"
	LogoMed       ImageSize = "logo_med"
	ScreenshotMed ImageSize = "screenshot_med"
)

var imageIDPattern = regexp.MustCompile(`^[a-z0-9]{1,40}$`)

// ImageRef identifies one cached image.
type ImageRef struct {
	Size ImageSize
	ID   string
}

// NewImageRef validates size and id (both end up in a file path and a URL).
func NewImageRef(size, id string) (ImageRef, error) {
	s := ImageSize(size)
	switch s {
	case CoverSmall, CoverBig, LogoMed, ScreenshotMed:
	default:
		return ImageRef{}, ErrInvalidImage
	}
	if !imageIDPattern.MatchString(id) {
		return ImageRef{}, ErrInvalidImage
	}
	return ImageRef{Size: s, ID: id}, nil
}

// Extension is the file format: logos need PNG transparency, the rest are JPEG.
func (r ImageRef) Extension() string {
	if r.Size == LogoMed {
		return ".png"
	}
	return ".jpg"
}

// ContentType matches Extension.
func (r ImageRef) ContentType() string {
	if r.Size == LogoMed {
		return "image/png"
	}
	return "image/jpeg"
}

// NormalizeQuery trims a search term and enforces 2..100 characters.
func NormalizeQuery(q string) (string, error) {
	q = strings.Join(strings.Fields(q), " ")
	if n := len([]rune(q)); n < 2 || n > 100 {
		return "", ErrInvalidQuery
	}
	return q, nil
}

// RankByName orders games by how well their name matches the query: exact,
// then prefix, then whole-word containment, then the provider's order. IGDB's
// relevance puts bundles and DLC before the game the user typed.
func RankByName(games []Game, query string) []Game {
	q := fold(query)
	score := func(name string) int {
		n := fold(name)
		switch {
		case n == q:
			return 0
		case strings.HasPrefix(n, q):
			return 1
		case strings.Contains(" "+n+" ", " "+q+" "):
			return 2
		default:
			return 3
		}
	}
	out := slices.Clone(games)
	slices.SortStableFunc(out, func(a, b Game) int { return cmp.Compare(score(a.Name), score(b.Name)) })
	return out
}

// fold lower-cases and drops punctuation so "INSIDE" == "Inside" and
// "Zelda: Breath" matches "zelda breath".
func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r > 127:
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
