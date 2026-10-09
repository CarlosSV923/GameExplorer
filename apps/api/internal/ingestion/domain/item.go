package domain

import (
	"context"
	"path"
	"slices"
	"strings"
)

// StagedFile is a file found in an upload (RF-07).
type StagedFile struct {
	JobID JobID
	// Path is relative to the job's staging directory ("/" separators).
	Path string
	Size int64
	// Unassigned is set for a file of an assigned unassigned entry that is
	// not in staging: it stays in the section (Path is relative to the
	// unassigned folder) until it is stored (RF-27a).
	Unassigned *int64
}

// StagedFileRepository persists the files found in each job.
type StagedFileRepository interface {
	// Replace stores files as the full set for the job (re-extraction safe).
	Replace(ctx context.Context, id JobID, files []StagedFile) error
	List(ctx context.Context, id JobID) ([]StagedFile, error)
}

// ConsoleRule is what validation needs to know about a console.
type ConsoleRule struct {
	Slug string
	// Extensions are every extension the console accepts.
	Extensions []string
	// MultipleFiles allows several game files in one upload (Switch).
	MultipleFiles bool
}

// ExtensionOf returns the longest of known that the file name ends with,
// or "" (the same rule as the catalog's, RF-41).
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

// Validate splits an upload's files into game files of the console and the
// rest (discarded), and says why the upload does not fit, if it does not
// (RF-07). known is every extension of every console: "Game.nkit.iso" is a
// .nkit.iso even for a console that accepts .iso.
func Validate(rule ConsoleRule, files []StagedFile, known []string) ([]StagedFile, InvalidReason) {
	var valid []StagedFile
	for _, f := range files {
		if ext := ExtensionOf(path.Base(f.Path), known); ext != "" && slices.Contains(rule.Extensions, ext) {
			valid = append(valid, f)
		}
	}
	switch {
	case len(valid) == 0:
		return nil, InvalidNone
	case len(valid) > 1 && !rule.MultipleFiles:
		return valid, InvalidMany
	}
	return valid, ""
}

// ValidateFor is Validate for a job: an assigned unassigned entry may hold
// several game files of a one-file console, and the user keeps one of them
// in the confirmation (RF-27a).
func ValidateFor(job *UploadJob, rule ConsoleRule, files []StagedFile, known []string) ([]StagedFile, InvalidReason) {
	valid, reason := Validate(rule, files, known)
	if reason == InvalidMany && job.FromEntry() {
		reason = ""
	}
	return valid, reason
}
