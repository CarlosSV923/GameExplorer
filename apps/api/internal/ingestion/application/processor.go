package application

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
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
	Files   domain.StagedFileRepository
	Uploads interface {
		UploadStore
		UploadAdopter
	}
	Extractor Extractor
	Staging   Staging
	Consoles  Consoles
	Library   Library
	Publisher Publisher
	Log       *slog.Logger
	Now       func() time.Time
}

type request struct {
	id       domain.JobID
	password string
}

// Processor takes uploaded jobs to confirmation: gathering multi-volume
// groups, extraction (with password and integrity checks) or adoption of a
// raw file, then validation against the chosen console (RF-03..RF-07).
type Processor struct {
	d     ProcessorDeps
	queue chan request

	mu      sync.Mutex
	running map[domain.JobID]context.CancelFunc

	// groupMu serializes group completion: parts arriving together must
	// not both promote the group.
	groupMu sync.Mutex
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
// (and, for multi-volume groups, its parts). A file taken from the
// unassigned section goes back there.
func (p *Processor) Discard(id domain.JobID) {
	p.mu.Lock()
	if cancel, ok := p.running[id]; ok {
		cancel()
	}
	p.mu.Unlock()
	ctx := context.Background()
	job, err := p.d.Jobs.Get(ctx, id)
	if err == nil {
		p.giveBack(ctx, job)
	}
	if err := p.d.Staging.Remove(id); err != nil {
		p.d.Log.Warn("remove staging", "job", id, "error", err)
	}
	if err != nil || job.GroupID == "" {
		return
	}
	if job.Status == domain.StatusWaitingParts {
		err = p.d.Staging.RemovePart(job.StoragePath)
	} else {
		err = p.d.Staging.RemoveVolumes(job.GroupID)
	}
	if err != nil {
		p.d.Log.Warn("remove volume files", "job", id, "error", err)
	}
}

// giveBack returns the file of a job started from the unassigned section,
// wherever it is now (its source folder, or staging if it was not an
// archive), to the section.
func (p *Processor) giveBack(ctx context.Context, job *domain.UploadJob) {
	if job.UnassignedOrigin == "" {
		return
	}
	src := job.StoragePath
	if !p.d.Staging.Exists(src) {
		return
	}
	err := p.d.Library.PutAside(ctx, SetAsideRequest{
		Source: string(job.ID), Root: filepath.Dir(src), Files: []string{filepath.Base(src)},
		Folder: path.Dir(job.UnassignedOrigin), Origin: job.UnassignedOrigin, Reason: "manual",
	})
	if err != nil {
		p.d.Log.Error("give file back to the unassigned section", "job", job.ID, "error", err)
		return
	}
	_ = p.d.Staging.RemoveSource(job.ID)
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
	if job.GroupID != "" && !strings.HasPrefix(job.StoragePath, p.d.Staging.VolumeDir(job.GroupID)) {
		return p.collectPart(ctx, job)
	}
	if job.GroupID == "" && domain.IsLegacyRarVolume(job.FileName) {
		return errLegacyVolume
	}
	header, err := p.d.Staging.ReadHeader(job.StoragePath, 8)
	if err != nil {
		return fmt.Errorf("read upload: %w", err)
	}
	if _, isArchive := domain.DetectArchive(header); !isArchive {
		if job.GroupID != "" {
			return errNotAGroup
		}
		return p.adoptRaw(ctx, job)
	}
	return p.extract(ctx, job, password)
}

// collectPart moves a part into its group's folder and waits for the rest.
func (p *Processor) collectPart(ctx context.Context, job *domain.UploadJob) error {
	dest := filepath.Join(p.d.Staging.VolumeDir(job.GroupID), job.FileName)
	if err := p.d.Uploads.Take(ctx, job.ID, dest); err != nil {
		return fmt.Errorf("collect part: %w", err)
	}
	if err := job.WaitForParts(dest, p.d.Now()); err != nil {
		return err
	}
	if err := p.save(ctx, job); err != nil {
		return err
	}
	return p.tryCompleteGroup(ctx, job.GroupID, job.GroupSize)
}

// tryCompleteGroup promotes the group's first volume once every part has
// arrived. Parts that do not form one archive fail together (RF-03a).
func (p *Processor) tryCompleteGroup(ctx context.Context, group string, size int) error {
	p.groupMu.Lock()
	defer p.groupMu.Unlock()

	jobs, err := p.d.Jobs.ListGroup(ctx, group)
	if err != nil {
		return err
	}
	var parts []*domain.UploadJob
	for _, j := range jobs {
		if j.Status == domain.StatusWaitingParts {
			parts = append(parts, j)
		}
	}
	if len(parts) < size {
		return nil // keep waiting
	}
	names := make([]string, len(parts))
	for i, part := range parts {
		names[i] = part.FileName
	}
	first, ok := domain.FirstVolume(names)
	if !ok {
		for _, part := range parts {
			if err := part.Fail("Los archivos no son las partes de un mismo comprimido, o falta alguna parte.", p.d.Now()); err != nil {
				return err
			}
			if err := p.save(ctx, part); err != nil {
				return err
			}
		}
		return p.d.Staging.RemoveVolumes(group)
	}
	for i, part := range parts {
		if i == first {
			continue
		}
		if err := part.MergeInto(parts[first].ID, p.d.Now()); err != nil {
			return err
		}
		if err := p.save(ctx, part); err != nil {
			return err
		}
	}
	if err := parts[first].PartsComplete(p.d.Now()); err != nil {
		return err
	}
	if err := p.save(ctx, parts[first]); err != nil {
		return err
	}
	p.enqueue(request{id: parts[first].ID})
	return nil
}

// adoptRaw moves a non-archive file (an .nsp, an .iso...) into staging.
func (p *Processor) adoptRaw(ctx context.Context, job *domain.UploadJob) error {
	if err := p.d.Staging.Reset(job.ID); err != nil {
		return err
	}
	dest := filepath.Join(p.d.Staging.Dir(job.ID), job.FileName)
	if job.UnassignedOrigin != "" {
		if err := p.d.Staging.Adopt(job.StoragePath, dest); err != nil {
			return fmt.Errorf("adopt file: %w", err)
		}
		job.StoragePath = dest // where to give it back from, if cancelled
	} else if err := p.d.Uploads.Take(ctx, job.ID, dest); err != nil {
		return fmt.Errorf("adopt upload: %w", err)
	}
	return p.validate(ctx, job, "")
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
	// The archive is no longer needed (RF-06); one taken from the unassigned
	// section stays until the job ends, to give it back if cancelled.
	if job.UnassignedOrigin == "" {
		p.deleteArchive(ctx, job)
	}
	return p.validate(ctx, job, warning)
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

// validate lists the staged files and checks them against the job's
// console (RF-07).
func (p *Processor) validate(ctx context.Context, job *domain.UploadJob, warning string) error {
	files, err := listFiles(p.d.Staging.FS(job.ID), job.ID)
	if err != nil {
		return fmt.Errorf("list files: %w", err)
	}
	if err := p.d.Files.Replace(ctx, job.ID, files); err != nil {
		return err
	}
	rules, known, err := p.d.Consoles.Rules(ctx)
	if err != nil {
		return err
	}
	_, reason := domain.Validate(rules[job.Console], files, known)
	if err := job.Validated(job.Console, reason, p.d.Now()); err != nil {
		return err
	}
	job.Warning = warning
	return p.save(ctx, job)
}

// listFiles returns every regular file of an upload, skipping the hidden
// ones and macOS resource forks (junk that zips made on a Mac carry).
func listFiles(fsys fs.FS, id domain.JobID) ([]domain.StagedFile, error) {
	var out []domain.StagedFile
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if p != "." && (strings.HasPrefix(name, ".") || name == "__MACOSX") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		out = append(out, domain.StagedFile{JobID: id, Path: p, Size: info.Size()})
		return nil
	})
	slices.SortFunc(out, func(a, b domain.StagedFile) int { return strings.Compare(a.Path, b.Path) })
	return out, err
}

func (p *Processor) fail(ctx context.Context, job *domain.UploadJob, reason string) {
	if err := job.Fail(reason, p.d.Now()); err != nil {
		return
	}
	_ = p.save(ctx, job)
	p.giveBack(ctx, job)
	if job.GroupID != "" { // a failed group frees its parts
		_ = p.d.Staging.RemoveVolumes(job.GroupID)
	}
}

// deleteArchive removes the uploaded archive (or every part of a group).
func (p *Processor) deleteArchive(ctx context.Context, job *domain.UploadJob) {
	var err error
	if job.GroupID != "" {
		err = p.d.Staging.RemoveVolumes(job.GroupID)
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
	errNotAGroup    = errors.New("group part is not an archive")
)

// failureMessage turns an error into the text shown to the user.
func failureMessage(err error) string {
	switch {
	case errors.Is(err, errNoSpace):
		return "Espacio insuficiente para descomprimir: " + strings.TrimPrefix(err.Error(), errNoSpace.Error()+": ")
	case errors.Is(err, errUnsafePath):
		return "El comprimido contiene rutas peligrosas (fuera de su carpeta) y se descartó."
	case errors.Is(err, ErrCorrupt):
		return "El comprimido está dañado, falta alguna parte o un archivo no pasó la verificación de integridad."
	case errors.Is(err, errLegacyVolume):
		return "Volúmenes RAR en formato antiguo (.r00, .r01…) no soportados; usa un comprimido de una parte o volúmenes .part1.rar."
	case errors.Is(err, errNotAGroup):
		return "Los archivos no son las partes de un mismo comprimido."
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
