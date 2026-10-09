// Package application holds the ingestion use cases and their ports.
package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
)

// UploadStore is where uploaded bytes live (the tus store).
type UploadStore interface {
	// Delete removes an upload's data; deleting a missing upload is not an error.
	Delete(ctx context.Context, id domain.JobID) error
	// IDs lists every upload present in the store.
	IDs(ctx context.Context) ([]domain.JobID, error)
}

// Queue processes finished uploads (implemented by Processor).
type Queue interface {
	Enqueue(id domain.JobID)
	// Discard stops work on the job and deletes its staged files.
	Discard(id domain.JobID)
}

// Publisher broadcasts job changes (server-sent events).
type Publisher interface {
	Publish(job domain.UploadJob)
}

// AbandonedAfter is how long an upload may stay idle before it is purged (RF-12).
const AbandonedAfter = 24 * time.Hour

// ErrCannotCancel means the job is already finished or committing.
var ErrCannotCancel = errors.New("job can no longer be cancelled")

// Service implements the ingestion use cases.
type Service struct {
	repo     domain.JobRepository
	store    UploadStore
	pub      Publisher
	queue    Queue
	consoles Consoles
	library  Library
	staging  Staging
	log      *slog.Logger
	now      func() time.Time
	newID    func() string
}

// WithQueue sets the processing queue that picks up finished uploads.
func (s *Service) WithQueue(q Queue) *Service {
	s.queue = q
	return s
}

// WithUnassigned lets the service start jobs from the unassigned section.
func (s *Service) WithUnassigned(library Library, staging Staging) *Service {
	s.library, s.staging = library, staging
	return s
}

// NewService builds the service. now may be nil (time.Now).
func NewService(repo domain.JobRepository, store UploadStore, consoles Consoles, pub Publisher, log *slog.Logger, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, store: store, consoles: consoles, pub: pub, log: log, now: now, newID: randomID}
}

func randomID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// UploadMeta is what the browser sends with a tus upload (Upload-Metadata).
type UploadMeta struct {
	FileName, Console, Title, IGDBID, Group, GroupSize string
}

// ErrInvalidMeta is returned for an upload whose form data is wrong.
var ErrInvalidMeta = errors.New("invalid upload metadata")

// SpecFromMeta checks an upload's form data (RF-03, RF-03a).
func (s *Service) SpecFromMeta(ctx context.Context, m UploadMeta) (domain.Spec, error) {
	spec := domain.Spec{Console: m.Console, Title: m.Title, GroupID: m.Group}
	rules, _, err := s.consoles.Rules(ctx)
	if err != nil {
		return spec, err
	}
	// No console sends the upload to the unassigned section (RF-07b).
	if _, ok := rules[m.Console]; m.Console != "" && !ok {
		return spec, fmt.Errorf("%w: unknown console %q", ErrInvalidMeta, m.Console)
	}
	if m.IGDBID != "" {
		id, err := strconv.ParseInt(m.IGDBID, 10, 64)
		if err != nil || id <= 0 {
			return spec, fmt.Errorf("%w: igdbId", ErrInvalidMeta)
		}
		spec.IGDBID = &id
	}
	if m.GroupSize != "" {
		n, err := strconv.Atoi(m.GroupSize)
		if err != nil {
			return spec, fmt.Errorf("%w: groupSize", ErrInvalidMeta)
		}
		spec.GroupSize = n
	}
	if m.Group != "" && !groupShape.MatchString(m.Group) {
		return spec, fmt.Errorf("%w: group", ErrInvalidMeta)
	}
	if _, err := domain.NewUploadJob("check", m.FileName, 0, spec, s.now()); err != nil {
		return spec, fmt.Errorf("%w: %w", ErrInvalidMeta, err)
	}
	return spec, nil
}

var groupShape = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

// UploadCreated registers a job for an upload the browser just started.
func (s *Service) UploadCreated(ctx context.Context, id domain.JobID, size int64, m UploadMeta) error {
	spec, err := s.SpecFromMeta(ctx, m)
	if err != nil {
		return err
	}
	job, err := domain.NewUploadJob(id, m.FileName, size, spec, s.now())
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, job); err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	s.pub.Publish(*job)
	return nil
}

// Assign starts a job from the entry of the unassigned section a file
// belongs to (RF-27a): the entry's files stay in the section, without
// actions, until the job stores them or ends.
func (s *Service) Assign(ctx context.Context, unassigned int64, spec domain.Spec) (*domain.UploadJob, error) {
	if s.library == nil || s.queue == nil {
		return nil, errors.New("uploads are disabled: the library is not writable")
	}
	spec.GroupID, spec.GroupSize = "", 0
	if spec.Console == "" {
		return nil, fmt.Errorf("%w: console is required", ErrInvalidMeta)
	}
	if _, err := s.SpecFromMeta(ctx, UploadMeta{FileName: "entry", Console: spec.Console, Title: spec.Title}); err != nil {
		return nil, err
	}
	id := domain.JobID(s.newID())
	entry, err := s.library.TakeEntry(ctx, unassigned, string(id))
	if err != nil {
		return nil, err
	}
	job, err := s.entryJob(ctx, id, entry, spec)
	if err != nil {
		if rerr := s.library.ReleaseJob(ctx, string(id)); rerr != nil {
			s.log.Error("release unassigned entry", "job", id, "error", rerr)
		}
		return nil, err
	}
	s.pub.Publish(*job)
	s.queue.Enqueue(id)
	return job, nil
}

func (s *Service) entryJob(ctx context.Context, id domain.JobID, entry UnassignedEntry, spec domain.Spec) (*domain.UploadJob, error) {
	job, err := domain.NewUploadJob(id, entry.Name, entry.Size, spec, s.now())
	if err != nil {
		return nil, err
	}
	job.UnassignedOrigin = entry.Name
	if err := job.MarkUploaded("", s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.Create(ctx, job); err != nil {
		return nil, fmt.Errorf("create job: %w", err)
	}
	return job, nil
}

// UploadProgress records received bytes.
func (s *Service) UploadProgress(ctx context.Context, id domain.JobID, received int64) error {
	return s.update(ctx, id, func(j *domain.UploadJob) error {
		j.RecordProgress(received, s.now())
		return nil
	})
}

// UploadFinished records that the upload is complete.
func (s *Service) UploadFinished(ctx context.Context, id domain.JobID, storagePath string) error {
	if err := s.update(ctx, id, func(j *domain.UploadJob) error {
		return j.MarkUploaded(storagePath, s.now())
	}); err != nil {
		return err
	}
	if s.queue != nil {
		s.queue.Enqueue(id)
	}
	return nil
}

// UploadTerminated handles the browser deleting its upload (tus termination).
func (s *Service) UploadTerminated(ctx context.Context, id domain.JobID) error {
	return s.update(ctx, id, func(j *domain.UploadJob) error {
		if j.Status.Terminal() {
			return nil
		}
		return j.Cancel(s.now())
	})
}

// Cancel stops a job and deletes its uploaded data.
func (s *Service) Cancel(ctx context.Context, id domain.JobID) (*domain.UploadJob, error) {
	job, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !job.Status.CanTransitionTo(domain.StatusCancelled) {
		return nil, ErrCannotCancel
	}
	if err := s.store.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("delete upload data: %w", err)
	}
	if s.queue != nil {
		s.queue.Discard(id)
	}
	if err := job.Cancel(s.now()); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, job); err != nil {
		return nil, err
	}
	s.pub.Publish(*job)
	return job, nil
}

// Get returns one job.
func (s *Service) Get(ctx context.Context, id domain.JobID) (*domain.UploadJob, error) {
	return s.repo.Get(ctx, id)
}

// List returns recent jobs, newest first.
func (s *Service) List(ctx context.Context) ([]*domain.UploadJob, error) {
	return s.repo.List(ctx, 100)
}

// PurgeResult summarizes a purge run.
type PurgeResult struct {
	Abandoned int
	Orphans   int
}

// Purge fails uploads idle for longer than AbandonedAfter and deletes stored
// uploads that no job knows about (RF-12).
func (s *Service) Purge(ctx context.Context) (PurgeResult, error) {
	var res PurgeResult
	stale, err := s.repo.ListStale(ctx, domain.StatusUploading, s.now().Add(-AbandonedAfter))
	if err != nil {
		return res, err
	}
	for _, job := range stale {
		if err := s.store.Delete(ctx, job.ID); err != nil {
			return res, err
		}
		if err := job.Fail("Subida abandonada: sin actividad durante 24 h.", s.now()); err != nil {
			return res, err
		}
		if err := s.repo.Save(ctx, job); err != nil {
			return res, err
		}
		s.pub.Publish(*job)
		res.Abandoned++
	}

	waiting, err := s.repo.ListStale(ctx, domain.StatusWaitingParts, s.now().Add(-AbandonedAfter))
	if err != nil {
		return res, err
	}
	for _, job := range waiting {
		if s.queue != nil {
			s.queue.Discard(job.ID) // deletes the gathered part
		}
		if err := job.Fail("Faltan partes del comprimido: no llegaron en 24 h.", s.now()); err != nil {
			return res, err
		}
		if err := s.repo.Save(ctx, job); err != nil {
			return res, err
		}
		s.pub.Publish(*job)
		res.Abandoned++
	}

	ids, err := s.store.IDs(ctx)
	if err != nil {
		return res, err
	}
	for _, id := range ids {
		exists, err := s.repo.Exists(ctx, id)
		if err != nil {
			return res, err
		}
		if !exists {
			if err := s.store.Delete(ctx, id); err != nil {
				return res, err
			}
			res.Orphans++
		}
	}
	return res, nil
}

// RunPurge purges at start and then every interval until ctx ends.
func (s *Service) RunPurge(ctx context.Context, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		res, err := s.Purge(ctx)
		switch {
		case err != nil && ctx.Err() == nil:
			s.log.Warn("upload purge failed", "error", err)
		case res.Abandoned > 0 || res.Orphans > 0:
			s.log.Info("upload purge", "abandoned", res.Abandoned, "orphans", res.Orphans)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (s *Service) update(ctx context.Context, id domain.JobID, fn func(*domain.UploadJob) error) error {
	job, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	before := *job
	if err := fn(job); err != nil {
		return err
	}
	if job.UpdatedAt.Equal(before.UpdatedAt) && job.Status == before.Status {
		return nil // nothing changed
	}
	if err := s.repo.Save(ctx, job); err != nil {
		return err
	}
	s.pub.Publish(*job)
	return nil
}
