package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ItemKind is what a file is within its game (spec §5).
type ItemKind string

// Item kinds. Consoles with add-ons (Switch) use base, update and DLC;
// a game of one file uses game, and each disc of a game of several discs
// (GameCube, PS2) is a disc with its number as label.
const (
	KindBase   ItemKind = "base"
	KindUpdate ItemKind = "update"
	KindDLC    ItemKind = "dlc"
	KindGame   ItemKind = "game"
	KindDisc   ItemKind = "disc"
)

// Valid reports whether k is a known kind.
func (k ItemKind) Valid() bool {
	switch k {
	case KindBase, KindUpdate, KindDLC, KindGame, KindDisc:
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
	Field string // title | label | version | disc | kind | extension
	Err   error
}

func (e *NameError) Error() string { return fmt.Sprintf("%s: %v", e.Field, e.Err) }
func (e *NameError) Unwrap() error { return e.Err }

// maxNameBytes is NAME_MAX on Linux file systems (ZFS included).
const maxNameBytes = 255

var colonReplacer = strings.NewReplacer(":", " -", "꞉", " -", "/", " ", `\`, " ")

// SanitizeTitle turns a game title (or a DLC name) into something every
// file system and SMB client accepts (spec §5). It returns "" when nothing
// is left.
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

var versionPattern = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)

// NormalizeVersion turns what the user typed for an update ("1.0.4",
// "v1.0.4", "122345") into the stored version, without the "v".
func NormalizeVersion(s string) (string, error) {
	v := strings.TrimSpace(s)
	v = strings.TrimPrefix(strings.TrimPrefix(v, "v"), "V")
	if len(v) > 32 || !versionPattern.MatchString(v) {
		return "", &NameError{Field: "version", Err: ErrInvalidName}
	}
	return v, nil
}

var discPattern = regexp.MustCompile(`^[0-9]{1,3}$`)

// NormalizeDisc turns a typed disc number (" 01 ") into the stored one
// ("1"): 1 to 99, without leading zeros (spec §5).
func NormalizeDisc(s string) (string, error) {
	d := strings.TrimSpace(s)
	if !discPattern.MatchString(d) {
		return "", &NameError{Field: "disc", Err: ErrInvalidName}
	}
	d = strings.TrimLeft(d, "0")
	if d == "" || len(d) > 2 {
		return "", &NameError{Field: "disc", Err: ErrInvalidName}
	}
	return d, nil
}

// ItemName describes one file to name.
type ItemName struct {
	Title string
	Kind  ItemKind
	// Label is the update version (as NormalizeVersion returns it), the
	// DLC name or the disc number; empty for base and game.
	Label string
}

// Stem is the file's name without extension: "Limbo [UPDATE v1.0.4]".
func (n ItemName) Stem() (string, error) {
	title := SanitizeTitle(n.Title)
	if title == "" {
		return "", &NameError{Field: "title", Err: ErrInvalidName}
	}
	switch n.Kind {
	case KindGame:
		return title, nil
	case KindBase:
		return title + " [BASE]", nil
	case KindUpdate:
		v, err := NormalizeVersion(n.Label)
		if err != nil {
			return "", err
		}
		return title + " [UPDATE v" + v + "]", nil
	case KindDLC:
		name := SanitizeTitle(n.Label)
		if name == "" {
			return "", &NameError{Field: "label", Err: ErrInvalidName}
		}
		return title + " [DLC " + name + "]", nil
	case KindDisc:
		d, err := NormalizeDisc(n.Label)
		if err != nil {
			return "", err
		}
		return title + " (Disc " + d + ")", nil
	}
	return "", &NameError{Field: "kind", Err: ErrInvalidName}
}

// FileName is Stem plus the lower-cased extension.
func (n ItemName) FileName(ext string) (string, error) {
	stem, err := n.Stem()
	if err != nil {
		return "", err
	}
	ext = strings.ToLower(ext)
	if !extensionPattern.MatchString(ext) {
		return "", &NameError{Field: "extension", Err: ErrInvalidName}
	}
	name := stem + ext
	if len(name) > maxNameBytes {
		return "", &NameError{Field: "title", Err: ErrNameTooLong}
	}
	return name, nil
}

// GameFolder is the game's folder name inside its console folder: the
// sanitized title (RF-11).
func GameFolder(title string) (string, error) {
	folder := SanitizeTitle(title)
	if folder == "" {
		return "", &NameError{Field: "title", Err: ErrInvalidName}
	}
	if len(folder) > maxNameBytes {
		return "", &NameError{Field: "title", Err: ErrNameTooLong}
	}
	return folder, nil
}

// CleanLabel normalizes an item's label for its kind: the version without
// "v" for updates, the trimmed name for DLC and nothing for the others.
func CleanLabel(kind ItemKind, label string) (string, error) {
	switch kind {
	case KindUpdate:
		return NormalizeVersion(label)
	case KindDisc:
		return NormalizeDisc(label)
	case KindDLC:
		name := strings.Join(strings.Fields(label), " ")
		if SanitizeTitle(name) == "" {
			return "", &NameError{Field: "label", Err: ErrInvalidName}
		}
		return name, nil
	}
	return "", nil
}
