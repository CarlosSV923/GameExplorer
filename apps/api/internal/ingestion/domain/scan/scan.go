// Package scan turns an extracted upload into reviewable items (spec RF-06):
// folder-format games stay whole, .cue sheets are grouped with their .bin
// tracks, junk is set aside, wrapper folders are irrelevant (items keep their
// relative path; the commit renames files) and every item gets a console
// suggestion from its content or, failing that, its name.
package scan

import (
	"bufio"
	"io"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/detection"
)

// junkExtensions are never game data.
var junkExtensions = map[string]bool{
	".txt": true, ".nfo": true, ".url": true, ".sfv": true, ".md5": true, ".sha1": true,
	".html": true, ".htm": true, ".lnk": true, ".db": true, ".ini": true, ".diz": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".pdf": true,
}

var cueFileLine = regexp.MustCompile(`(?i)^\s*FILE\s+(?:"([^"]+)"|(\S+))`)

// Scan inspects fsys (the job's staging directory) and returns its items,
// sorted by path.
func Scan(fsys fs.FS, id domain.JobID, profiles []detection.ConsoleProfile) ([]domain.StagedItem, error) {
	type file struct {
		path string
		size int64
	}
	var files []file
	var items []domain.StagedItem

	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != "." && (strings.HasPrefix(name, ".") || name == "__MACOSX") {
				return fs.SkipDir
			}
			if key := detection.SniffFolder(fsys, p); key != "" {
				items = append(items, folderItem(fsys, id, p, key, profiles))
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil // links and devices were already rejected after extraction
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if strings.HasPrefix(name, ".") || junkExtensions[strings.ToLower(path.Ext(name))] {
			items = append(items, domain.StagedItem{
				JobID: id, Shape: domain.ShapeFile, Path: p, Parts: []string{p}, Size: info.Size(),
				Ignored: true, Consoles: []string{}, Confidence: domain.ConfidenceNone,
			})
			return nil
		}
		files = append(files, file{path: p, size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Group each .cue with the tracks it references.
	sizes := map[string]int64{}
	byLower := map[string]string{}
	for _, f := range files {
		sizes[f.path] = f.size
		byLower[strings.ToLower(f.path)] = f.path
	}
	grouped := map[string]bool{}
	for _, f := range files {
		if strings.ToLower(path.Ext(f.path)) != ".cue" {
			continue
		}
		parts := []string{f.path}
		size := f.size
		for _, ref := range cueReferences(fsys, f.path) {
			if actual, ok := byLower[strings.ToLower(path.Join(path.Dir(f.path), ref))]; ok && actual != f.path {
				parts = append(parts, actual)
				size += sizes[actual]
				grouped[actual] = true
			}
		}
		grouped[f.path] = true
		items = append(items, discItem(fsys, id, parts, size, profiles))
	}

	for _, f := range files {
		if !grouped[f.path] {
			items = append(items, fileItem(fsys, id, f.path, f.size, profiles))
		}
	}

	slices.SortFunc(items, func(a, b domain.StagedItem) int { return strings.Compare(a.Path, b.Path) })
	return items, nil
}

func cueReferences(fsys fs.FS, cue string) []string {
	f, err := fsys.Open(cue)
	if err != nil {
		return nil
	}
	defer f.Close()
	var refs []string
	sc := bufio.NewScanner(io.LimitReader(f, 64<<10))
	for sc.Scan() {
		if m := cueFileLine.FindStringSubmatch(sc.Text()); m != nil {
			refs = append(refs, m[1]+m[2])
		}
	}
	return refs
}

func fileItem(fsys fs.FS, id domain.JobID, p string, size int64, profiles []detection.ConsoleProfile) domain.StagedItem {
	item := domain.StagedItem{JobID: id, Shape: domain.ShapeFile, Path: p, Parts: []string{p}, Size: size}
	applyName(&item, path.Base(p), profiles)
	applyHeader(&item, sniffPath(fsys, p, size), profiles)
	return item
}

func discItem(fsys fs.FS, id domain.JobID, parts []string, size int64, profiles []detection.ConsoleProfile) domain.StagedItem {
	item := domain.StagedItem{JobID: id, Shape: domain.ShapeDisc, Path: parts[0], Parts: parts, Size: size}
	applyName(&item, path.Base(parts[0]), profiles)
	if len(parts) > 1 { // the first track holds the ISO9660 file system
		applyHeader(&item, sniffPath(fsys, parts[1], 0), profiles)
	}
	if item.SuggestedKind == "" {
		item.SuggestedKind = detection.KindDisc
		if item.DiscNumber == 0 {
			item.DiscNumber = 1
		}
	}
	return item
}

func folderItem(fsys fs.FS, id domain.JobID, dir, key string, profiles []detection.ConsoleProfile) domain.StagedItem {
	var size int64
	_ = fs.WalkDir(fsys, dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	item := domain.StagedItem{
		JobID: id, Shape: domain.ShapeFolder, Path: dir, Parts: []string{dir}, Size: size,
		Consoles: []string{}, Confidence: domain.ConfidenceNone, SuggestedKind: detection.KindBase,
	}
	applyHeader(&item, key, profiles)
	return item
}

func applyName(item *domain.StagedItem, name string, profiles []detection.ConsoleProfile) {
	r := detection.FromName(name, profiles)
	item.Consoles = r.Consoles
	item.SuggestedKind = r.Kind
	item.TitleID, item.VersionCode, item.DisplayVersion, item.DiscNumber = r.TitleID, r.VersionCode, r.DisplayVersion, r.DiscNumber
	item.Confidence = domain.ConfidenceNone
	if len(r.Consoles) > 0 {
		item.Confidence = domain.ConfidenceExtension
	}
}

// applyHeader narrows the suggestion to one console when the content is conclusive.
func applyHeader(item *domain.StagedItem, key string, profiles []detection.ConsoleProfile) {
	if key == "" {
		return
	}
	for _, p := range profiles {
		if p.DetectorKey == key {
			item.Consoles = []string{p.Slug}
			item.Confidence = domain.ConfidenceHeader
			return
		}
	}
}

func sniffPath(fsys fs.FS, p string, size int64) string {
	f, err := fsys.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	ra, ok := f.(io.ReaderAt)
	if !ok {
		return ""
	}
	if size == 0 {
		if info, err := f.Stat(); err == nil {
			size = info.Size()
		}
	}
	return detection.SniffFile(ra, size)
}
