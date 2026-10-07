// Package detection suggests a console and an item kind for a file, from its
// name (contracts/detection-cases.json) and from its content (magic bytes).
// It is pure domain logic: inputs are names, byte readers and fs.FS views.
package detection

import (
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ItemKind is what a file is within a game (mirrors the API's ItemKind).
type ItemKind string

// Item kinds.
const (
	KindBase   ItemKind = "base"
	KindUpdate ItemKind = "update"
	KindDLC    ItemKind = "dlc"
	KindDisc   ItemKind = "disc"
)

// ConsoleProfile is what detection needs to know about a console.
type ConsoleProfile struct {
	Slug       string
	Extensions []string
	// DetectorKey links the console to a header detector ("" for consoles
	// added by the user, which are detected by extension only).
	DetectorKey string
}

// NameResult is what a file name alone suggests.
type NameResult struct {
	// Consoles are the candidate console slugs (a set; empty means unknown).
	Consoles       []string
	Kind           ItemKind
	TitleID        string
	VersionCode    string
	DisplayVersion string
	DiscNumber     int
}

var (
	titleIDPattern     = regexp.MustCompile(`\[([0-9A-Fa-f]{16})\]`)
	versionCodePattern = regexp.MustCompile(`\[v(\d+)\]`)
	// "[1.0.3]" or "Update 1.0.4" / "Update v1.0.4".
	displayVersionPattern = regexp.MustCompile(`(?i)(?:\[(\d+(?:\.\d+){1,3})\]|update\s*v?(\d+(?:\.\d+){1,3}))`)
	discPattern           = regexp.MustCompile(`(?i)\(\s*disc\s*(\d{1,2})\s*\)`)
)

// FromName applies the file-name rules (spec §6). Matching is case-insensitive.
func FromName(fileName string, profiles []ConsoleProfile) NameResult {
	var res NameResult
	ext := strings.ToLower(path.Ext(fileName))
	if ext != "" {
		for _, p := range profiles {
			if slices.Contains(p.Extensions, ext) {
				res.Consoles = append(res.Consoles, p.Slug)
			}
		}
	}
	if res.Consoles == nil {
		res.Consoles = []string{}
	}

	if m := titleIDPattern.FindStringSubmatch(fileName); m != nil {
		res.TitleID = strings.ToUpper(m[1])
		res.Kind = switchKind(res.TitleID)
	}
	if m := versionCodePattern.FindStringSubmatch(fileName); m != nil {
		res.VersionCode = m[1]
	}
	if m := displayVersionPattern.FindStringSubmatch(fileName); m != nil {
		res.DisplayVersion = m[1] + m[2]
	}
	if m := discPattern.FindStringSubmatch(fileName); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			res.DiscNumber = n
			if res.Kind == "" {
				res.Kind = KindDisc
			}
		}
	}
	return res
}

// switchKind reads a Nintendo Switch title id: base games end in 000,
// updates in 800, anything else is add-on content.
func switchKind(titleID string) ItemKind {
	switch {
	case strings.HasSuffix(titleID, "000"):
		return KindBase
	case strings.HasSuffix(titleID, "800"):
		return KindUpdate
	default:
		return KindDLC
	}
}
