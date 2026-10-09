package application

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// An operation changes files and the catalog together (RF-10, RF-24):
//
//  1. a builder plans it: moves and folders to create — reading the disk
//     but changing nothing;
//  2. run checks that nothing is in the way, creates the folders, journals
//     the moves and performs them;
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
	// After success: purge is deleted recursively, prune only if empty (in order).
	purge, prune []string
	// targets are the destinations planned so far, so two moves of the same
	// operation never pick the same free name.
	targets map[string]bool
	n       int
}

// ErrUndoFailed means a failed change could not be fully reverted: some
// files may be in the library without being recorded. The journal is kept
// and the next start retries.
var ErrUndoFailed = errors.New("library change could not be undone")

func (s *LibraryService) newOp(source string) *opBuilder {
	return &opBuilder{s: s, op: domain.Operation{ID: s.newID(), Source: source, CreatedAt: s.now()}, targets: map[string]bool{}}
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
	b.targets[strings.ToLower(to)] = true
	switch {
	case from == to:
	case strings.EqualFold(from, to):
		tmp := b.scratch("case-" + strconv.Itoa(b.next()))
		b.op.Moves = append(b.op.Moves, domain.Move{From: from, To: tmp}, domain.Move{From: tmp, To: to})
	default:
		b.op.Moves = append(b.op.Moves, domain.Move{From: from, To: to})
	}
}

// place moves one file from srcDir/src to dstDir/dst. A missing source is
// skipped unless required.
func (b *opBuilder) place(srcDir, src, dstDir, dst string, required bool) error {
	return b.placeAfter(srcDir, srcDir, src, dstDir, dst, required)
}

// placeAfter is place for a file that an earlier move of the same operation
// brings to srcDir: it is looked up in nowDir.
func (b *opBuilder) placeAfter(nowDir, srcDir, src, dstDir, dst string, required bool) error {
	from := filepath.Join(srcDir, filepath.FromSlash(src))
	ok, err := b.s.files.Exists(filepath.Join(nowDir, filepath.FromSlash(src)))
	if err != nil {
		return err
	}
	if !ok {
		if required {
			return fmt.Errorf("%s is missing", from)
		}
		return nil
	}
	b.move(from, filepath.Join(dstDir, filepath.FromSlash(dst)))
	return nil
}

// freeName returns rel (relative to dir, "/" separators) or, when it is
// taken on disk or by this operation, rel with " (2)", " (3)"… before the
// extension.
func (b *opBuilder) freeName(dir, rel string) (string, error) {
	ext := path.Ext(rel)
	stem := strings.TrimSuffix(rel, ext)
	for i := 1; i < 1000; i++ {
		candidate := rel
		if i > 1 {
			candidate = fmt.Sprintf("%s (%d)%s", stem, i, ext)
		}
		full := filepath.Join(dir, filepath.FromSlash(candidate))
		if b.targets[strings.ToLower(full)] {
			continue
		}
		taken, err := b.s.files.Exists(full)
		if err != nil {
			return "", err
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free name for %s", rel)
}

// trash plans moving game files from gameDir into a new trash directory and
// returns its name (relative to the trash).
func (b *opBuilder) trash(gameDir string, items ...domain.GameItem) (string, error) {
	dir := b.newTrashDir()
	for _, it := range items {
		if err := b.place(gameDir, it.File, b.s.files.TrashPath(dir), it.File, false); err != nil {
			return "", err
		}
	}
	return dir, nil
}

func (b *opBuilder) newTrashDir() string {
	dir := b.op.ID + "-" + strconv.Itoa(b.next())
	b.mkdir(b.s.files.TrashPath(dir))
	return dir
}

// mkdirsFor plans the parent folders of rel (relative to dir).
func (b *opBuilder) mkdirsFor(dir, rel string) {
	if parent := path.Dir(rel); parent != "." {
		b.mkdir(filepath.Join(dir, filepath.FromSlash(parent)))
	}
}

// pruneParents removes the empty folders between a file and stop
// (exclusive), innermost first.
func (b *opBuilder) pruneParents(stop, rel string) {
	for d := path.Dir(rel); d != "." && d != "/"; d = path.Dir(d) {
		b.prune = append(b.prune, filepath.Join(stop, filepath.FromSlash(d)))
	}
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

	if len(b.op.Moves) > 0 {
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
	for _, d := range slices.Compact(b.prune) {
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
