package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
)

// CommitItem is the user's decision for one staged item (RF-09, RF-10).
type CommitItem struct {
	Path        string
	Skip        bool // leave it out of the library
	Kind        string
	Label       string
	DiscNumber  int
	OnDuplicate string // "", "replace" or "skip"
}

// CommitRequest stores a reviewed upload as one game of one console.
type CommitRequest struct {
	Console    string
	IGDBGameID int64
	Items      []CommitItem
}

// LibraryItem is a staged item handed to the library.
type LibraryItem struct {
	Ref         string
	Shape       domain.ItemShape
	Root        string // absolute directory the parts are relative to
	Parts       []string
	Size        int64
	Kind        string
	Label       string
	DiscNumber  int
	TitleID     string
	OnDuplicate string
}

// LibraryRequest is a commit as the library sees it.
type LibraryRequest struct {
	Source     string
	Console    string
	IGDBGameID int64
	Items      []LibraryItem
}

// ExistingItem is an item already in the game's folder.
type ExistingItem struct {
	ID         int64
	Kind       string
	Label      string
	DiscNumber int
	Files      []string
	Size       int64
}

// PlannedItem is what the commit will do with one item.
type PlannedItem struct {
	Path      string
	Files     []string // names inside the game folder
	Duplicate *ExistingItem
	// Action is store, replace, skip or undecided (a duplicate without decision).
	Action string
}

// CommitPlan previews a commit: final folder, names and duplicates.
type CommitPlan struct {
	Console  string
	Title    string
	Folder   string
	GameID   int64 // 0 when the game is new
	Existing []ExistingItem
	Items    []PlannedItem
}

// CommitResult summarizes a finished commit.
type CommitResult struct {
	GameID                    int64
	Path                      string // "<console>/<game folder>"
	Stored, Replaced, Skipped int
}

// Library is the catalog seen from ingestion (wired in the composition root).
type Library interface {
	Plan(ctx context.Context, req LibraryRequest) (CommitPlan, error)
	// Store moves the items into the library; on failure everything is undone
	// unless the error wraps ErrLibraryInconsistent.
	Store(ctx context.Context, req LibraryRequest) (CommitResult, error)
	// Recover undoes library changes interrupted by a restart.
	Recover(ctx context.Context) (int, error)
	HasItemsFrom(ctx context.Context, source string) (bool, error)
}

// ErrLibraryInconsistent means a failed commit could not be fully undone.
var ErrLibraryInconsistent = errors.New("library change could not be undone")

// ErrNotInReview is returned when a job cannot be committed in its state.
var ErrNotInReview = errors.New("job is not in review")

// RejectReason classifies a refused commit.
type RejectReason string

// Reasons for refusing a commit.
const (
	RejectInvalid     RejectReason = "invalid"
	RejectConflict    RejectReason = "conflict"
	RejectUnavailable RejectReason = "unavailable"
	RejectUpstream    RejectReason = "upstream"
)

// CommitRejected is a refused commit; nothing was changed. Message is shown
// to the user.
type CommitRejected struct {
	Reason  RejectReason
	Message string
}

func (e *CommitRejected) Error() string { return string(e.Reason) + ": " + e.Message }

func invalid(format string, args ...any) *CommitRejected {
	return &CommitRejected{Reason: RejectInvalid, Message: fmt.Sprintf(format, args...)}
}

// Committer moves reviewed uploads into the library (RF-11).
type Committer struct {
	jobs    domain.JobRepository
	items   domain.StagedItemRepository
	staging Staging
	library Library
	pub     Publisher
	log     *slog.Logger
	now     func() time.Time

	mu sync.Mutex // one commit at a time; it also guards the review → committing check
}

// NewCommitter builds the use case. now may be nil (time.Now).
func NewCommitter(jobs domain.JobRepository, items domain.StagedItemRepository, staging Staging, library Library, pub Publisher, log *slog.Logger, now func() time.Time) *Committer {
	if now == nil {
		now = time.Now
	}
	return &Committer{jobs: jobs, items: items, staging: staging, library: library, pub: pub, log: log, now: now}
}

// Plan previews the commit of a job in review.
func (c *Committer) Plan(ctx context.Context, id domain.JobID, req CommitRequest) (CommitPlan, error) {
	job, err := c.jobs.Get(ctx, id)
	if err != nil {
		return CommitPlan{}, err
	}
	if job.Status != domain.StatusReview {
		return CommitPlan{}, ErrNotInReview
	}
	lreq, err := c.libraryRequest(ctx, job, req)
	if err != nil {
		return CommitPlan{}, err
	}
	return c.library.Plan(ctx, lreq)
}

// Commit stores a job in review. On failure the job returns to review with
// its files back in staging (or fails, if the change could not be undone).
func (c *Committer) Commit(ctx context.Context, id domain.JobID, req CommitRequest) (*domain.UploadJob, CommitResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	job, err := c.jobs.Get(ctx, id)
	if err != nil {
		return nil, CommitResult{}, err
	}
	if job.Status != domain.StatusReview {
		return nil, CommitResult{}, ErrNotInReview
	}
	lreq, err := c.libraryRequest(ctx, job, req)
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
	c.removeStaging(id)
	return job, res, nil
}

// Recover finishes commits interrupted by a restart: the library undoes
// unrecorded changes, then each job is done (its items were recorded) or back
// in review (they were not).
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
			c.removeStaging(job.ID)
		}
	}
	return nil
}

// libraryRequest checks the request against the staged items: every item is
// decided exactly once, and only staged paths are referenced.
func (c *Committer) libraryRequest(ctx context.Context, job *domain.UploadJob, req CommitRequest) (LibraryRequest, error) {
	staged, err := c.items.List(ctx, job.ID)
	if err != nil {
		return LibraryRequest{}, err
	}
	byPath := map[string]domain.StagedItem{}
	for _, it := range staged {
		if !it.Ignored {
			byPath[it.Path] = it
		}
	}

	out := LibraryRequest{Source: string(job.ID), Console: req.Console, IGDBGameID: req.IGDBGameID}
	seen := map[string]bool{}
	for _, it := range req.Items {
		s, ok := byPath[it.Path]
		if !ok {
			return out, invalid("%q no es un elemento de esta subida.", it.Path)
		}
		if seen[it.Path] {
			return out, invalid("%q aparece dos veces.", it.Path)
		}
		seen[it.Path] = true
		if it.Skip {
			continue
		}
		switch it.OnDuplicate {
		case "", "replace", "skip":
		default:
			return out, invalid("%q: decisión de duplicado %q desconocida.", it.Path, it.OnDuplicate)
		}
		out.Items = append(out.Items, LibraryItem{
			Ref: it.Path, Shape: s.Shape, Root: c.staging.Dir(job.ID), Parts: s.Parts, Size: s.Size,
			Kind: it.Kind, Label: it.Label, DiscNumber: it.DiscNumber, TitleID: s.TitleID, OnDuplicate: it.OnDuplicate,
		})
	}
	for _, it := range staged {
		if !it.Ignored && !seen[it.Path] {
			return out, invalid("Falta decidir qué hacer con %q.", it.Path)
		}
	}
	if len(out.Items) == 0 {
		return out, invalid("Elige al menos un elemento para guardar, o cancela la subida.")
	}
	return out, nil
}

func (c *Committer) removeStaging(id domain.JobID) {
	if err := c.staging.Remove(id); err != nil {
		c.log.Warn("remove staging after commit", "job", id, "error", err)
	}
}

func (c *Committer) save(ctx context.Context, job *domain.UploadJob) error {
	if err := c.jobs.Save(ctx, job); err != nil {
		return err
	}
	c.pub.Publish(*job)
	return nil
}
