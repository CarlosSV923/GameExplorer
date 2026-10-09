package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
)

// CommitFile is the user's data for one valid file (RF-08, RF-09).
type CommitFile struct {
	Path        string
	Kind        string // "" on consoles with one kind
	Label       string
	OnDuplicate string // "", "replace" or "skip"
	// Skip leaves the file in the unassigned section ("No guardar"); only
	// for assigned entries (RF-27a).
	Skip bool
}

// LibraryFile is a staged file handed to the library.
type LibraryFile struct {
	Ref         string
	Root        string // absolute directory Path is relative to
	Path        string
	Size        int64
	Kind        string
	Label       string
	OnDuplicate string
	// Unassigned is the file's id in the unassigned section when it comes
	// straight from there (Root is the section's folder).
	Unassigned int64
}

// Renumbering gives a file already in the game a disc number (RF-08a).
type Renumbering struct {
	Item  int64
	Label string
}

// CommitInput is the user's data for a commit: one entry per valid file,
// and the disc numbers of files already in the game.
type CommitInput struct {
	Files    []CommitFile
	Renumber []Renumbering
}

// LibraryRequest is a commit as the library sees it.
type LibraryRequest struct {
	Source   string
	Console  string
	Title    string
	IGDBID   *int64
	Files    []LibraryFile
	Renumber []Renumbering
}

// RenamedItem is a file already in the game that the commit renumbers.
type RenamedItem struct {
	Item ExistingItem
	File string
}

// ExistingItem is a file already in the game's folder.
type ExistingItem struct {
	ID        int64
	Kind      string
	Label     string
	File      string
	Size      int64
	CreatedAt time.Time
}

// PlannedFile is what the commit will do with one file.
type PlannedFile struct {
	Path      string
	File      string // name inside the game folder
	Duplicate *ExistingItem
	// Action is store, replace, skip or undecided (a duplicate without decision).
	Action string
}

// CommitPlan previews a commit: final folder, names and duplicates.
type CommitPlan struct {
	Console   string
	Title     string
	Folder    string
	GameID    int64 // 0 when the game is new
	Existing  []ExistingItem
	Files     []PlannedFile
	Renamed   []RenamedItem
	Discarded []string
}

// CommitResult summarizes a finished commit.
type CommitResult struct {
	GameID                    int64
	Path                      string // "<console>/<game folder>"
	Stored, Replaced, Skipped int
}

// SetAsideRequest puts files that did not become game files into the
// unassigned section (or the trash, restorable there).
type SetAsideRequest struct {
	Source string
	// Root is the absolute directory Files are relative to.
	Root   string
	Files  []string
	Folder string
	Origin string
	// Reason is upload (did not fit its console or had none) or manual
	// (given back).
	Reason string
	// Console and IGDBID prefill a later assignment (optional).
	Console string
	IGDBID  *int64
	ToTrash bool
}

// UnassignedSource is a file of the unassigned section.
type UnassignedSource struct {
	ID   int64
	Name string
	// Path is relative to the unassigned folder; Abs is absolute.
	Path string
	Abs  string
	Size int64
}

// UnassignedEntry is an entry of the section an assignment took (RF-27a).
type UnassignedEntry struct {
	Name  string
	Size  int64
	Files []UnassignedSource
}

// Library is the catalog seen from ingestion (wired in the composition root).
type Library interface {
	Plan(ctx context.Context, req LibraryRequest) (CommitPlan, error)
	// Store moves the files into the library; on failure everything is undone
	// unless the error wraps ErrLibraryInconsistent.
	Store(ctx context.Context, req LibraryRequest) (CommitResult, error)
	// Recover undoes library changes interrupted by a restart.
	Recover(ctx context.Context) (int, error)
	HasItemsFrom(ctx context.Context, source string) (bool, error)
	PutAside(ctx context.Context, req SetAsideRequest) error
	// TakeEntry marks the entry a file of the section belongs to as used by
	// the job (ErrUnassignedNotFound, or a CommitRejected if it is busy).
	TakeEntry(ctx context.Context, id int64, job string) (UnassignedEntry, error)
	// JobFiles returns the files of the section the job uses.
	JobFiles(ctx context.Context, job string) ([]UnassignedSource, error)
	// ReleaseJob frees them.
	ReleaseJob(ctx context.Context, job string) error
	// DeleteJobFiles deletes some of them for good (extracted archives).
	DeleteJobFiles(ctx context.Context, job string, ids []int64) error
	// UnassignedDir is the section's absolute folder.
	UnassignedDir() string
}

// Errors of the commit and the unassigned section.
var (
	// ErrLibraryInconsistent means a failed commit could not be fully undone.
	ErrLibraryInconsistent = errors.New("library change could not be undone")
	// ErrNotConfirmable is returned when a job cannot be committed in its state.
	ErrNotConfirmable = errors.New("job is not waiting for confirmation")
	// ErrNotInvalid is returned when setting aside a job that fits its console.
	ErrNotInvalid = errors.New("job is not invalid")
	// ErrUnassignedNotFound means the unassigned file does not exist.
	ErrUnassignedNotFound = errors.New("unassigned file not found")
)

// RejectReason classifies a refused request.
type RejectReason string

// Reasons for refusing a request.
const (
	RejectInvalid     RejectReason = "invalid"
	RejectConflict    RejectReason = "conflict"
	RejectUnavailable RejectReason = "unavailable"
	RejectUpstream    RejectReason = "upstream"
)

// CommitRejected is a refused request; nothing was changed. Message is
// shown to the user.
type CommitRejected struct {
	Reason  RejectReason
	Message string
}

func (e *CommitRejected) Error() string { return string(e.Reason) + ": " + e.Message }

func invalid(format string, args ...any) *CommitRejected {
	return &CommitRejected{Reason: RejectInvalid, Message: fmt.Sprintf(format, args...)}
}

// Committer moves confirmed uploads into the library (RF-08..RF-10), and
// changes the console of, or sets aside, uploads that do not fit (RF-07).
type Committer struct {
	jobs     domain.JobRepository
	files    domain.StagedFileRepository
	staging  Staging
	library  Library
	consoles Consoles
	pub      Publisher
	log      *slog.Logger
	now      func() time.Time

	mu sync.Mutex // one change at a time; it also guards the status checks
}

// NewCommitter builds the use case. now may be nil (time.Now).
func NewCommitter(jobs domain.JobRepository, files domain.StagedFileRepository, staging Staging, library Library, consoles Consoles, pub Publisher, log *slog.Logger, now func() time.Time) *Committer {
	if now == nil {
		now = time.Now
	}
	return &Committer{jobs: jobs, files: files, staging: staging, library: library, consoles: consoles, pub: pub, log: log, now: now}
}

// FileView is a staged file with what the confirmation screen shows.
type FileView struct {
	domain.StagedFile
	// Valid is a game file for the job's console.
	Valid bool
	// Consoles accept the file's extension.
	Consoles []string
}

// Files lists the files found in a job.
func (c *Committer) Files(ctx context.Context, id domain.JobID) ([]FileView, error) {
	job, err := c.jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	files, err := c.files.List(ctx, id)
	if err != nil {
		return nil, err
	}
	rules, known, err := c.consoles.Rules(ctx)
	if err != nil {
		return nil, err
	}
	valid, _ := domain.ValidateFor(job, rules[job.Console], files, known)
	isValid := map[string]bool{}
	for _, f := range valid {
		isValid[f.Path] = true
	}
	out := make([]FileView, 0, len(files))
	for _, f := range files {
		v := FileView{StagedFile: f, Valid: isValid[f.Path], Consoles: []string{}}
		ext := domain.ExtensionOf(path.Base(f.Path), known)
		for slug, r := range rules {
			for _, e := range r.Extensions {
				if e == ext {
					v.Consoles = append(v.Consoles, slug)
				}
			}
		}
		slices.Sort(v.Consoles)
		out = append(out, v)
	}
	return out, nil
}

// Plan previews the commit of a job waiting for confirmation.
func (c *Committer) Plan(ctx context.Context, id domain.JobID, req CommitInput) (CommitPlan, error) {
	job, err := c.jobs.Get(ctx, id)
	if err != nil {
		return CommitPlan{}, err
	}
	if job.Status != domain.StatusConfirm {
		return CommitPlan{}, ErrNotConfirmable
	}
	lreq, discarded, err := c.libraryRequest(ctx, job, req)
	if err != nil {
		return CommitPlan{}, err
	}
	plan, err := c.library.Plan(ctx, lreq)
	plan.Discarded = discarded
	return plan, err
}

// Commit stores a job waiting for confirmation. On failure the job returns
// to confirmation with its files back in staging (or fails, if the change
// could not be undone).
func (c *Committer) Commit(ctx context.Context, id domain.JobID, req CommitInput) (*domain.UploadJob, CommitResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	job, err := c.jobs.Get(ctx, id)
	if err != nil {
		return nil, CommitResult{}, err
	}
	if job.Status != domain.StatusConfirm {
		return nil, CommitResult{}, ErrNotConfirmable
	}
	lreq, _, err := c.libraryRequest(ctx, job, req)
	if err != nil {
		return nil, CommitResult{}, err
	}
	if err := job.StartCommit(c.now()); err != nil {
		return nil, CommitResult{}, err
	}
	if err := c.save(ctx, job); err != nil {
		return nil, CommitResult{}, err
	}

	// The move must finish even if the browser goes away.
	ctx = context.WithoutCancel(ctx)
	res, err := c.library.Store(ctx, lreq)
	if err != nil {
		var rejected *CommitRejected
		switch {
		case errors.As(err, &rejected):
			_ = job.AbortCommit("", c.now())
		case errors.Is(err, ErrLibraryInconsistent):
			c.log.Error("commit could not be undone", "job", id, "error", err)
			_ = job.Fail("No se pudo guardar ni deshacer el guardado; revisa la carpeta del juego por SMB. "+
				"Al reiniciar la app se intentará deshacer de nuevo.", c.now())
		default:
			c.log.Warn("commit failed and was undone", "job", id, "error", err)
			_ = job.AbortCommit("No se pudo guardar en la biblioteca y se deshicieron los cambios: "+err.Error(), c.now())
		}
		if saveErr := c.save(ctx, job); saveErr != nil {
			c.log.Error("save job after failed commit", "job", id, "error", saveErr)
		}
		return job, res, err
	}

	if err := job.FinishCommit(c.now()); err != nil {
		return nil, res, err
	}
	if err := c.save(ctx, job); err != nil {
		return nil, res, err
	}
	c.finishEntry(ctx, job)
	c.cleanUp(job)
	return job, res, nil
}

// ChangeConsole validates the job's files for another console, without
// extracting again (RF-07).
func (c *Committer) ChangeConsole(ctx context.Context, id domain.JobID, console string) (*domain.UploadJob, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	job, err := c.jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if job.Status != domain.StatusConfirm && job.Status != domain.StatusInvalid {
		return nil, ErrNotConfirmable
	}
	rules, known, err := c.consoles.Rules(ctx)
	if err != nil {
		return nil, err
	}
	rule, ok := rules[console]
	if !ok {
		return nil, invalid("La consola %q no existe.", console)
	}
	files, err := c.files.List(ctx, id)
	if err != nil {
		return nil, err
	}
	_, reason := domain.ValidateFor(job, rule, files, known)
	if err := job.Validated(console, reason, c.now()); err != nil {
		return nil, err
	}
	return job, c.save(ctx, job)
}

// Resolution is what to do with an upload that does not fit (RF-07).
type Resolution string

// Resolutions.
const (
	ResolveUnassigned Resolution = "unassigned"
	ResolveTrash      Resolution = "trash"
	ResolveDelete     Resolution = "delete"
)

// Resolve sets aside an invalid upload: its extracted files go to the
// unassigned section, the trash, or are deleted (RF-07).
func (c *Committer) Resolve(ctx context.Context, id domain.JobID, r Resolution) (*domain.UploadJob, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	job, err := c.jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if job.Status != domain.StatusInvalid {
		return nil, ErrNotInvalid
	}
	ctx = context.WithoutCancel(ctx)
	if job.FromEntry() {
		// The files are still in the section: setting them aside is just
		// ending the assignment (RF-27a).
		if r != ResolveDelete && r != ResolveUnassigned {
			return nil, invalid("Los archivos siguen en No asignados: cancela la asignación.")
		}
		if err := job.Cancel(c.now()); err != nil {
			return nil, err
		}
		if err := c.library.ReleaseJob(ctx, string(job.ID)); err != nil {
			return nil, err
		}
		if err := c.save(ctx, job); err != nil {
			return nil, err
		}
		c.cleanUp(job)
		return job, nil
	}
	switch r {
	case ResolveUnassigned, ResolveTrash:
		files, err := c.files.List(ctx, id)
		if err != nil {
			return nil, err
		}
		paths := make([]string, len(files))
		for i, f := range files {
			paths[i] = f.Path
		}
		if len(paths) > 0 {
			if err := c.library.PutAside(ctx, SetAsideRequest{
				Source: string(job.ID), Root: c.staging.Dir(id), Files: paths, Folder: job.Title,
				Origin: job.FileName, Reason: "upload", Console: job.Console, IGDBID: job.IGDBID, ToTrash: r == ResolveTrash,
			}); err != nil {
				return nil, err
			}
		}
		err = job.SetAside(r == ResolveTrash, c.now())
		if err != nil {
			return nil, err
		}
	case ResolveDelete:
		if err := job.Cancel(c.now()); err != nil {
			return nil, err
		}
	default:
		return nil, invalid("Acción %q desconocida.", r)
	}
	if err := c.save(ctx, job); err != nil {
		return nil, err
	}
	c.cleanUp(job)
	return job, nil
}

// Recover finishes commits interrupted by a restart: the library undoes
// unrecorded changes, then each job is done (its files were recorded) or
// back to confirmation (they were not).
func (c *Committer) Recover(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := c.library.Recover(ctx); err != nil {
		return fmt.Errorf("recover library: %w", err)
	}
	jobs, err := c.jobs.ListStale(ctx, domain.StatusCommitting, c.now().Add(time.Hour))
	if err != nil {
		return err
	}
	for _, job := range jobs {
		stored, err := c.library.HasItemsFrom(ctx, string(job.ID))
		if err != nil {
			return err
		}
		if stored {
			err = job.FinishCommit(c.now())
		} else {
			err = job.AbortCommit("El guardado se interrumpió (la app se reinició) y se deshizo; vuelve a confirmar.", c.now())
		}
		if err != nil {
			return err
		}
		if err := c.save(ctx, job); err != nil {
			return err
		}
		if stored {
			c.finishEntry(ctx, job)
			c.cleanUp(job)
		}
	}
	return nil
}

// libraryRequest checks the request against the staged files: every valid
// file is described exactly once, and only those. It also returns the
// files that will be discarded.
func (c *Committer) libraryRequest(ctx context.Context, job *domain.UploadJob, input CommitInput) (LibraryRequest, []string, error) {
	req := input.Files
	files, err := c.files.List(ctx, job.ID)
	if err != nil {
		return LibraryRequest{}, nil, err
	}
	rules, known, err := c.consoles.Rules(ctx)
	if err != nil {
		return LibraryRequest{}, nil, err
	}
	valid, reason := domain.ValidateFor(job, rules[job.Console], files, known)
	if reason != "" {
		return LibraryRequest{}, nil, invalid("Los archivos ya no encajan en la consola: vuelve a elegirla.")
	}
	byPath := map[string]domain.StagedFile{}
	for _, f := range valid {
		byPath[f.Path] = f
	}
	var discarded []string
	for _, f := range files {
		if _, ok := byPath[f.Path]; !ok {
			discarded = append(discarded, f.Path)
		}
	}

	out := LibraryRequest{
		Source: string(job.ID), Console: job.Console, Title: job.Title, IGDBID: job.IGDBID, Renumber: input.Renumber,
	}
	seen := map[string]bool{}
	kept := 0
	for _, f := range req {
		s, ok := byPath[f.Path]
		if !ok {
			return out, nil, invalid("%q no es un archivo de esta subida.", f.Path)
		}
		if seen[f.Path] {
			return out, nil, invalid("%q aparece dos veces.", f.Path)
		}
		seen[f.Path] = true
		if f.Skip {
			if !job.FromEntry() {
				return out, nil, invalid("%q: solo los archivos de No asignados se pueden dejar sin guardar.", f.Path)
			}
			continue
		}
		kept++
		switch f.OnDuplicate {
		case "", "replace", "skip":
		default:
			return out, nil, invalid("%q: decisión de duplicado %q desconocida.", f.Path, f.OnDuplicate)
		}
		lf := LibraryFile{
			Ref: f.Path, Root: c.staging.Dir(job.ID), Path: s.Path, Size: s.Size,
			Kind: f.Kind, Label: strings.TrimSpace(f.Label), OnDuplicate: f.OnDuplicate,
		}
		if s.Unassigned != nil {
			lf.Root, lf.Unassigned = c.library.UnassignedDir(), *s.Unassigned
		}
		out.Files = append(out.Files, lf)
	}
	for _, f := range valid {
		if !seen[f.Path] {
			return out, nil, invalid("Faltan los datos de %q.", f.Path)
		}
	}
	if job.FromEntry() {
		switch {
		case kept == 0:
			return out, nil, invalid("Elige al menos un archivo para guardar.")
		case kept > 1 && !rules[job.Console].MultipleFiles:
			return out, nil, invalid("Esta consola guarda un solo archivo por juego: marca los demás como «No guardar».")
		}
	}
	return out, discarded, nil
}

// finishEntry completes a stored assignment (RF-27a): extracted files that
// were not stored stay in the entry, the extracted archives are deleted
// (RF-06) and the entry's other files are free again.
func (c *Committer) finishEntry(ctx context.Context, job *domain.UploadJob) {
	if !job.FromEntry() {
		return
	}
	id := string(job.ID)
	sources, err := c.library.JobFiles(ctx, id)
	if err != nil {
		c.log.Error("finish assignment: list entry", "job", id, "error", err)
		return
	}
	staged, err := c.files.List(ctx, job.ID)
	if err != nil {
		c.log.Error("finish assignment: list files", "job", id, "error", err)
		return
	}
	inPlace := map[int64]bool{}
	var leftovers []string
	for _, f := range staged {
		switch {
		case f.Unassigned != nil:
			inPlace[*f.Unassigned] = true
		case c.staging.Exists(filepath.Join(c.staging.Dir(job.ID), filepath.FromSlash(f.Path))):
			leftovers = append(leftovers, f.Path)
		}
	}
	// Archives of a folder entry were extracted under it; a loose one's
	// files need a folder: the title's.
	folder := job.Title
	for _, s := range sources {
		if strings.Contains(s.Path, "/") {
			folder = ""
		}
	}
	if len(leftovers) > 0 {
		if err := c.library.PutAside(ctx, SetAsideRequest{
			Source: id, Root: c.staging.Dir(job.ID), Files: leftovers, Folder: folder,
			Origin: job.UnassignedOrigin, Reason: "upload", Console: job.Console, IGDBID: job.IGDBID,
		}); err != nil {
			c.log.Error("finish assignment: keep files not stored", "job", id, "error", err)
		}
	}
	var archives []int64
	for _, s := range sources {
		if !inPlace[s.ID] {
			archives = append(archives, s.ID)
		}
	}
	if len(archives) > 0 {
		if err := c.library.DeleteJobFiles(ctx, id, archives); err != nil {
			c.log.Error("finish assignment: delete extracted archives", "job", id, "error", err)
		}
	}
	if err := c.library.ReleaseJob(ctx, id); err != nil {
		c.log.Error("finish assignment: release entry", "job", id, "error", err)
	}
}

// cleanUp deletes what a finished job leaves in staging.
func (c *Committer) cleanUp(job *domain.UploadJob) {
	if err := c.staging.Remove(job.ID); err != nil {
		c.log.Warn("remove staging", "job", job.ID, "error", err)
	}
	if err := c.staging.RemoveSource(job.ID); err != nil {
		c.log.Warn("remove source", "job", job.ID, "error", err)
	}
}

func (c *Committer) save(ctx context.Context, job *domain.UploadJob) error {
	if err := c.jobs.Save(ctx, job); err != nil {
		return err
	}
	c.pub.Publish(*job)
	return nil
}
