package domain

import (
	"regexp"
	"strings"
)

var cueFileLine = regexp.MustCompile(`(?i)^(\s*FILE\s+)(?:"([^"]*)"|(\S+))(.*)$`)

// RewriteCue replaces the file names referenced by FILE lines. rename maps a
// reference as written in the sheet to its new name; references it does not
// know are left untouched. Line endings are preserved.
func RewriteCue(sheet string, rename func(ref string) (string, bool)) string {
	lines := strings.SplitAfter(sheet, "\n")
	for i, line := range lines {
		body := strings.TrimRight(line, "\r\n")
		m := cueFileLine.FindStringSubmatch(body)
		if m == nil {
			continue
		}
		name, ok := rename(m[2] + m[3])
		if !ok {
			continue
		}
		lines[i] = m[1] + `"` + name + `"` + m[4] + line[len(body):]
	}
	return strings.Join(lines, "")
}
