package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpapi"
)

// CommitService plans and performs commits (implemented by application.Committer).
type CommitService interface {
	Plan(ctx context.Context, id domain.JobID, req application.CommitRequest) (application.CommitPlan, error)
	Commit(ctx context.Context, id domain.JobID, req application.CommitRequest) (*domain.UploadJob, application.CommitResult, error)
}

// WithCommits sets the commit use case.
func (h *Handler) WithCommits(c CommitService) *Handler {
	h.commits = c
	return h
}

// PlanJobCommit implements httpapi.StrictServerInterface.
func (h *Handler) PlanJobCommit(ctx context.Context, req httpapi.PlanJobCommitRequestObject) (httpapi.PlanJobCommitResponseObject, error) {
	if req.Body == nil {
		return httpapi.PlanJobCommit400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: badRequest("Falta el cuerpo de la petición.")}, nil
	}
	plan, err := h.commits.Plan(ctx, domain.JobID(req.Id), commitRequest(*req.Body))
	if err == nil {
		return httpapi.PlanJobCommit200JSONResponse(planToAPI(plan)), nil
	}
	status, p := commitProblem(err)
	switch status {
	case http.StatusBadRequest:
		return httpapi.PlanJobCommit400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.PlanJobCommit404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.PlanJobCommit409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusBadGateway:
		return httpapi.PlanJobCommit502ApplicationProblemPlusJSONResponse{BadGatewayApplicationProblemPlusJSONResponse: httpapi.BadGatewayApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusServiceUnavailable:
		return httpapi.PlanJobCommit503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse: httpapi.ServiceUnavailableApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// CommitJob implements httpapi.StrictServerInterface.
func (h *Handler) CommitJob(ctx context.Context, req httpapi.CommitJobRequestObject) (httpapi.CommitJobResponseObject, error) {
	if req.Body == nil {
		return httpapi.CommitJob400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: badRequest("Falta el cuerpo de la petición.")}, nil
	}
	job, res, err := h.commits.Commit(ctx, domain.JobID(req.Id), commitRequest(*req.Body))
	if err == nil {
		return httpapi.CommitJob200JSONResponse{
			Job: ToAPI(*job), GameId: res.GameID, Path: res.Path,
			Stored: res.Stored, Replaced: res.Replaced, Skipped: res.Skipped,
		}, nil
	}
	status, p := commitProblem(err)
	if status == http.StatusInternalServerError && job != nil && job.Error != "" {
		// The commit failed and was undone (or could not be): say why.
		detail := job.Error
		p = problem(http.StatusInternalServerError, "Internal Server Error", detail)
		return httpapi.CommitJob500ApplicationProblemPlusJSONResponse{InternalErrorApplicationProblemPlusJSONResponse: httpapi.InternalErrorApplicationProblemPlusJSONResponse(p)}, nil
	}
	switch status {
	case http.StatusBadRequest:
		return httpapi.CommitJob400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.CommitJob404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.CommitJob409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusBadGateway:
		return httpapi.CommitJob502ApplicationProblemPlusJSONResponse{BadGatewayApplicationProblemPlusJSONResponse: httpapi.BadGatewayApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusServiceUnavailable:
		return httpapi.CommitJob503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse: httpapi.ServiceUnavailableApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

func badRequest(detail string) httpapi.BadRequestApplicationProblemPlusJSONResponse {
	return httpapi.BadRequestApplicationProblemPlusJSONResponse(problem(http.StatusBadRequest, "Bad Request", detail))
}

// commitProblem maps a commit error to its HTTP status and problem.
func commitProblem(err error) (int, httpapi.Problem) {
	var rejected *application.CommitRejected
	switch {
	case errors.Is(err, domain.ErrJobNotFound):
		return http.StatusNotFound, problem(http.StatusNotFound, "Not Found", "La subida no existe.")
	case errors.Is(err, application.ErrNotInReview):
		return http.StatusConflict, problem(http.StatusConflict, "Conflict", "La subida no está en revisión.")
	case errors.As(err, &rejected):
		status := map[application.RejectReason]int{
			application.RejectInvalid:     http.StatusBadRequest,
			application.RejectConflict:    http.StatusConflict,
			application.RejectUnavailable: http.StatusServiceUnavailable,
			application.RejectUpstream:    http.StatusBadGateway,
		}[rejected.Reason]
		if status == 0 {
			status = http.StatusBadRequest
		}
		return status, problem(status, http.StatusText(status), rejected.Message)
	}
	return http.StatusInternalServerError, problem(http.StatusInternalServerError, "Internal Server Error", "")
}

func commitRequest(b httpapi.CommitRequest) application.CommitRequest {
	out := application.CommitRequest{Console: b.Console, IGDBGameID: b.IgdbGameId}
	for _, it := range b.Items {
		ci := application.CommitItem{Path: it.Path}
		if it.Skip != nil {
			ci.Skip = *it.Skip
		}
		if it.Kind != nil {
			ci.Kind = string(*it.Kind)
		}
		if it.Label != nil {
			ci.Label = *it.Label
		}
		if it.DiscNumber != nil {
			ci.DiscNumber = *it.DiscNumber
		}
		if it.OnDuplicate != nil {
			ci.OnDuplicate = string(*it.OnDuplicate)
		}
		out.Items = append(out.Items, ci)
	}
	return out
}

func planToAPI(p application.CommitPlan) httpapi.CommitPlan {
	out := httpapi.CommitPlan{
		Console: p.Console, Title: p.Title, Folder: p.Folder,
		Existing: make([]httpapi.LibraryItem, 0, len(p.Existing)),
		Items:    make([]httpapi.PlannedItem, 0, len(p.Items)),
	}
	if p.GameID != 0 {
		id := p.GameID
		out.GameId = &id
	}
	for _, e := range p.Existing {
		out.Existing = append(out.Existing, libraryItemToAPI(e))
	}
	for _, it := range p.Items {
		pi := httpapi.PlannedItem{Path: it.Path, Files: it.Files, Action: httpapi.PlannedItemAction(it.Action)}
		if it.Duplicate != nil {
			d := libraryItemToAPI(*it.Duplicate)
			pi.Duplicate = &d
		}
		out.Items = append(out.Items, pi)
	}
	return out
}

func libraryItemToAPI(e application.ExistingItem) httpapi.LibraryItem {
	out := httpapi.LibraryItem{Id: e.ID, Kind: httpapi.ItemKind(e.Kind), Files: e.Files, Size: e.Size}
	if e.Label != "" {
		l := e.Label
		out.Label = &l
	}
	if e.DiscNumber > 0 {
		n := e.DiscNumber
		out.DiscNumber = &n
	}
	return out
}
