// Package http adapts the ingestion use cases to the generated HTTP contract.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpapi"
)

// heartbeat keeps idle event streams open through proxies.
const heartbeat = 25 * time.Second

// Subscriber gives access to live job changes.
type Subscriber interface {
	Subscribe() (<-chan domain.UploadJob, func())
}

// PasswordSubmitter retries encrypted archives (implemented by the Processor).
type PasswordSubmitter interface {
	SubmitPassword(ctx context.Context, id domain.JobID, password string) (*domain.UploadJob, error)
}

// Handler implements the job operations of httpapi.StrictServerInterface.
type Handler struct {
	svc       *application.Service
	passwords PasswordSubmitter
	sub       Subscriber
	shutdown  <-chan struct{}
}

// NewHandler builds the handler. shutdown closes when the server stops, so
// open event streams end instead of delaying the graceful shutdown.
func NewHandler(svc *application.Service, passwords PasswordSubmitter, sub Subscriber, shutdown <-chan struct{}) *Handler {
	return &Handler{svc: svc, passwords: passwords, sub: sub, shutdown: shutdown}
}

// ListJobItems implements httpapi.StrictServerInterface.
func (h *Handler) ListJobItems(ctx context.Context, req httpapi.ListJobItemsRequestObject) (httpapi.ListJobItemsResponseObject, error) {
	items, err := h.svc.Items(ctx, domain.JobID(req.Id))
	if errors.Is(err, domain.ErrJobNotFound) {
		return httpapi.ListJobItems404ApplicationProblemPlusJSONResponse{
			NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, "Not Found", "La subida no existe.")),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(httpapi.ListJobItems200JSONResponse, 0, len(items))
	for _, it := range items {
		out = append(out, itemToAPI(it))
	}
	return out, nil
}

// SubmitJobPassword implements httpapi.StrictServerInterface.
func (h *Handler) SubmitJobPassword(ctx context.Context, req httpapi.SubmitJobPasswordRequestObject) (httpapi.SubmitJobPasswordResponseObject, error) {
	if req.Body == nil || req.Body.Password == "" {
		return httpapi.SubmitJobPassword400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(problem(http.StatusBadRequest, "Bad Request", "Escribe la contraseña.")),
		}, nil
	}
	job, err := h.passwords.SubmitPassword(ctx, domain.JobID(req.Id), req.Body.Password)
	switch {
	case errors.Is(err, domain.ErrJobNotFound):
		return httpapi.SubmitJobPassword404ApplicationProblemPlusJSONResponse{
			NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, "Not Found", "La subida no existe.")),
		}, nil
	case errors.Is(err, application.ErrNotWaitingForPassword):
		return httpapi.SubmitJobPassword409ApplicationProblemPlusJSONResponse{
			ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(problem(http.StatusConflict, "Conflict", "Esta subida no está esperando una contraseña.")),
		}, nil
	case err != nil:
		return nil, err
	}
	return httpapi.SubmitJobPassword202JSONResponse(ToAPI(*job)), nil
}

func itemToAPI(it domain.StagedItem) httpapi.StagedItem {
	out := httpapi.StagedItem{
		Path:       it.Path,
		Shape:      httpapi.StagedItemShape(it.Shape),
		Parts:      it.Parts,
		Size:       it.Size,
		Ignored:    it.Ignored,
		Consoles:   it.Consoles,
		Confidence: httpapi.StagedItemConfidence(it.Confidence),
	}
	if it.SuggestedKind != "" {
		k := httpapi.ItemKind(it.SuggestedKind)
		out.SuggestedKind = &k
	}
	optional := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	out.TitleId, out.VersionCode, out.DisplayVersion = optional(it.TitleID), optional(it.VersionCode), optional(it.DisplayVersion)
	if it.DiscNumber > 0 {
		n := it.DiscNumber
		out.DiscNumber = &n
	}
	return out
}

// ListJobs implements httpapi.StrictServerInterface.
func (h *Handler) ListJobs(ctx context.Context, _ httpapi.ListJobsRequestObject) (httpapi.ListJobsResponseObject, error) {
	jobs, err := h.svc.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(httpapi.ListJobs200JSONResponse, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, ToAPI(*j))
	}
	return out, nil
}

// GetJob implements httpapi.StrictServerInterface.
func (h *Handler) GetJob(ctx context.Context, req httpapi.GetJobRequestObject) (httpapi.GetJobResponseObject, error) {
	job, err := h.svc.Get(ctx, domain.JobID(req.Id))
	if errors.Is(err, domain.ErrJobNotFound) {
		return httpapi.GetJob404ApplicationProblemPlusJSONResponse{
			NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, "Not Found", "La subida no existe.")),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return httpapi.GetJob200JSONResponse(ToAPI(*job)), nil
}

// CancelJob implements httpapi.StrictServerInterface.
func (h *Handler) CancelJob(ctx context.Context, req httpapi.CancelJobRequestObject) (httpapi.CancelJobResponseObject, error) {
	job, err := h.svc.Cancel(ctx, domain.JobID(req.Id))
	switch {
	case errors.Is(err, domain.ErrJobNotFound):
		return httpapi.CancelJob404ApplicationProblemPlusJSONResponse{
			NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, "Not Found", "La subida no existe.")),
		}, nil
	case errors.Is(err, application.ErrCannotCancel):
		return httpapi.CancelJob409ApplicationProblemPlusJSONResponse{
			ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(problem(http.StatusConflict, "Conflict", "La subida ya terminó o se está guardando; no se puede cancelar.")),
		}, nil
	case err != nil:
		return nil, err
	}
	return httpapi.CancelJob200JSONResponse(ToAPI(*job)), nil
}

// StreamJobEvents implements httpapi.StrictServerInterface.
func (h *Handler) StreamJobEvents(ctx context.Context, _ httpapi.StreamJobEventsRequestObject) (httpapi.StreamJobEventsResponseObject, error) {
	// Subscribe before reading the snapshot so no change falls in between.
	events, unsubscribe := h.sub.Subscribe()
	jobs, err := h.svc.List(ctx)
	if err != nil {
		unsubscribe()
		return nil, err
	}
	return eventStream{ctx: ctx, shutdown: h.shutdown, events: events, unsubscribe: unsubscribe, snapshot: jobs}, nil
}

// eventStream writes server-sent events. It replaces the generated response
// type, which only flushes when the writer itself is an http.Flusher; our
// middleware wraps the writer, and http.ResponseController sees through it.
type eventStream struct {
	ctx         context.Context
	shutdown    <-chan struct{}
	events      <-chan domain.UploadJob
	unsubscribe func()
	snapshot    []*domain.UploadJob
}

func (s eventStream) VisitStreamJobEventsResponse(w http.ResponseWriter) error {
	defer s.unsubscribe()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no") // disable buffering in nginx-style proxies
	w.WriteHeader(http.StatusOK)
	rc := http.NewResponseController(w)

	for _, j := range s.snapshot {
		if !j.Status.Terminal() {
			if err := writeEvent(w, *j); err != nil {
				return err
			}
		}
	}
	if err := rc.Flush(); err != nil {
		return err
	}

	tick := time.NewTicker(heartbeat)
	defer tick.Stop()
	for {
		select {
		case <-s.ctx.Done():
			return nil
		case <-s.shutdown:
			return nil
		case <-tick.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return nil //nolint:nilerr // the client went away
			}
		case job, ok := <-s.events:
			if !ok {
				return nil
			}
			if err := writeEvent(w, job); err != nil {
				return nil //nolint:nilerr // the client went away
			}
		}
		if err := rc.Flush(); err != nil {
			return nil //nolint:nilerr // the client went away
		}
	}
}

func writeEvent(w http.ResponseWriter, job domain.UploadJob) error {
	data, err := json.Marshal(ToAPI(job))
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "event: job\ndata: %s\n\n", data)
	return err
}

// ToAPI maps a job to its HTTP representation.
func ToAPI(j domain.UploadJob) httpapi.UploadJob {
	out := httpapi.UploadJob{
		Id:            string(j.ID),
		FileName:      j.FileName,
		Size:          j.Size,
		Received:      j.Received,
		Status:        httpapi.JobStatus(j.Status),
		Progress:      &j.Progress,
		OriginConsole: j.OriginConsole,
		CreatedAt:     j.CreatedAt,
		UpdatedAt:     j.UpdatedAt,
	}
	if j.Error != "" {
		e := j.Error
		out.Error = &e
	}
	if j.Warning != "" {
		w := j.Warning
		out.Warning = &w
	}
	return out
}

func problem(status int, title, detail string) httpapi.Problem {
	return httpapi.Problem{Status: status, Title: title, Detail: &detail}
}
