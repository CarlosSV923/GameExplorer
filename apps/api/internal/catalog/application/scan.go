package application

import (
	"context"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// ScanReport summarizes a library scan (RF-26).
type ScanReport struct {
	ScannedAt time.Time
	// Unassigned is how many unknown files moved to (or were found in) the
	// unassigned section.
	Unassigned int
	// Removed is how many files deleted over SMB left the library.
	Removed int
	// Pending is how many unknown files are still changing (a copy in
	// progress): the next scan moves them.
	Pending int
}

// appDir holds the app's own data inside the library (staging, trash,
// scratch); the scan never looks inside.
const appDir = ".gameexplorer"

// ignoredName reports files that SMB clients and NAS tools create on their
// own (Finder's .DS_Store, Windows' Thumbs.db…): they are neither games nor
// worth moving.
func ignoredName(rel string) bool {
	for seg := range strings.SplitSeq(rel, "/") {
		if strings.HasPrefix(seg, ".") || strings.HasPrefix(seg, "@") || strings.HasPrefix(seg, "#") || strings.HasPrefix(seg, "$") {
			return true
		}
	}
	switch strings.ToLower(path.Base(rel)) {
	case "thumbs.db", "desktop.ini":
		return true
	}
	return false
}

// Scan compares the library with what the app knows (RF-26): files deleted
// over SMB leave the library; unknown files in the console folders or the
// root move to the unassigned section, keeping their path, once a later
// scan finds them unchanged; files copied straight into the unassigned
// folder join the section.
func (s *LibraryService) Scan(ctx context.Context) (ScanReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rep := ScanReport{ScannedAt: s.now()}

	known, err := s.repo.LibraryFiles(ctx)
	if err != nil {
		return rep, err
	}
	byPath := map[string]domain.LibraryFile{}
	for _, f := range known {
		byPath[strings.ToLower(path.Join(string(f.Console), f.Folder, f.File))] = f
	}
	pendingList, err := s.repo.PendingFiles(ctx)
	if err != nil {
		return rep, err
	}
	pending := map[string]domain.PendingFile{}
	for _, p := range pendingList {
		pending[p.Path] = p
	}

	roots, err := s.files.List(s.files.LibraryPath())
	if err != nil {
		return rep, err
	}
	seen := map[string]bool{}
	var stable []TreeFile
	var stillPending []domain.PendingFile
	for _, name := range roots {
		if name == appDir || name == unassignedDir || ignoredName(name) {
			continue
		}
		tree, err := s.files.Tree(s.files.LibraryPath(name))
		if err != nil {
			return rep, err
		}
		for _, f := range tree {
			rel := name
			if f.Rel != "" {
				rel = name + "/" + f.Rel
			}
			if ignoredName(rel) {
				continue
			}
			if _, ok := byPath[strings.ToLower(rel)]; ok {
				seen[strings.ToLower(rel)] = true
				continue
			}
			p, ok := pending[rel]
			if ok && p.Size == f.Size && p.ModTime.Equal(f.Modified) {
				f.Rel = rel
				stable = append(stable, f)
				continue
			}
			stillPending = append(stillPending, domain.PendingFile{Path: rel, Size: f.Size, ModTime: f.Modified})
		}
	}

	// An unmounted or emptied dataset must not wipe the catalog.
	var gone []domain.LibraryFile
	for key, f := range byPath {
		if !seen[key] {
			gone = append(gone, f)
		}
	}
	if len(roots) == 0 && len(gone) > 0 {
		s.log.Error("library root is empty: not removing files from the catalog", "known", len(gone))
		gone = nil
	}

	newRows, staleRows, err := s.syncUnassignedFolder(ctx)
	if err != nil {
		return rep, err
	}

	b := s.newOp("scan")
	type moved struct {
		rel  string
		from string
		size int64
	}
	var moves []moved
	for _, f := range stable {
		target, err := b.freeName(s.files.UnassignedPath(), f.Rel)
		if err != nil {
			return rep, err
		}
		b.mkdirsFor(s.files.UnassignedPath(), target)
		if err := b.place(s.files.LibraryPath(), f.Rel, s.files.UnassignedPath(), target, false); err != nil {
			return rep, err
		}
		b.pruneParents(s.files.LibraryPath(), f.Rel)
		moves = append(moves, moved{target, f.Rel, f.Size})
	}
	for _, f := range gone {
		b.pruneGame(f.Console, f.Folder)
	}
	now := s.now()
	err = s.run(ctx, b, func(tx domain.LibraryTx) error {
		for _, m := range moves {
			if _, err := tx.InsertUnassigned(ctx, domain.UnassignedFile{
				Path: m.rel, Origin: m.from, Reason: domain.UnassignedSamba, Size: m.size, ArrivedAt: now,
			}); err != nil {
				return err
			}
		}
		for _, f := range newRows {
			if _, err := tx.InsertUnassigned(ctx, f); err != nil {
				return err
			}
		}
		for _, id := range staleRows {
			if err := tx.DeleteUnassigned(ctx, id); err != nil {
				return err
			}
		}
		for _, f := range gone {
			if err := tx.DeleteItem(ctx, f.Item); err != nil {
				return err
			}
			if err := tx.DeleteGameIfEmpty(ctx, f.Game); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return rep, err
	}
	if err := s.repo.SetPendingFiles(ctx, stillPending); err != nil {
		return rep, err
	}
	rep.Unassigned, rep.Removed, rep.Pending = len(moves)+len(newRows), len(gone), len(stillPending)
	s.lastScan = &rep
	if rep.Unassigned > 0 || rep.Removed > 0 {
		s.log.Info("library scan", "unassigned", rep.Unassigned, "removed", rep.Removed, "pending", rep.Pending)
	}
	return rep, nil
}

// syncUnassignedFolder finds files copied into the unassigned folder over
// SMB (new rows) and rows whose file is gone.
func (s *LibraryService) syncUnassignedFolder(ctx context.Context) ([]domain.UnassignedFile, []domain.UnassignedID, error) {
	rows, err := s.repo.UnassignedFiles(ctx)
	if err != nil {
		return nil, nil, err
	}
	byPath := map[string]domain.UnassignedID{}
	for _, r := range rows {
		byPath[strings.ToLower(r.Path)] = r.ID
	}
	exists, err := s.files.Exists(s.files.UnassignedPath())
	if err != nil {
		return nil, nil, err
	}
	var tree []TreeFile
	if exists {
		if tree, err = s.files.Tree(s.files.UnassignedPath()); err != nil {
			return nil, nil, err
		}
	}
	now := s.now()
	var added []domain.UnassignedFile
	present := map[string]bool{}
	for _, f := range tree {
		rel := filepath.ToSlash(f.Rel)
		if ignoredName(rel) {
			continue
		}
		key := strings.ToLower(rel)
		present[key] = true
		if _, ok := byPath[key]; !ok {
			added = append(added, domain.UnassignedFile{
				Path: rel, Origin: unassignedDir + "/" + rel, Reason: domain.UnassignedSamba, Size: f.Size, ArrivedAt: now,
			})
		}
	}
	var stale []domain.UnassignedID
	for key, id := range byPath {
		if !present[key] {
			stale = append(stale, id)
		}
	}
	return added, stale, nil
}

// LastScan returns the last scan, or nil before the first.
func (s *LibraryService) LastScan() *ScanReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastScan == nil {
		return nil
	}
	rep := *s.lastScan
	return &rep
}

// RunMaintenance scans the library and purges the expired trash at start
// and then every interval, until ctx ends.
func (s *LibraryService) RunMaintenance(ctx context.Context, interval, retention time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if _, err := s.Scan(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("library scan failed", "error", err)
		}
		if n, err := s.PurgeExpired(ctx, retention); err != nil && ctx.Err() == nil {
			s.log.Warn("trash purge failed", "error", err)
		} else if n > 0 {
			s.log.Info("trash purged", "entries", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
