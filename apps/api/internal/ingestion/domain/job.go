// Package domain is the ingestion model: an UploadJob follows one uploaded
// file from the first byte to its place in the library.
package domain

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode/utf8"
)

// JobID identifies a job. It is the tus upload id, so the browser can resume
// an upload and the server can relate both without a mapping table.
type JobID string

// Status is a step of the ingestion pipeline (spec §3, RF-01..RF-13).
type Status string

// Pipeline steps. Phase 3 drives Uploading → Uploaded; extraction, review and
// commit (phases 4-5) move the job further.
const (
	StatusUploading Status = "uploading"
	StatusUploaded  Status = "uploaded"
	// StatusWaitingParts: a part of a multi-volume archive waits for the rest.
	StatusWaitingParts Status = "waiting_parts"
	// StatusMerged: a part whose content now belongs to the set's first volume job.
	StatusMerged        Status = "merged"
	StatusExtracting    Status = "extracting"
	StatusNeedsPassword Status = "needs_password"
	StatusReview        Status = "review"
	StatusCommitting    Status = "committing"
	StatusDone          Status = "done"
	StatusFailed        Status = "failed"
	StatusCancelled     Status = "cancelled"
)

// transitions lists the allowed next states of each state.
var transitions = map[Status][]Status{
	StatusUploading:     {StatusUploaded, StatusFailed, StatusCancelled},
	StatusUploaded:      {StatusExtracting, StatusReview, StatusWaitingParts, StatusFailed, StatusCancelled},
	StatusWaitingParts:  {StatusUploaded, StatusMerged, StatusFailed, StatusCancelled},
	StatusExtracting:    {StatusReview, StatusNeedsPassword, StatusFailed, StatusCancelled},
	StatusNeedsPassword: {StatusExtracting, StatusFailed, StatusCancelled},
	StatusReview:        {StatusCommitting, StatusFailed, StatusCancelled},
	StatusCommitting:    {StatusDone, StatusReview, StatusFailed},
}

// Terminal reports whether no further transition is possible.
func (s Status) Terminal() bool {
	_, ok := transitions[s]
	return !ok
}

// CanTransitionTo reports whether s → next is a valid move.
func (s Status) CanTransitionTo(next Status) bool {
	for _, allowed := range transitions[s] {
		if allowed == next {
			return true
		}
	}
	return false
}

var (
	// ErrInvalidTransition is returned for moves the state machine forbids.
	ErrInvalidTransition = errors.New("invalid job state transition")
	// ErrJobNotFound is returned when a job does not exist.
	ErrJobNotFound = errors.New("upload job not found")
	// ErrInvalidFileName is returned for unusable upload file names.
	ErrInvalidFileName = errors.New("invalid file name")
)

// UploadJob is the aggregate root of the ingestion context.
type UploadJob struct {
	ID       JobID
	FileName string
	Size     int64
	Received int64
	Status   Status
	Error    string
	// Progress is the extraction percentage (0-100) while extracting.
	Progress int
	// Warning is a non-fatal note, e.g. archive attributes not applied.
	Warning   string
	CreatedAt time.Time
	UpdatedAt time.Time

	// OriginConsole is the console screen the upload started from (RF-08), if any.
	OriginConsole *string
	// StoragePath is where the uploaded bytes live (opaque to the domain).
	StoragePath string

	// VolumeSet and VolumeIndex are set for parts of multi-volume archives.
	VolumeSet   string
	VolumeIndex int
	// MergedInto is the job (first volume) that took over this part.
	MergedInto JobID
}

// NewUploadJob starts a job for a file the browser announced.
func NewUploadJob(id JobID, fileName string, size int64, origin *string, now time.Time) (*UploadJob, error) {
	name, err := CleanFileName(fileName)
	if err != nil {
		return nil, err
	}
	if size < 0 {
		return nil, fmt.Errorf("negative size %d", size)
	}
	return &UploadJob{
		ID: id, FileName: name, Size: size, Status: StatusUploading,
		OriginConsole: origin, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// RecordProgress stores how many bytes arrived so far.
func (j *UploadJob) RecordProgress(received int64, now time.Time) {
	if j.Status != StatusUploading || received < j.Received {
		return
	}
	j.Received = min(received, j.Size)
	j.UpdatedAt = now
}

// MarkUploaded records that every byte arrived and where they are.
func (j *UploadJob) MarkUploaded(storagePath string, now time.Time) error {
	if err := j.moveTo(StatusUploaded, now); err != nil {
		return err
	}
	j.Received = j.Size
	j.StoragePath = storagePath
	return nil
}

// StartExtraction begins (or retries, after a password) extracting the archive.
func (j *UploadJob) StartExtraction(now time.Time) error {
	if err := j.moveTo(StatusExtracting, now); err != nil {
		return err
	}
	j.Progress, j.Error, j.Warning = 0, "", ""
	return nil
}

// ResumeAfterRestart puts a job whose extraction was interrupted (the process
// stopped mid-way) back to uploaded, so it can be extracted from scratch.
func (j *UploadJob) ResumeAfterRestart(now time.Time) {
	if j.Status == StatusExtracting {
		j.Status, j.Progress, j.UpdatedAt = StatusUploaded, 0, now
	}
}

// RecordExtractionProgress stores the extraction percentage.
func (j *UploadJob) RecordExtractionProgress(pct int, now time.Time) {
	if j.Status != StatusExtracting || pct <= j.Progress {
		return
	}
	j.Progress = min(pct, 100)
	j.UpdatedAt = now
}

// RequirePassword pauses the job until the user provides the archive password.
func (j *UploadJob) RequirePassword(reason string, now time.Time) error {
	if err := j.moveTo(StatusNeedsPassword, now); err != nil {
		return err
	}
	j.Progress, j.Error = 0, reason
	return nil
}

// FinishExtraction moves the job to review once its items are staged.
func (j *UploadJob) FinishExtraction(warning string, now time.Time) error {
	if err := j.moveTo(StatusReview, now); err != nil {
		return err
	}
	j.Progress, j.Warning = 100, warning
	return nil
}

// ReadyForReview moves an uploaded raw (non-archive) file straight to review.
func (j *UploadJob) ReadyForReview(now time.Time) error {
	return j.moveTo(StatusReview, now)
}

// WaitForParts parks a multi-volume part until the whole set has arrived.
func (j *UploadJob) WaitForParts(v Volume, storagePath string, now time.Time) error {
	if err := j.moveTo(StatusWaitingParts, now); err != nil {
		return err
	}
	j.VolumeSet, j.VolumeIndex, j.StoragePath = v.Set, v.Index, storagePath
	return nil
}

// PartsComplete makes the first volume's job ready to extract the whole set.
func (j *UploadJob) PartsComplete(now time.Time) error {
	return j.moveTo(StatusUploaded, now)
}

// MergeInto closes a part whose data is now handled by the first volume's job.
func (j *UploadJob) MergeInto(primary JobID, now time.Time) error {
	if err := j.moveTo(StatusMerged, now); err != nil {
		return err
	}
	j.MergedInto = primary
	return nil
}

// StartCommit begins moving the reviewed items into the library.
func (j *UploadJob) StartCommit(now time.Time) error {
	if err := j.moveTo(StatusCommitting, now); err != nil {
		return err
	}
	j.Error = ""
	return nil
}

// FinishCommit records that the items are in the library.
func (j *UploadJob) FinishCommit(now time.Time) error {
	return j.moveTo(StatusDone, now)
}

// AbortCommit returns the job to review after the commit was undone; reason
// ("" when the request was simply refused) is shown in the review.
func (j *UploadJob) AbortCommit(reason string, now time.Time) error {
	if err := j.moveTo(StatusReview, now); err != nil {
		return err
	}
	j.Error = reason
	return nil
}

// Cancel stops the job at the user's request.
func (j *UploadJob) Cancel(now time.Time) error {
	return j.moveTo(StatusCancelled, now)
}

// Fail stops the job with a human-readable reason.
func (j *UploadJob) Fail(reason string, now time.Time) error {
	if err := j.moveTo(StatusFailed, now); err != nil {
		return err
	}
	j.Error = reason
	return nil
}

func (j *UploadJob) moveTo(next Status, now time.Time) error {
	if !j.Status.CanTransitionTo(next) {
		return fmt.Errorf("%w: %s → %s", ErrInvalidTransition, j.Status, next)
	}
	j.Status = next
	j.UpdatedAt = now
	return nil
}

// CleanFileName keeps only the base name the browser sent (never a path), so
// it can be shown and later used to infer the console and item kind.
func CleanFileName(name string) (string, error) {
	name = strings.ReplaceAll(name, `\`, "/")
	name = strings.TrimSpace(path.Base(name))
	if name == "" || name == "." || name == "/" || name == ".." || !utf8.ValidString(name) || len(name) > 255 {
		return "", fmt.Errorf("%w: %q", ErrInvalidFileName, name)
	}
	if strings.ContainsFunc(name, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return "", fmt.Errorf("%w: control characters", ErrInvalidFileName)
	}
	return name, nil
}
