package domain

import (
	"regexp"
	"strconv"
	"strings"
)

// Volume describes one part of a multi-volume archive.
type Volume struct {
	// Set identifies the archive the part belongs to (lower-case base name
	// plus naming style), so parts uploaded separately find each other.
	Set string
	// Index is 1 for the first volume.
	Index int
}

var (
	// game.part1.rar, game.part01.rar, game.part001.rar (RAR 3+ naming).
	partStyle = regexp.MustCompile(`(?i)^(.+)\.part(\d{1,4})\.rar$`)
	// game.7z.001, game.zip.002, game.rar.001, game.iso.001 (split files).
	numberedStyle = regexp.MustCompile(`(?i)^(.+\.[a-z0-9]{1,4})\.(\d{3})$`)
	// game.r00, game.r01: the pre-RAR-3 naming, whose first volume is a plain
	// .rar that cannot be told apart from a single archive.
	legacyRarStyle = regexp.MustCompile(`(?i)\.r\d{2}$`)
)

// ParseVolume recognizes multi-volume archive parts by name.
func ParseVolume(fileName string) (Volume, bool) {
	if m := partStyle.FindStringSubmatch(fileName); m != nil {
		n, _ := strconv.Atoi(m[2])
		return Volume{Set: strings.ToLower(m[1]) + "|rar-part", Index: n}, n > 0
	}
	if m := numberedStyle.FindStringSubmatch(fileName); m != nil {
		n, _ := strconv.Atoi(m[2])
		return Volume{Set: strings.ToLower(m[1]) + "|numbered", Index: n}, n > 0
	}
	return Volume{}, false
}

// IsLegacyRarVolume reports old-style RAR volumes (.r00, .r01…), which are
// not supported: the first one is indistinguishable from a single .rar.
func IsLegacyRarVolume(fileName string) bool {
	return legacyRarStyle.MatchString(fileName)
}
