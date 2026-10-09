package application

import (
	"context"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// unassignedDir is the unassigned folder's name in the library root (RF-27).
const unassignedDir = "_unassigned"

// settleTime is how long a file copied into the unassigned folder must stay
// unmodified before it joins the section (RF-26a).
const settleTime = time.Minute

// UnassignedView is an unassigned file with what the section shows.
type UnassignedView struct {
	domain.UnassignedFile
	// Consoles accept the file's extension.
	Consoles []string
	// Archive is a zip, 7z or rar (by name): it is validated once extracted.
	Archive bool
	// Copying: the file is still being copied over SMB (ID is 0).
	Copying bool
}

// UnassignedEntry is what the section lists (RF-27): a first-level folder
// with its files, or a loose file.
type UnassignedEntry struct {
	// ID is the lowest id of its files; 0 while every file is still copying.
	ID     domain.UnassignedID
	Name   string
	Folder bool
	Files  []UnassignedView
	Size   int64
	// Console and IGDBID prefill Asignar, when a file knows them.
	Console   domain.Slug
	IGDBID    *int64
	ArrivedAt time.Time
	// Copying: a file is still being copied; the entry admits no action.
	Copying bool
	// Busy: an assignment in progress is using the entry.
	Busy bool
}

var archiveName = regexp.MustCompile(`(?i)\.(zip|7z|rar|[0-9]{3})$`)

// seenFile is a file of the unassigned folder as the last read saw it.
type seenFile struct {
	size int64
	mod  time.Time
}

// errBusy refuses changes to files an assignment is using.
func errBusy() error {
	return reject(RejectConflict, "Una asignación en curso está usando estos archivos: termínala o cancélala antes.")
}

// EnsureUnassignedDir creates the unassigned folder if it is missing, so
// files can be copied into it over SMB from the start (RF-26a).
func (s *LibraryService) EnsureUnassignedDir() error {
	_, err := s.files.MkdirAll(s.files.UnassignedPath())
	return err
}

// Unassigned lists the unassigned section by entry, newest first. It reads
// the folder first: files copied into it over SMB show up without waiting
// for the scan (RF-26a).
func (s *LibraryService) Unassigned(ctx context.Context) ([]UnassignedEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copying, err := s.syncUnassigned(ctx)
	if err != nil {
		return nil, err
	}
	files, err := s.repo.UnassignedFiles(ctx)
	if err != nil {
		return nil, err
	}
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return nil, err
	}
	byKey := map[string]*UnassignedEntry{}
	var order []string
	add := func(v UnassignedView) {
		key := strings.ToLower(v.EntryKey())
		e, ok := byKey[key]
		if !ok {
			e = &UnassignedEntry{Name: v.EntryKey(), Folder: strings.Contains(v.Path, "/")}
			byKey[key] = e
			order = append(order, key)
		}
		e.Files = append(e.Files, v)
		e.Size += v.Size
		e.Copying = e.Copying || v.Copying
		e.Busy = e.Busy || v.Job != ""
		if v.ID != 0 && (e.ID == 0 || v.ID < e.ID) {
			e.ID = v.ID
		}
		if e.Console == "" {
			e.Console = v.Console
		}
		if e.IGDBID == nil {
			e.IGDBID = v.IGDBID
		}
		if v.ArrivedAt.After(e.ArrivedAt) {
			e.ArrivedAt = v.ArrivedAt
		}
	}
	for _, f := range files {
		add(UnassignedView{UnassignedFile: f, Consoles: Accepting(consoles, f.Name()), Archive: archiveName.MatchString(f.Name())})
	}
	now := s.now()
	for _, f := range copying {
		add(UnassignedView{
			UnassignedFile: domain.UnassignedFile{Path: f.Rel, Size: f.Size, ArrivedAt: now},
			Consoles:       Accepting(consoles, path.Base(f.Rel)), Archive: archiveName.MatchString(f.Rel), Copying: true,
		})
	}
	out := make([]UnassignedEntry, 0, len(order))
	for _, key := range order {
		e := byKey[key]
		slices.SortFunc(e.Files, func(a, b UnassignedView) int { return strings.Compare(a.Path, b.Path) })
		out = append(out, *e)
	}
	slices.SortStableFunc(out, func(a, b UnassignedEntry) int {
		if c := b.ArrivedAt.Compare(a.ArrivedAt); c != 0 {
			return c
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

// syncUnassigned compares the unassigned folder with the section (guarded
// by mu): settled new files join it, rows whose file is gone leave it, and
// the files still being copied are returned (RF-26a).
func (s *LibraryService) syncUnassigned(ctx context.Context) ([]TreeFile, error) {
	added, stale, copying, err := s.readUnassignedFolder(ctx)
	if err != nil || (len(added) == 0 && len(stale) == 0) {
		return copying, err
	}
	return copying, s.repo.Apply(ctx, "", func(tx domain.LibraryTx) error {
		for _, f := range added {
			if _, err := tx.InsertUnassigned(ctx, f); err != nil {
				return err
			}
		}
		for _, id := range stale {
			if err := tx.DeleteUnassigned(ctx, id); err != nil {
				return err
			}
		}
		return nil
	})
}

// readUnassignedFolder finds files copied into the unassigned folder over
// SMB (new rows, once settled), rows whose file is gone, and files still
// being copied: modified less than settleTime ago, or changed since the
// previous read.
func (s *LibraryService) readUnassignedFolder(ctx context.Context) ([]domain.UnassignedFile, []domain.UnassignedID, []TreeFile, error) {
	rows, err := s.repo.UnassignedFiles(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	byPath := map[string]domain.UnassignedID{}
	for _, r := range rows {
		byPath[strings.ToLower(r.Path)] = r.ID
	}
	exists, err := s.files.Exists(s.files.UnassignedPath())
	if err != nil {
		return nil, nil, nil, err
	}
	var tree []TreeFile
	if exists {
		if tree, err = s.files.Tree(s.files.UnassignedPath()); err != nil {
			return nil, nil, nil, err
		}
	}
	now := s.now()
	seen := map[string]seenFile{}
	var added []domain.UnassignedFile
	var copying []TreeFile
	present := map[string]bool{}
	for _, f := range tree {
		rel := filepath.ToSlash(f.Rel)
		if ignoredName(rel) {
			continue
		}
		key := strings.ToLower(rel)
		present[key] = true
		if _, ok := byPath[key]; ok {
			continue
		}
		cur := seenFile{f.Size, f.Modified}
		seen[key] = cur
		prev, wasSeen := s.unassignedSeen[key]
		if now.Sub(f.Modified) < settleTime || (wasSeen && (prev.size != cur.size || !prev.mod.Equal(cur.mod))) {
			f.Rel = rel
			copying = append(copying, f)
			continue
		}
		added = append(added, domain.UnassignedFile{
			Path: rel, Origin: unassignedDir + "/" + rel, Reason: domain.UnassignedSamba, Size: f.Size, ArrivedAt: now,
		})
	}
	s.unassignedSeen = seen
	var stale []domain.UnassignedID
	for key, id := range byPath {
		if !present[key] {
			stale = append(stale, id)
		}
	}
	return added, stale, copying, nil
}

// liveUnassigned returns a file of the section that is not in the trash
// nor used by an assignment.
func (s *LibraryService) liveUnassigned(ctx context.Context, id domain.UnassignedID) (domain.UnassignedFile, error) {
	f, err := s.repo.UnassignedByID(ctx, id)
	if err != nil {
		return f, err
	}
	if f.TrashEntry != nil {
		return f, domain.ErrUnassignedNotFound
	}
	if f.Job != "" {
		return f, errBusy()
	}
	return f, nil
}

// entryFiles returns the files of the entry a file belongs to (RF-27).
func (s *LibraryService) entryFiles(ctx context.Context, id domain.UnassignedID) ([]domain.UnassignedFile, error) {
	f, err := s.liveUnassigned(ctx, id)
	if err != nil {
		return nil, err
	}
	all, err := s.repo.UnassignedFiles(ctx)
	if err != nil {
		return nil, err
	}
	var out []domain.UnassignedFile
	for _, o := range all {
		if strings.EqualFold(o.EntryKey(), f.EntryKey()) {
			if o.Job != "" {
				return nil, errBusy()
			}
			out = append(out, o)
		}
	}
	slices.SortFunc(out, func(a, b domain.UnassignedFile) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

// UnassignedPath is the absolute path of rel ("/" separators) inside the
// unassigned folder; "" is the folder itself.
func (s *LibraryService) UnassignedPath(rel string) string {
	if rel == "" {
		return s.files.UnassignedPath()
	}
	return s.files.UnassignedPath(filepath.FromSlash(rel))
}

// UnassignedFile returns the absolute path of a file of the section.
func (s *LibraryService) UnassignedFile(ctx context.Context, id domain.UnassignedID) (string, domain.UnassignedFile, error) {
	f, err := s.liveUnassigned(ctx, id)
	if err != nil {
		return "", f, err
	}
	return s.files.UnassignedPath(filepath.FromSlash(f.Path)), f, nil
}

// UnassignedEntryDownload prepares an entry's download: the file itself, or
// the folder as a zip (RF-27).
func (s *LibraryService) UnassignedEntryDownload(ctx context.Context, id domain.UnassignedID) (Download, error) {
	files, err := s.entryFiles(ctx, id)
	if err != nil {
		return Download{}, err
	}
	var entries []ZipFile
	for _, f := range files {
		tree, err := s.files.Tree(s.files.UnassignedPath(filepath.FromSlash(f.Path)))
		if err != nil || len(tree) != 1 {
			return Download{}, reject(RejectConflict, "Falta %q en la carpeta de No asignados.", f.Path)
		}
		entries = append(entries, ZipFile{Name: f.Path, TreeFile: tree[0]})
	}
	if len(files) == 1 && !strings.Contains(files[0].Path, "/") {
		return Download{Name: files[0].Name(), File: &entries[0].TreeFile}, nil
	}
	return Download{Name: files[0].EntryKey() + ".zip", Entries: entries}, nil
}

// DeleteUnassigned deletes a file of the section for good (RF-27).
func (s *LibraryService) DeleteUnassigned(ctx context.Context, id domain.UnassignedID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := s.liveUnassigned(ctx, id)
	if err != nil {
		return err
	}
	return s.deleteUnassigned(ctx, f)
}

// DeleteUnassignedEntry deletes every file of an entry for good (RF-27).
func (s *LibraryService) DeleteUnassignedEntry(ctx context.Context, id domain.UnassignedID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := s.entryFiles(ctx, id)
	if err != nil {
		return err
	}
	return s.deleteUnassigned(ctx, files...)
}

func (s *LibraryService) deleteUnassigned(ctx context.Context, files ...domain.UnassignedFile) error {
	if err := s.repo.Apply(ctx, "", func(tx domain.LibraryTx) error {
		for _, f := range files {
			if err := tx.DeleteUnassigned(ctx, f.ID); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, f := range files {
		if err := s.files.RemoveAll(s.files.UnassignedPath(filepath.FromSlash(f.Path))); err != nil {
			s.log.Warn("delete unassigned file", "path", f.Path, "error", err)
		}
		s.pruneUnassigned(f.Path)
	}
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
	return s.trashUnassigned(ctx, f)
}

// TrashUnassignedEntry sends an entry to the trash as one trash entry (RF-27).
func (s *LibraryService) TrashUnassignedEntry(ctx context.Context, id domain.UnassignedID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := s.entryFiles(ctx, id)
	if err != nil {
		return err
	}
	return s.trashUnassigned(ctx, files...)
}

func (s *LibraryService) trashUnassigned(ctx context.Context, files ...domain.UnassignedFile) error {
	b := s.newOp("trash")
	dir := b.newTrashDir()
	for _, f := range files {
		b.mkdirsFor(s.files.TrashPath(dir), f.Path)
		if err := b.place(s.files.UnassignedPath(), f.Path, s.files.TrashPath(dir), f.Path, true); err != nil {
			return err
		}
		b.pruneParents(s.files.UnassignedPath(), f.Path)
	}
	now := s.now()
	return s.run(ctx, b, func(tx domain.LibraryTx) error {
		entry, err := tx.InsertTrashEntry(ctx, domain.TrashEntry{Reason: domain.TrashDeleted, Dir: dir, TrashedAt: now})
		if err != nil {
			return err
		}
		for _, f := range files {
			if err := tx.SetUnassignedPlace(ctx, f.ID, f.Path, &entry); err != nil {
				return err
			}
		}
		return nil
	})
}

// TakenEntry is an entry an assignment now uses (RF-27a).
type TakenEntry struct {
	Name    string
	Folder  bool
	Console domain.Slug
	IGDBID  *int64
	Files   []domain.UnassignedFile
}

// TakeEntry marks every file of the entry a file belongs to as used by the
// assignment job: they stay in the section, without actions, until the job
// stores them or ends (RF-27a).
func (s *LibraryService) TakeEntry(ctx context.Context, id domain.UnassignedID, job string) (TakenEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	files, err := s.entryFiles(ctx, id)
	if err != nil {
		return TakenEntry{}, err
	}
	t := TakenEntry{Name: files[0].EntryKey(), Folder: strings.Contains(files[0].Path, "/"), Files: files}
	for _, f := range files {
		if t.Console == "" {
			t.Console = f.Console
		}
		if t.IGDBID == nil {
			t.IGDBID = f.IGDBID
		}
		if err := s.repo.SetUnassignedJob(ctx, f.ID, job); err != nil {
			return t, err
		}
	}
	return t, nil
}

// JobFiles returns the files an assignment uses, with their absolute paths.
func (s *LibraryService) JobFiles(ctx context.Context, job string) ([]domain.UnassignedFile, error) {
	all, err := s.repo.UnassignedFiles(ctx)
	if err != nil {
		return nil, err
	}
	var out []domain.UnassignedFile
	for _, f := range all {
		if f.Job == job {
			out = append(out, f)
		}
	}
	slices.SortFunc(out, func(a, b domain.UnassignedFile) int { return strings.Compare(a.Path, b.Path) })
	return out, nil
}

// ReleaseJob frees the files an assignment was using.
func (s *LibraryService) ReleaseJob(ctx context.Context, job string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.repo.ReleaseUnassignedJob(ctx, job)
}

// DeleteJobFiles deletes for good files an assignment extracted and no
// longer needs (its archives, RF-06).
func (s *LibraryService) DeleteJobFiles(ctx context.Context, job string, ids []domain.UnassignedID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var files []domain.UnassignedFile
	for _, id := range ids {
		f, err := s.repo.UnassignedByID(ctx, id)
		if err != nil {
			return err
		}
		if f.Job != job || f.TrashEntry != nil {
			return domain.ErrUnassignedNotFound
		}
		files = append(files, f)
	}
	return s.deleteUnassigned(ctx, files...)
}

// SetAside is a request to put files that did not become game files into
// the unassigned section (or the trash, restorable there).
type SetAside struct {
	Source string
	// Root is the absolute directory Files are relative to ("/" separators).
	Root  string
	Files []string
	// Folder groups the files in the section ("" keeps their paths as they
	// are); an existing folder with that name, in any case, is reused.
	Folder string
	Origin string
	Reason domain.UnassignedReason
	// Console and IGDBID prefill Asignar later (both optional).
	Console domain.Slug
	IGDBID  *int64
	// ToTrash sends them to the trash instead.
	ToTrash bool
}

// PutAside moves files into the unassigned section, or the trash (RF-07,
// RF-07b, RF-27). Names taken in the section get a " (2)" suffix.
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
	folder, err := s.entryFolder(safeFolder(req.Folder))
	if err != nil {
		return err
	}
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
				Console: req.Console, IGDBID: req.IGDBID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// entryFolder returns the existing first-level folder of the section that
// matches folder ignoring case (the share does not tell them apart), or
// folder itself.
func (s *LibraryService) entryFolder(folder string) (string, error) {
	if folder == "" {
		return "", nil
	}
	first, rest, _ := strings.Cut(folder, "/")
	names, err := s.files.List(s.files.UnassignedPath())
	if err != nil {
		return "", err
	}
	for _, n := range names {
		if strings.EqualFold(n, first) {
			return path.Join(n, rest), nil
		}
	}
	return folder, nil
}

// UnassignGame moves every file of a game to the unassigned section, under
// "<game folder>/" (RF-24).
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
	folder, err := s.entryFolder(g.Folder)
	if err != nil {
		return err
	}
	var out []placed
	for _, it := range items {
		rel, err := b.freeName(s.files.UnassignedPath(), path.Join(folder, it.File))
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
				Console: g.Console, IGDBID: g.IGDBID,
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
// section ("Zelda: TP" → "Zelda - TP").
func safeFolder(folder string) string {
	var segs []string
	for seg := range strings.SplitSeq(folder, "/") {
		if s := domain.SanitizeTitle(seg); s != "" && s != "." && s != ".." {
			segs = append(segs, s)
		}
	}
	return strings.Join(segs, "/")
}

// FlattenConsoleFolders moves the files that earlier scans left under a
// console folder of the unassigned section ("switch/Zelda/z.nsp") out of
// it ("Zelda/z.nsp"), remembering the console, as scans do now (RF-26):
// otherwise a whole console folder would be one entry of mixed games.
func (s *LibraryService) FlattenConsoleFolders(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return 0, err
	}
	isConsole := map[string]bool{}
	for _, c := range consoles {
		isConsole[strings.ToLower(string(c.Slug))] = true
	}
	files, err := s.repo.UnassignedFiles(ctx)
	if err != nil {
		return 0, err
	}
	b := s.newOp("flatten")
	type moved struct {
		file    domain.UnassignedFile
		rel     string
		console domain.Slug
	}
	var moves []moved
	for _, f := range files {
		first, rest, ok := strings.Cut(f.Path, "/")
		if !ok || f.Job != "" || !isConsole[strings.ToLower(first)] {
			continue
		}
		target, err := b.freeName(s.files.UnassignedPath(), rest)
		if err != nil {
			return 0, err
		}
		b.mkdirsFor(s.files.UnassignedPath(), target)
		if err := b.place(s.files.UnassignedPath(), f.Path, s.files.UnassignedPath(), target, false); err != nil {
			return 0, err
		}
		b.pruneParents(s.files.UnassignedPath(), f.Path)
		console := f.Console
		if console == "" {
			console = domain.Slug(strings.ToLower(first))
		}
		moves = append(moves, moved{f, target, console})
	}
	if len(moves) == 0 {
		return 0, nil
	}
	err = s.run(ctx, b, func(tx domain.LibraryTx) error {
		for _, m := range moves {
			if err := tx.DeleteUnassigned(ctx, m.file.ID); err != nil {
				return err
			}
			f := m.file
			f.Path, f.Console = m.rel, m.console
			if _, err := tx.InsertUnassigned(ctx, f); err != nil {
				return err
			}
		}
		return nil
	})
	return len(moves), err
}
