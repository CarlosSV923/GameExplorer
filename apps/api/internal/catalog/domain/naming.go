package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ItemKind is what an item is within a game folder.
type ItemKind string

// Item kinds (spec §5).
const (
	KindBase   ItemKind = "base"
	KindUpdate ItemKind = "update"
	KindDLC    ItemKind = "dlc"
	KindDisc   ItemKind = "disc"
)

// Valid reports whether k is a known kind.
func (k ItemKind) Valid() bool {
	switch k {
	case KindBase, KindUpdate, KindDLC, KindDisc:
		return true
	}
	return false
}

// Naming errors. NameError says which field was rejected.
var (
	ErrInvalidName = errors.New("invalid name")
	ErrNameTooLong = errors.New("name too long")
)

// NameError is a rejected naming input.
type NameError struct {
	Field string // title | label | disc | kind | extension
	Err   error
}

func (e *NameError) Error() string { return fmt.Sprintf("%s: %v", e.Field, e.Err) }
func (e *NameError) Unwrap() error { return e.Err }

// maxNameBytes is NAME_MAX on Linux file systems (ZFS included).
const maxNameBytes = 255

var colonReplacer = strings.NewReplacer(":", " -", "꞉", " -", "/", " ", `\`, " ")

// SanitizeTitle turns an IGDB title (or a label) into something every file
// system and SMB client accepts (spec §5). It returns "" when nothing is left.
func SanitizeTitle(s string) string {
	s = norm.NFC.String(s)
	s = colonReplacer.Replace(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case unicode.IsSpace(r):
			return ' '
		case unicode.IsControl(r), strings.ContainsRune(`?<>"|*`, r):
			return -1
		}
		return r
	}, s)
	s = strings.Join(strings.Fields(s), " ")
	return strings.TrimRight(s, " .")
}

// ItemName describes one item to name.
type ItemName struct {
	Title      string
	Kind       ItemKind
	Label      string // update version or DLC name
	DiscNumber int
}

// Stem is the item's name without extension: "Juego [Update v1.2.1]".
func (n ItemName) Stem() (string, error) {
	title := SanitizeTitle(n.Title)
	if title == "" {
		return "", &NameError{Field: "title", Err: ErrInvalidName}
	}
	var stem string
	switch n.Kind {
	case KindBase:
		stem = title
	case KindUpdate, KindDLC:
		label := SanitizeTitle(n.Label)
		if label == "" {
			return "", &NameError{Field: "label", Err: ErrInvalidName}
		}
		if n.Kind == KindUpdate {
			stem = title + " [Update " + label + "]"
		} else {
			stem = title + " [DLC] " + label
		}
	case KindDisc:
		if n.DiscNumber < 1 || n.DiscNumber > 99 {
			return "", &NameError{Field: "disc", Err: ErrInvalidName}
		}
		stem = title + " (Disc " + strconv.Itoa(n.DiscNumber) + ")"
	default:
		return "", &NameError{Field: "kind", Err: ErrInvalidName}
	}
	return stem, nil
}

// FileName is Stem plus the lower-cased extension ("" for folder games).
func (n ItemName) FileName(ext string) (string, error) {
	stem, err := n.Stem()
	if err != nil {
		return "", err
	}
	return withExtension(stem, ext)
}

// TrackNames names the tracks of a disc, Redump style: one track takes the
// disc's name; several are "(Track N)", with two digits from ten tracks on.
func TrackNames(stem string, extensions []string) ([]string, error) {
	out := make([]string, len(extensions))
	for i, ext := range extensions {
		name := stem
		if len(extensions) > 1 {
			format := "%s (Track %d)"
			if len(extensions) >= 10 {
				format = "%s (Track %02d)"
			}
			name = fmt.Sprintf(format, stem, i+1)
		}
		var err error
		if out[i], err = withExtension(name, ext); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func withExtension(stem, ext string) (string, error) {
	ext = strings.ToLower(ext)
	if ext != "" && (!strings.HasPrefix(ext, ".") || SanitizeTitle(ext) != ext || strings.Contains(ext, " ")) {
		return "", &NameError{Field: "extension", Err: ErrInvalidName}
	}
	name := stem + ext
	if len(name) > maxNameBytes {
		return "", &NameError{Field: "title", Err: ErrNameTooLong}
	}
	return name, nil
}

// GameFolder picks the game's folder name inside its console folder. A
// different game may already use the plain title (a remake, or a folder
// created over SMB): then the release year is added and, if that is taken
// too, the IGDB id.
func GameFolder(title string, year *int, igdbID int64, taken func(string) (bool, error)) (string, error) {
	base := SanitizeTitle(title)
	if base == "" {
		return "", &NameError{Field: "title", Err: ErrInvalidName}
	}
	candidates := []string{base}
	if year != nil {
		base = fmt.Sprintf("%s (%d)", base, *year)
		candidates = append(candidates, base)
	}
	candidates = append(candidates, fmt.Sprintf("%s [%d]", base, igdbID))
	for _, c := range candidates {
		if len(c) > maxNameBytes {
			return "", &NameError{Field: "title", Err: ErrNameTooLong}
		}
		t, err := taken(c)
		if err != nil {
			return "", err
		}
		if !t {
			return c, nil
		}
	}
	return "", fmt.Errorf("game folder %q: every candidate name is taken", candidates[len(candidates)-1])
}
