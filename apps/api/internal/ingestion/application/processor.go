package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/scan"
)

// ExtractionMargin is kept free on top of an archive's uncompressed size.
const ExtractionMargin = 1 << 30 // 1 GiB

// progressInterval throttles extraction progress writes and events.
const progressInterval = time.Second

// ErrNotWaitingForPassword is returned when a password arrives for a job
// that is not paused on one.
var ErrNotWaitingForPassword = errors.New("job is not waiting for a password")

// ProcessorDeps are the processor's collaborators.
type ProcessorDeps struct {
	Jobs    domain.JobRepository
	Items   domain.StagedItemRepository
	Uploads interface {
		UploadStore
		UploadAdopter
	}
	Extractor Extractor
	Staging   Staging
	Profiles  ProfileSource
	Publisher Publisher
	Log       *slog.Logger
	Now       func() time.Time
}

type request struct {
	id       domain.JobID
	password string
}

// Processor moves uploaded jobs to review: extraction (with password and
// integrity checks) or adoption of a raw file, then scanning and detection.
type Processor struct {
	d     ProcessorDeps
	queue chan request

	mu      sync.Mutex
	running map[domain.JobID]context.CancelFunc

	// setMu serializes volume-set completion: parts arriving together must
	// not both promote the set.
	setMu sync.Mutex
}

// NewProcessor builds a processor.
func NewProcessor(d ProcessorDeps) *Processor {
	if d.Now == nil {
		d.Now = time.Now
	}
	return &Processor{d: d, queue: make(chan request, 256), running: map[domain.JobID]context.CancelFunc{}}
}

// Run starts workers (concurrency ≥ 1) and re-queues work interrupted by a
// restart. It returns when ctx ends.
func (p *Processor) Run(ctx context.Context, concurrency int) {
	p.resume(ctx)
	var wg sync.WaitGroup
	for range max(1, concurrency) {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case req := <-p.queue:
					p.process(ctx, req)
				}
			}
		})
	}
	wg.Wait()
}

// Enqueue schedules a job (called when an upload finishes).
func (p *Processor) Enqueue(id domain.JobID) {
	p.enqueue(request{id: id})
}

// SubmitPassword retries a job paused on a password.
func (p *Processor) SubmitPassword(ctx context.Context, id domain.JobID, password string) (*domain.UploadJob, error) {
	job, err := p.d.Jobs.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if job.Status != domain.StatusNeedsPassword {
		return nil, ErrNotWaitingForPassword
	}
	p.enqueue(request{id: id, password: password})
	return job, nil
}

// Discard stops any running work for the job and deletes its staged files
// (and, for multi-volume archives, its parts).
func (p *Processor) Discard(id domain.JobID) {
	p.mu.Lock()
	if cancel, ok := p.running[id]; ok {
		cancel()
	}
	p.mu.Unlock()
	if err := p.d.Staging.Remove(id); err != nil {
		p.d.Log.Warn("remove staging", "job", id, "error", err)
	}
	job, err := p.d.Jobs.Get(context.Background(), id)
	if err != nil || job.VolumeSet == "" {
		return
	}
	if job.Status == domain.StatusWaitingParts {
		err = p.d.Staging.RemovePart(job.StoragePath)
	} else {
		err = p.d.Staging.RemoveVolumes(job.VolumeSet)
	}
	if err != nil {
		p.d.Log.Warn("remove volume files", "job", id, "error", err)
	}
}

func (p *Processor) enqueue(r request) {
	select {
	case p.queue <- r:
	default:
		p.d.Log.Error("processing queue full; job will resume on restart", "job", r.id)
	}
}

func (p *Processor) resume(ctx context.Context) {
	for _, status := range []domain.Status{domain.StatusUploaded, domain.StatusExtracting} {
		jobs, err := p.d.Jobs.ListStale(ctx, status, p.d.Now().Add(time.Hour))
		if err != nil {
			p.d.Log.Warn("resume jobs", "error", err)
			return
		}
		for _, j := range jobs {
			p.enqueue(request{id: j.ID})
		}
	}
}

func (p *Processor) process(parent context.Context, req request) {
	ctx, cancel := context.WithCancel(parent)
	p.mu.Lock()
	p.running[req.id] = cancel
	p.mu.Unlock()
	defer func() {
		cancel()
		p.mu.Lock()
		delete(p.running, req.id)
		p.mu.Unlock()
	}()

	job, err := p.d.Jobs.Get(ctx, req.id)
	if err != nil {
		p.d.Log.Warn("process: load job", "job", req.id, "error", err)
		return
	}
	switch job.Status {
	case domain.StatusUploaded, domain.StatusNeedsPassword:
	case domain.StatusExtracting: // interrupted by a restart: start over
		job.ResumeAfterRestart(p.d.Now())
	default:
		return // cancelled or already handled
	}

	if err := p.handle(ctx, job, req.password); err != nil && ctx.Err() == nil {
		p.d.Log.Warn("process job", "job", job.ID, "error", err)
		p.fail(parent, job, failureMessage(err))
	}
	if ctx.Err() != nil && parent.Err() == nil {
		p.d.Log.Info("processing discarded", "job", job.ID)
	}
}

func (p *Processor) handle(ctx context.Context, job *domain.UploadJob, password string) error {
	if job.VolumeSet == "" { // not yet sorted into a volume set
		if v, ok := domain.ParseVolume(job.FileName); ok {
			return p.collectPart(ctx, job, v)
		}
		if domain.IsLegacyRarVolume(job.FileName) {
			return errLegacyVolume
		}
	}
	header, err := p.d.Staging.ReadHeader(job.StoragePath, 8)
	if err != nil {
		return fmt.Errorf("read upload: %w", err)
	}
	if _, isArchive := domain.DetectArchive(header); !isArchive {
		return p.adoptRaw(ctx, job)
	}
	return p.extract(ctx, job, password)
}

// collectPart moves a volume into its set's folder and waits for the rest.
func (p *Processor) collectPart(ctx context.Context, job *domain.UploadJob, v domain.Volume) error {
	dest := filepath.Join(p.d.Staging.VolumeDir(v.Set), job.FileName)
	if err := p.d.Uploads.Take(ctx, job.ID, dest); err != nil {
		return fmt.Errorf("collect volume: %w", err)
	}
	if err := job.WaitForParts(v, dest, p.d.Now()); err != nil {
		return err
	}
	if err := p.save(ctx, job); err != nil {
		return err
	}
	return p.tryCompleteSet(ctx, v.Set)
}

// tryCompleteSet promotes the set's first volume once every part is there.
// 7-Zip refuses to open a split archive with missing parts, so a successful
// listing of the first volume means the set is complete. Header-encrypted
// sets cannot be listed without the password; they are promoted when the
// parts are contiguous, and extraction reports anything still missing.
func (p *Processor) tryCompleteSet(ctx context.Context, set string) error {
	p.setMu.Lock()
	defer p.setMu.Unlock()

	parts, err := p.d.Jobs.ListWaitingParts(ctx, set)
	if err != nil {
		return err
	}
	var first *domain.UploadJob
	for _, part := range parts {
		if part.VolumeIndex == 1 {
			first = part
		}
	}
	if first == nil {
		return nil // keep waiting for the first volume
	}
	_, err = p.d.Extractor.List(ctx, first.StoragePath, "")
	complete := err == nil ||
		((errors.Is(err, ErrPasswordRequired) || errors.Is(err, ErrWrongPassword)) && contiguous(parts))
	if !complete {
		return nil
	}

	for _, part := range parts {
		if part.ID == first.ID {
			continue
		}
		if err := part.MergeInto(first.ID, p.d.Now()); err != nil {
			return err
		}
		if err := p.save(ctx, part); err != nil {
			return err
		}
	}
	if err := first.PartsComplete(p.d.Now()); err != nil {
		return err
	}
	if err := p.save(ctx, first); err != nil {
		return err
	}
	p.enqueue(request{id: first.ID})
	return nil
}

func contiguous(parts []*domain.UploadJob) bool {
	for i, part := range parts { // sorted by volume index
		if part.VolumeIndex != i+1 {
			return false
		}
	}
	return len(parts) > 0
}

// adoptRaw moves a non-archive upload (an .nsp, an .iso...) into staging.
func (p *Processor) adoptRaw(ctx context.Context, job *domain.UploadJob) error {
	if err := p.d.Staging.Reset(job.ID); err != nil {
		return err
	}
	if err := p.d.Uploads.Take(ctx, job.ID, filepath.Join(p.d.Staging.Dir(job.ID), job.FileName)); err != nil {
		return fmt.Errorf("adopt upload: %w", err)
	}
	if err := p.stageItems(ctx, job); err != nil {
		return err
	}
	if err := job.ReadyForReview(p.d.Now()); err != nil {
		return err
	}
	return p.save(ctx, job)
}

func (p *Processor) extract(ctx context.Context, job *domain.UploadJob, password string) error {
	if err := job.StartExtraction(p.d.Now()); err != nil {
		return err
	}
	if err := p.save(ctx, job); err != nil {
		return err
	}

	listing, err := p.d.Extractor.List(ctx, job.StoragePath, password)
	if err == nil && listing.Encrypted() && password == "" {
		err = ErrPasswordRequired
	}
	if handled, perr := p.passwordProblem(ctx, job, err); handled {
		return perr
	}
	if err != nil {
		return err
	}
	if bad := unsafeEntry(listing); bad != "" {
		return fmt.Errorf("%w: %q", errUnsafePath, bad)
	}
	if free, err := p.d.Staging.FreeSpace(); err == nil {
		need := uint64(listing.TotalSize()) + ExtractionMargin //nolint:gosec // sizes are non-negative
		if need > free {
			return fmt.Errorf("%w: se necesitan %s y hay %s libres", errNoSpace, gib(need), gib(free))
		}
	}

	if err := p.d.Staging.Reset(job.ID); err != nil {
		return err
	}
	last := time.Time{}
	warning, err := p.d.Extractor.Extract(ctx, job.StoragePath, p.d.Staging.Dir(job.ID), password, listing, func(pct int) {
		if now := p.d.Now(); now.Sub(last) >= progressInterval {
			last = now
			job.RecordExtractionProgress(pct, now)
			_ = p.save(ctx, job)
		}
	})
	if err != nil {
		_ = p.d.Staging.Remove(job.ID)
		if handled, perr := p.passwordProblem(ctx, job, err); handled {
			return perr
		}
		return err
	}
	if err := p.d.Staging.CheckTree(job.ID); err != nil {
		_ = p.d.Staging.Remove(job.ID)
		return err
	}
	// The archive is no longer needed (RF-03).
	p.deleteArchive(ctx, job)
	if err := p.stageItems(ctx, job); err != nil {
		return err
	}
	if err := job.FinishExtraction(warning, p.d.Now()); err != nil {
		return err
	}
	return p.save(ctx, job)
}

// passwordProblem pauses the job when err is a password error.
func (p *Processor) passwordProblem(ctx context.Context, job *domain.UploadJob, err error) (bool, error) {
	var reason string
	switch {
	case errors.Is(err, ErrPasswordRequired):
		reason = "El comprimido está protegido con contraseña."
	case errors.Is(err, ErrWrongPassword):
		reason = "Contraseña incorrecta."
	default:
		return false, nil
	}
	if err := job.RequirePassword(reason, p.d.Now()); err != nil {
		return true, err
	}
	return true, p.save(ctx, job)
}

func (p *Processor) stageItems(ctx context.Context, job *domain.UploadJob) error {
	profiles, err := p.d.Profiles.Profiles(ctx)
	if err != nil {
		return err
	}
	items, err := scan.Scan(p.d.Staging.FS(job.ID), job.ID, profiles)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	return p.d.Items.Replace(ctx, job.ID, items)
}

func (p *Processor) fail(ctx context.Context, job *domain.UploadJob, reason string) {
	if err := job.Fail(reason, p.d.Now()); err != nil {
		return
	}
	_ = p.save(ctx, job)
	if job.VolumeSet != "" { // a failed set frees its parts
		_ = p.d.Staging.RemoveVolumes(job.VolumeSet)
	}
}

// deleteArchive removes the uploaded archive (or every part of a volume set).
func (p *Processor) deleteArchive(ctx context.Context, job *domain.UploadJob) {
	var err error
	if job.VolumeSet != "" {
		err = p.d.Staging.RemoveVolumes(job.VolumeSet)
	} else {
		err = p.d.Uploads.Delete(ctx, job.ID)
	}
	if err != nil {
		p.d.Log.Warn("delete extracted archive", "job", job.ID, "error", err)
	}
}

func (p *Processor) save(ctx context.Context, job *domain.UploadJob) error {
	if err := p.d.Jobs.Save(ctx, job); err != nil {
		return err
	}
	p.d.Publisher.Publish(*job)
	return nil
}

var (
	errUnsafePath   = errors.New("unsafe path in archive")
	errNoSpace      = errors.New("not enough free space")
	errLegacyVolume = errors.New("legacy rar volume")
)

// failureMessage turns an error into the text shown to the user.
func failureMessage(err error) string {
	switch {
	case errors.Is(err, errNoSpace):
		return "Espacio insuficiente para descomprimir: " + strings.TrimPrefix(err.Error(), errNoSpace.Error()+": ")
	case errors.Is(err, errUnsafePath):
		return "El comprimido contiene rutas peligrosas (fuera de su carpeta) y se descartó."
	case errors.Is(err, ErrCorrupt):
		return "El comprimido está dañado o un archivo no pasó la verificación de integridad."
	case errors.Is(err, errLegacyVolume):
		return "Volúmenes RAR en formato antiguo (.r00, .r01…) no soportados; usa un comprimido de una parte o volúmenes .part1.rar."
	case errors.Is(err, ErrUnsafeEntry):
		return "El comprimido contiene enlaces o archivos especiales y se descartó."
	default:
		return "No se pudo procesar el archivo: " + err.Error()
	}
}

// ErrUnsafeEntry is returned by Staging.CheckTree for links and special files.
var ErrUnsafeEntry = errors.New("unsafe entry in extracted tree")

// unsafeEntry returns the first entry whose path could escape the target.
func unsafeEntry(l Listing) string {
	for _, e := range l.Entries {
		p := strings.ReplaceAll(e.Path, `\`, "/")
		if path.IsAbs(p) || (len(p) > 1 && p[1] == ':') {
			return e.Path
		}
		for seg := range strings.SplitSeq(p, "/") {
			if seg == ".." {
				return e.Path
			}
		}
	}
	return ""
}

func gib(b uint64) string { return fmt.Sprintf("%.1f GB", float64(b)/1e9) }
