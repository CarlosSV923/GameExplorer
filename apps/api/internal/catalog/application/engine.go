package application

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// An operation changes files and the catalog together (RF-11, RF-24):
//
//  1. a builder plans it: moves, folders to create and scratch files
//     (rewritten .cue sheets) — reading the disk but changing nothing;
//  2. run checks that nothing is in the way, creates the folders, writes the
//     scratch files, journals the moves and performs them;
//  3. the catalog changes are applied in one transaction that also deletes
//     the journal entry.
//
// If a move or the transaction fails, or the process stops in between, the
// moves are undone (now, or by Recover on the next start). Nothing is ever
// overwritten: Files.Move refuses existing destinations.

// opBuilder plans one operation.
type opBuilder struct {
	s  *LibraryService
	op domain.Operation
	// mkdirs are created before moving, in order.
	mkdirs []string
	writes []scratchWrite
	// After success: purge is deleted recursively, prune only if empty.
	purge, prune []string
	n            int
}

type scratchWrite struct {
	path string
	data []byte
}

// ErrUndoFailed means a failed change could not be fully reverted: some
// files may be in the library without being recorded. The journal is kept
// and the next start retries.
var ErrUndoFailed = errors.New("library change could not be undone")

func (s *LibraryService) newOp(source string) *opBuilder {
	return &opBuilder{s: s, op: domain.Operation{ID: s.newID(), Source: source, CreatedAt: s.now()}}
}

func (b *opBuilder) next() int {
	b.n++
	return b.n
}

func (b *opBuilder) scratch(elem ...string) string {
	return b.s.files.ScratchPath(append([]string{b.op.ID}, elem...)...)
}

func (b *opBuilder) mkdir(dir string) { b.mkdirs = append(b.mkdirs, dir) }

// move plans one rename. A rename that only changes the letter case goes
// through a temporary name: on case-insensitive datasets (TrueNAS SMB shares
// usually are) the destination "exists" because it is the source.
func (b *opBuilder) move(from, to string) {
	switch {
	case from == to:
	case strings.EqualFold(from, to):
		tmp := b.scratch("case-" + strconv.Itoa(b.next()))
		b.op.Moves = append(b.op.Moves, domain.Move{From: from, To: tmp}, domain.Move{From: tmp, To: to})
	default:
		b.op.Moves = append(b.op.Moves, domain.Move{From: from, To: to})
	}
}

// place moves an item's files from srcDir (names src, "/" separators) to
// dstDir with the names dst. A disc whose track names change gets a
// rewritten .cue sheet; the original goes to scratch (restored on undo,
// deleted on success). Missing sources are skipped unless required.
func (b *opBuilder) place(shape domain.Shape, srcDir string, src []string, dstDir string, dst []string, required bool) error {
	return b.placeAfter(srcDir, shape, srcDir, src, dstDir, dst, required)
}

// placeAfter is place for files that an earlier move of the same operation
// brings to srcDir: they are looked up (and .cue sheets read) in nowDir.
func (b *opBuilder) placeAfter(nowDir string, shape domain.Shape, srcDir string, src []string, dstDir string, dst []string, required bool) error {
	for i := range src {
		from := filepath.Join(srcDir, filepath.FromSlash(src[i]))
		now := filepath.Join(nowDir, filepath.FromSlash(src[i]))
		to := filepath.Join(dstDir, dst[i])
		ok, err := b.s.files.Exists(now)
		if err != nil {
			return err
		}
		if !ok {
			if required {
				return fmt.Errorf("%s is missing", from)
			}
			continue
		}
		if shape == domain.ShapeDisc && i == 0 && tracksRenamed(src, dst) {
			sheet, err := b.s.files.ReadFile(now, maxCueSize)
			if err != nil {
				return fmt.Errorf("read cue: %w", err)
			}
			n := strconv.Itoa(b.next())
			rewritten, original := b.scratch(n+".cue"), b.scratch(n+".orig.cue")
			b.writes = append(b.writes, scratchWrite{rewritten, []byte(rewriteCue(string(sheet), src, dst))})
			b.move(from, original)
			b.move(rewritten, to)
			continue
		}
		b.move(from, to)
	}
	return nil
}

func tracksRenamed(src, dst []string) bool {
	for i := 1; i < len(src); i++ {
		if path.Base(src[i]) != dst[i] {
			return true
		}
	}
	return false
}

// rewriteCue points the sheet's FILE lines (relative to the sheet) at the
// new track names.
func rewriteCue(sheet string, src, dst []string) string {
	tracks := map[string]string{}
	for i := 1; i < len(src); i++ {
		tracks[strings.ToLower(src[i])] = dst[i]
	}
	dir := path.Dir(src[0])
	return domain.RewriteCue(sheet, func(ref string) (string, bool) {
		name, ok := tracks[strings.ToLower(path.Join(dir, strings.ReplaceAll(ref, `\`, "/")))]
		return name, ok
	})
}

// trash plans moving items' files from gameDir into a new trash directory
// and returns its name (relative to the trash).
func (b *opBuilder) trash(gameDir string, items ...domain.GameItem) (string, error) {
	dir := b.op.ID + "-" + strconv.Itoa(b.next())
	b.mkdir(b.s.files.TrashPath(dir))
	for _, it := range items {
		if err := b.place(it.Shape, gameDir, it.Files, b.s.files.TrashPath(dir), it.Files, false); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// run performs the operation (see the top of this file).
func (s *LibraryService) run(ctx context.Context, b *opBuilder, apply func(domain.LibraryTx) error) error {
	// Nothing may be in the way, except paths that an earlier move frees.
	freed := map[string]bool{}
	for _, m := range b.op.Moves {
		freed[strings.ToLower(m.From)] = true
		if freed[strings.ToLower(m.To)] {
			continue
		}
		taken, err := s.files.Exists(m.To)
		if err != nil {
			return err
		}
		if taken {
			return reject(RejectConflict,
				"Ya hay un archivo o carpeta %q que la app no conoce; muévelo o bórralo por SMB.", filepath.Base(m.To))
		}
	}

	if len(b.writes) > 0 || len(b.op.Moves) > 0 {
		b.mkdirs = append(b.mkdirs, b.scratch())
	}
	for _, d := range b.mkdirs {
		created, err := s.files.MkdirAll(d)
		b.op.CreatedDirs = append(b.op.CreatedDirs, created...)
		if err != nil {
			s.discard(b.op)
			return fmt.Errorf("create folder: %w", err)
		}
	}
	for _, w := range b.writes {
		if err := s.files.WriteFile(w.path, w.data); err != nil {
			s.discard(b.op)
			return fmt.Errorf("write %s: %w", filepath.Base(w.path), err)
		}
	}

	journal := ""
	if len(b.op.Moves) > 0 {
		if err := s.repo.SaveOperation(ctx, b.op); err != nil {
			s.discard(b.op)
			return fmt.Errorf("save journal: %w", err)
		}
		journal = b.op.ID
	}
	for _, m := range b.op.Moves {
		if err := s.files.Move(m.From, m.To); err != nil {
			return s.undoAfter(ctx, b.op, fmt.Errorf("move %s: %w", filepath.Base(m.From), err))
		}
	}
	if err := s.repo.Apply(ctx, journal, apply); err != nil {
		if journal == "" {
			s.discard(b.op)
			return err
		}
		return s.undoAfter(ctx, b.op, fmt.Errorf("record changes: %w", err))
	}

	s.removeAll(b.scratch())
	for _, d := range b.purge {
		s.removeAll(d)
	}
	for _, d := range b.prune {
		if err := s.files.RemoveEmptyDir(d); err != nil {
			s.log.Warn("remove empty folder", "dir", d, "error", err)
		}
	}
	s.removeDirs(b.op.CreatedDirs) // only the ones left empty
	return nil
}

// discard cleans up an operation that failed before any move.
func (s *LibraryService) discard(op domain.Operation) {
	s.removeAll(s.files.ScratchPath(op.ID))
	s.removeDirs(op.CreatedDirs)
}

func (s *LibraryService) undoAfter(ctx context.Context, op domain.Operation, cause error) error {
	if !s.undo(ctx, op) {
		return fmt.Errorf("%w: %w", ErrUndoFailed, cause)
	}
	return cause
}

// undo reverts an operation's moves (newest first) and removes what it
// created. A move is reverted only when its destination exists and its
// source does not, so undo is safe to repeat after a crash at any point.
func (s *LibraryService) undo(ctx context.Context, op domain.Operation) bool {
	failed := false
	for i := len(op.Moves) - 1; i >= 0; i-- {
		m := op.Moves[i]
		fromExists, err1 := s.files.Exists(m.From)
		toExists, err2 := s.files.Exists(m.To)
		if err := errors.Join(err1, err2); err != nil {
			s.log.Error("undo move: stat", "from", m.From, "to", m.To, "error", err)
			failed = true
			continue
		}
		if fromExists || !toExists {
			continue
		}
		if err := s.files.Move(m.To, m.From); err != nil {
			s.log.Error("undo move", "from", m.To, "to", m.From, "error", err)
			failed = true
		}
	}
	if failed {
		return false // keep the journal (and scratch) so the next start tries again
	}
	s.discard(op)
	if err := s.repo.DeleteOperation(ctx, op.ID); err != nil {
		s.log.Warn("delete journal", "operation", op.ID, "error", err)
	}
	return true
}

func (s *LibraryService) removeDirs(dirs []string) {
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := s.files.RemoveEmptyDir(dirs[i]); err != nil {
			s.log.Warn("remove created folder", "dir", dirs[i], "error", err)
		}
	}
}

func (s *LibraryService) removeAll(p string) {
	if err := s.files.RemoveAll(p); err != nil {
		s.log.Warn("remove", "path", p, "error", err)
	}
}

// Recover undoes the operations interrupted by a restart and deletes
// leftover scratch files. Call it before anything else touches the library.
func (s *LibraryService) Recover(ctx context.Context) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ops, err := s.repo.Operations(ctx)
	if err != nil {
		return 0, err
	}
	keep := map[string]bool{}
	for _, op := range ops {
		s.log.Warn("undoing interrupted library change", "operation", op.ID, "source", op.Source, "moves", len(op.Moves))
		if !s.undo(ctx, op) {
			keep[op.ID] = true
		}
	}
	names, err := s.files.List(s.files.ScratchPath())
	if err != nil {
		return len(ops), err
	}
	for _, n := range names {
		if !keep[n] {
			s.removeAll(s.files.ScratchPath(n))
		}
	}
	return len(ops), nil
}
