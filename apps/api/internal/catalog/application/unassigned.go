package application

import (
	"context"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// unassignedDir is the unassigned folder's name in the library root (RF-27).
const unassignedDir = "_unassigned"

// UnassignedView is an unassigned file with what the section shows.
type UnassignedView struct {
	domain.UnassignedFile
	// Consoles accept the file's extension.
	Consoles []string
	// Archive is a zip, 7z or rar (by name): it is validated once extracted.
	Archive bool
}

var archiveName = regexp.MustCompile(`(?i)\.(zip|7z|rar|[0-9]{3})$`)

// Unassigned lists the unassigned section, newest first.
func (s *LibraryService) Unassigned(ctx context.Context) ([]UnassignedView, error) {
	files, err := s.repo.UnassignedFiles(ctx)
	if err != nil {
		return nil, err
	}
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]UnassignedView, 0, len(files))
	for _, f := range files {
		out = append(out, UnassignedView{
			UnassignedFile: f,
			Consoles:       Accepting(consoles, f.Name()),
			Archive:        archiveName.MatchString(f.Name()),
		})
	}
	return out, nil
}

// liveUnassigned returns a file of the section that is not in the trash.
func (s *LibraryService) liveUnassigned(ctx context.Context, id domain.UnassignedID) (domain.UnassignedFile, error) {
	f, err := s.repo.UnassignedByID(ctx, id)
	if err != nil {
		return f, err
	}
	if f.TrashEntry != nil {
		return f, domain.ErrUnassignedNotFound
	}
	return f, nil
}

// UnassignedFile returns the absolute path of a file of the section.
func (s *LibraryService) UnassignedFile(ctx context.Context, id domain.UnassignedID) (string, domain.UnassignedFile, error) {
	f, err := s.liveUnassigned(ctx, id)
	if err != nil {
		return "", f, err
	}
	return s.files.UnassignedPath(filepath.FromSlash(f.Path)), f, nil
}

// DeleteUnassigned deletes a file of the section for good (RF-27).
func (s *LibraryService) DeleteUnassigned(ctx context.Context, id domain.UnassignedID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.liveUnassigned(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Apply(ctx, "", func(tx domain.LibraryTx) error { return tx.DeleteUnassigned(ctx, f.ID) }); err != nil {
		return err
	}
	if err := s.files.RemoveAll(s.files.UnassignedPath(filepath.FromSlash(f.Path))); err != nil {
		s.log.Warn("delete unassigned file", "path", f.Path, "error", err)
	}
	s.pruneUnassigned(f.Path)
	return nil
}

func (s *LibraryService) pruneUnassigned(rel string) {
	for d := path.Dir(rel); d != "." && d != "/"; d = path.Dir(d) {
		if err := s.files.RemoveEmptyDir(s.files.UnassignedPath(filepath.FromSlash(d))); err != nil {
			s.log.Warn("remove empty folder", "dir", d, "error", err)
		}
	}
}

// TrashUnassigned sends a file of the section to the trash (RF-27).
func (s *LibraryService) TrashUnassigned(ctx context.Context, id domain.UnassignedID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.liveUnassigned(ctx, id)
	if err != nil {
		return err
	}
	b := s.newOp("trash")
	dir := b.newTrashDir()
	b.mkdirsFor(s.files.TrashPath(dir), f.Path)
	if err := b.place(s.files.UnassignedPath(), f.Path, s.files.TrashPath(dir), f.Path, true); err != nil {
		return err
	}
	b.pruneParents(s.files.UnassignedPath(), f.Path)
	now := s.now()
	return s.run(ctx, b, func(tx domain.LibraryTx) error {
		entry, err := tx.InsertTrashEntry(ctx, domain.TrashEntry{Reason: domain.TrashDeleted, Dir: dir, TrashedAt: now})
		if err != nil {
			return err
		}
		return tx.SetUnassignedPlace(ctx, f.ID, f.Path, &entry)
	})
}

// TakeUnassigned moves a file of the section to dest (an absolute path in
// the staging area) and forgets it: an upload job now owns it (RF-27).
func (s *LibraryService) TakeUnassigned(ctx context.Context, id domain.UnassignedID, dest string) (domain.UnassignedFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.liveUnassigned(ctx, id)
	if err != nil {
		return f, err
	}
	b := s.newOp("assign")
	b.mkdir(filepath.Dir(dest))
	if err := b.place(s.files.UnassignedPath(), f.Path, filepath.Dir(dest), filepath.Base(dest), true); err != nil {
		return f, err
	}
	b.pruneParents(s.files.UnassignedPath(), f.Path)
	return f, s.run(ctx, b, func(tx domain.LibraryTx) error { return tx.DeleteUnassigned(ctx, f.ID) })
}

// SetAside is a request to put files that did not become game files into
// the unassigned section (or the trash, restorable there).
type SetAside struct {
	Source string
	// Root is the absolute directory Files are relative to ("/" separators).
	Root  string
	Files []string
	// Folder groups the files in the section ("" keeps their paths as they are).
	Folder string
	Origin string
	Reason domain.UnassignedReason
	// ToTrash sends them to the trash instead.
	ToTrash bool
}

// PutAside moves files into the unassigned section, or the trash (RF-07,
// RF-27). Names taken in the section get a " (2)" suffix.
func (s *LibraryService) PutAside(ctx context.Context, req SetAside) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b := s.newOp(req.Source)
	type placed struct {
		rel  string
		size int64
	}
	var out []placed
	var trashDir string
	if req.ToTrash {
		trashDir = b.newTrashDir()
	}
	folder := safeFolder(req.Folder)
	for _, rel := range req.Files {
		tree, err := s.files.Tree(filepath.Join(req.Root, filepath.FromSlash(rel)))
		if err != nil {
			return err
		}
		var size int64
		for _, t := range tree {
			size += t.Size
		}
		target := rel
		if folder != "" {
			target = path.Join(folder, rel)
		}
		destDir := s.files.UnassignedPath()
		if req.ToTrash {
			destDir = s.files.TrashPath(trashDir)
		} else if target, err = b.freeName(destDir, target); err != nil {
			return err
		}
		b.mkdirsFor(destDir, target)
		if err := b.place(req.Root, rel, destDir, target, true); err != nil {
			return err
		}
		out = append(out, placed{target, size})
	}
	now := s.now()
	return s.run(ctx, b, func(tx domain.LibraryTx) error {
		var entry *domain.TrashEntryID
		if req.ToTrash {
			id, err := tx.InsertTrashEntry(ctx, domain.TrashEntry{Reason: domain.TrashDeleted, Dir: trashDir, TrashedAt: now})
			if err != nil {
				return err
			}
			entry = &id
		}
		for _, p := range out {
			if _, err := tx.InsertUnassigned(ctx, domain.UnassignedFile{
				Path: p.rel, Origin: req.Origin, Reason: req.Reason, Size: p.size, ArrivedAt: now, TrashEntry: entry,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// UnassignGame moves every file of a game to the unassigned section, under
// "<console>/<game folder>/" (RF-24).
func (s *LibraryService) UnassignGame(ctx context.Context, id domain.GameID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, err := s.repo.GameByID(ctx, id)
	if err != nil {
		return err
	}
	items := g.LiveItems()
	if len(items) == 0 {
		return domain.ErrGameNotFound
	}
	return s.unassign(ctx, g, items...)
}

// UnassignItem moves one file of a game to the unassigned section (RF-24).
func (s *LibraryService) UnassignItem(ctx context.Context, id domain.ItemID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, err := s.repo.ItemByID(ctx, id)
	if err != nil {
		return err
	}
	if !it.Live() {
		return domain.ErrItemNotFound
	}
	g, err := s.repo.GameByID(ctx, it.GameID)
	if err != nil {
		return err
	}
	return s.unassign(ctx, g, it)
}

func (s *LibraryService) unassign(ctx context.Context, g *domain.Game, items ...domain.GameItem) error {
	b := s.newOp("unassign")
	type placed struct {
		item domain.GameItem
		rel  string
	}
	var out []placed
	for _, it := range items {
		rel, err := b.freeName(s.files.UnassignedPath(), path.Join(string(g.Console), g.Folder, it.File))
		if err != nil {
			return err
		}
		b.mkdirsFor(s.files.UnassignedPath(), rel)
		if err := b.place(s.gameDir(g), it.File, s.files.UnassignedPath(), rel, true); err != nil {
			return err
		}
		out = append(out, placed{it, rel})
	}
	b.pruneGame(g.Console, g.Folder)
	now := s.now()
	return s.run(ctx, b, func(tx domain.LibraryTx) error {
		for _, p := range out {
			if _, err := tx.InsertUnassigned(ctx, domain.UnassignedFile{
				Path: p.rel, Origin: strings.Join([]string{string(g.Console), g.Folder, p.item.File}, "/"),
				Reason: domain.UnassignedManual, Size: p.item.Size, ArrivedAt: now,
			}); err != nil {
				return err
			}
			if err := tx.DeleteItem(ctx, p.item.ID); err != nil {
				return err
			}
		}
		return tx.DeleteGameIfEmpty(ctx, g.ID)
	})
}

// safeFolder sanitizes each segment of a folder path for the unassigned
// section ("wii/Zelda: TP" → "wii/Zelda - TP").
func safeFolder(folder string) string {
	var segs []string
	for seg := range strings.SplitSeq(folder, "/") {
		if s := domain.SanitizeTitle(seg); s != "" && s != "." && s != ".." {
			segs = append(segs, s)
		}
	}
	return strings.Join(segs, "/")
}
