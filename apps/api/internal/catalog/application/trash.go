package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// TrashView is a trash entry with what the trash screen shows.
type TrashView struct {
	domain.TrashEntry
	// Game is set for entries of game files.
	Game      *domain.Game
	Size      int64
	ExpiresAt time.Time
}

// trashItems records game files as one trash entry (inside a transaction).
func trashItems(ctx context.Context, tx domain.LibraryTx, game domain.GameID, reason domain.TrashReason, whole bool, dir string, now time.Time, items ...domain.ItemID) error {
	entry, err := tx.InsertTrashEntry(ctx, domain.TrashEntry{GameID: &game, WholeGame: whole, Reason: reason, Dir: dir, TrashedAt: now})
	if err != nil {
		return err
	}
	for _, id := range items {
		if err := tx.SetItemTrash(ctx, id, &entry); err != nil {
			return err
		}
	}
	return nil
}

// TrashItem sends one file to the trash (RF-25).
func (s *LibraryService) TrashItem(ctx context.Context, id domain.ItemID) error {
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
	return s.trash(ctx, g, false, it)
}

// TrashGame sends every file of a game to the trash as one entry (RF-25).
func (s *LibraryService) TrashGame(ctx context.Context, id domain.GameID) error {
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
	return s.trash(ctx, g, true, items...)
}

func (s *LibraryService) trash(ctx context.Context, g *domain.Game, whole bool, items ...domain.GameItem) error {
	b := s.newOp("trash")
	dir, err := b.trash(s.gameDir(g), items...)
	if err != nil {
		return err
	}
	b.pruneGame(g.Console, g.Folder) // the folders go when their last file does
	ids := make([]domain.ItemID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	now := s.now()
	return s.run(ctx, b, func(tx domain.LibraryTx) error {
		return trashItems(ctx, tx, g.ID, domain.TrashDeleted, whole, dir, now, ids...)
	})
}

// Trash lists the trash, newest first. retention gives each entry's
// expiry date.
func (s *LibraryService) Trash(ctx context.Context, retention time.Duration) ([]TrashView, error) {
	entries, err := s.repo.TrashEntries(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TrashView, 0, len(entries))
	games := map[domain.GameID]*domain.Game{}
	for _, e := range entries {
		v := TrashView{TrashEntry: e, ExpiresAt: e.TrashedAt.Add(retention)}
		if e.GameID != nil {
			g, ok := games[*e.GameID]
			if !ok {
				if g, err = s.repo.GameByID(ctx, *e.GameID); err != nil {
					return nil, err
				}
				games[*e.GameID] = g
			}
			v.Game = g
		}
		for _, it := range e.Items {
			v.Size += it.Size
		}
		for _, f := range e.Files {
			v.Size += f.Size
		}
		out = append(out, v)
	}
	return out, nil
}

// RestoreResult says where a trash entry went back to.
type RestoreResult struct {
	// GameID is set for entries of game files.
	GameID *domain.GameID
	// Path is the library-relative folder: the game's, or the unassigned one.
	Path string
}

// Restore puts a trash entry back where it came from. Game files take the
// names of the game's current title; if a place is taken (a duplicate was
// stored since), onConflict must be Replace: the current file then goes to
// the trash. Unassigned files go back to the unassigned section, with a
// free name if theirs was taken meanwhile.
func (s *LibraryService) Restore(ctx context.Context, id domain.TrashEntryID, onConflict domain.DuplicateAction) (RestoreResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.repo.TrashEntry(ctx, id)
	if err != nil {
		return RestoreResult{}, err
	}
	if e.GameID == nil {
		return s.restoreUnassigned(ctx, e)
	}
	g, err := s.repo.GameByID(ctx, *e.GameID)
	if err != nil {
		return RestoreResult{}, err
	}
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return RestoreResult{}, err
	}
	res := RestoreResult{GameID: &g.ID, Path: string(g.Console) + "/" + g.Folder}

	live := g.LiveItems()
	names := nameSet{}
	newNames := make([]string, len(e.Items))
	var conflicts []domain.GameItem
	seen := map[domain.ItemID]bool{}
	for i, it := range e.Items {
		name, err := fileName(g.Title, it.Kind, it.Label, extensionOf(it.File, consoles), it.File)
		if err != nil {
			return res, err
		}
		if err := names.add(it.File, name); err != nil {
			return res, err
		}
		newNames[i] = name
		if dup := domain.FindDuplicate(live, name); dup != nil && !seen[dup.ID] {
			seen[dup.ID] = true
			conflicts = append(conflicts, *dup)
		}
	}
	if len(conflicts) > 0 && onConflict != domain.Replace {
		var taken []string
		for _, c := range conflicts {
			taken = append(taken, c.File)
		}
		return res, reject(RejectConflict, "Su lugar está ocupado por: %s. Elige Reemplazar (lo actual va a la papelera) o Cancelar.",
			strings.Join(taken, ", "))
	}

	b := s.newOp("restore")
	gameDir := s.gameDir(g)
	b.mkdir(gameDir)
	var replacedDir string
	if len(conflicts) > 0 {
		if replacedDir, err = b.trash(gameDir, conflicts...); err != nil {
			return res, err
		}
	}
	for i, it := range e.Items {
		if err := b.place(s.files.TrashPath(e.Dir), it.File, gameDir, newNames[i], false); err != nil {
			return res, err
		}
	}
	b.purge = append(b.purge, s.files.TrashPath(e.Dir))
	now := s.now()
	err = s.run(ctx, b, func(tx domain.LibraryTx) error {
		if len(conflicts) > 0 {
			ids := make([]domain.ItemID, len(conflicts))
			for i, c := range conflicts {
				ids[i] = c.ID
			}
			if err := trashItems(ctx, tx, g.ID, domain.TrashReplaced, false, replacedDir, now, ids...); err != nil {
				return err
			}
		}
		for i, it := range e.Items {
			if err := tx.SetItemTrash(ctx, it.ID, nil); err != nil {
				return err
			}
			restored := it
			restored.File = newNames[i]
			if err := tx.UpdateItem(ctx, restored); err != nil {
				return err
			}
		}
		return tx.DeleteTrashEntry(ctx, e.ID)
	})
	return res, err
}

func (s *LibraryService) restoreUnassigned(ctx context.Context, e *domain.TrashEntry) (RestoreResult, error) {
	res := RestoreResult{Path: unassignedDir}
	b := s.newOp("restore")
	paths := make([]string, len(e.Files))
	for i, f := range e.Files {
		free, err := b.freeName(s.files.UnassignedPath(), f.Path)
		if err != nil {
			return res, err
		}
		paths[i] = free
		b.mkdirsFor(s.files.UnassignedPath(), free)
		if err := b.place(s.files.TrashPath(e.Dir), f.Path, s.files.UnassignedPath(), free, false); err != nil {
			return res, err
		}
	}
	b.purge = append(b.purge, s.files.TrashPath(e.Dir))
	return res, s.run(ctx, b, func(tx domain.LibraryTx) error {
		for i, f := range e.Files {
			if err := tx.SetUnassignedPlace(ctx, f.ID, paths[i], nil); err != nil {
				return err
			}
		}
		return tx.DeleteTrashEntry(ctx, e.ID)
	})
}

// DeleteTrashEntry deletes an entry and its files for good.
func (s *LibraryService) DeleteTrashEntry(ctx context.Context, id domain.TrashEntryID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, err := s.repo.TrashEntry(ctx, id)
	if err != nil {
		return err
	}
	return s.purge(ctx, *e)
}

// EmptyTrash deletes every entry for good and returns how many there were.
func (s *LibraryService) EmptyTrash(ctx context.Context) (int, error) {
	return s.purgeWhere(ctx, func(domain.TrashEntry) bool { return true })
}

// PurgeExpired deletes the entries older than retention (RF-30) and any
// trash folder no entry knows about.
func (s *LibraryService) PurgeExpired(ctx context.Context, retention time.Duration) (int, error) {
	limit := s.now().Add(-retention)
	return s.purgeWhere(ctx, func(e domain.TrashEntry) bool { return e.TrashedAt.Before(limit) })
}

func (s *LibraryService) purgeWhere(ctx context.Context, match func(domain.TrashEntry) bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := s.repo.TrashEntries(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	known := map[string]bool{}
	var errs []error
	for _, e := range entries {
		if !match(e) {
			known[e.Dir] = true
			continue
		}
		if err := s.purge(ctx, e); err != nil {
			errs = append(errs, err)
			known[e.Dir] = true
			continue
		}
		n++
	}
	// Folders left by an interrupted purge, or by a trash operation undone
	// after its folder was created. Folders of an operation whose undo is
	// still pending stay: the next start moves their files back.
	pending, err := s.repo.Operations(ctx)
	if err != nil {
		return n, errors.Join(append(errs, err)...)
	}
	dirs, err := s.files.List(s.files.TrashPath())
	if err != nil {
		return n, errors.Join(append(errs, err)...)
	}
	for _, d := range dirs {
		if known[d] || slices.ContainsFunc(pending, func(op domain.Operation) bool { return strings.HasPrefix(d, op.ID+"-") }) {
			continue
		}
		s.removeAll(s.files.TrashPath(d))
	}
	return n, errors.Join(errs...)
}

// purge forgets the entry first, then deletes its files: a failure leaves
// files that the next purge sweeps, never records without files.
func (s *LibraryService) purge(ctx context.Context, e domain.TrashEntry) error {
	err := s.repo.Apply(ctx, "", func(tx domain.LibraryTx) error {
		for _, it := range e.Items {
			if err := tx.DeleteItem(ctx, it.ID); err != nil {
				return err
			}
		}
		for _, f := range e.Files {
			if err := tx.DeleteUnassigned(ctx, f.ID); err != nil {
				return err
			}
		}
		if err := tx.DeleteTrashEntry(ctx, e.ID); err != nil {
			return err
		}
		if e.GameID != nil {
			return tx.DeleteGameIfEmpty(ctx, *e.GameID)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("purge trash entry %d: %w", e.ID, err)
	}
	if err := s.files.RemoveAll(s.files.TrashPath(e.Dir)); err != nil {
		s.log.Warn("delete trash files", "entry", e.ID, "error", err)
	}
	return nil
}
