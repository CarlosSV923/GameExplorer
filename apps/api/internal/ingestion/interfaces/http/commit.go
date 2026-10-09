package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpapi"
)

// CommitService confirms, re-validates and sets aside uploads (implemented
// by application.Committer).
type CommitService interface {
	Files(ctx context.Context, id domain.JobID) ([]application.FileView, error)
	Plan(ctx context.Context, id domain.JobID, req application.CommitInput) (application.CommitPlan, error)
	Commit(ctx context.Context, id domain.JobID, req application.CommitInput) (*domain.UploadJob, application.CommitResult, error)
	ChangeConsole(ctx context.Context, id domain.JobID, console string) (*domain.UploadJob, error)
	Resolve(ctx context.Context, id domain.JobID, r application.Resolution) (*domain.UploadJob, error)
}

// WithCommits sets the commit use case.
func (h *Handler) WithCommits(c CommitService) *Handler {
	h.commits = c
	return h
}

// ListJobFiles implements httpapi.StrictServerInterface.
func (h *Handler) ListJobFiles(ctx context.Context, req httpapi.ListJobFilesRequestObject) (httpapi.ListJobFilesResponseObject, error) {
	files, err := h.commits.Files(ctx, domain.JobID(req.Id))
	if errors.Is(err, domain.ErrJobNotFound) {
		return httpapi.ListJobFiles404ApplicationProblemPlusJSONResponse{
			NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(problem(http.StatusNotFound, "Not Found", "La subida no existe.")),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	out := make(httpapi.ListJobFiles200JSONResponse, 0, len(files))
	for _, f := range files {
		sf := httpapi.StagedFile{Path: f.Path, Size: f.Size, Valid: f.Valid, Consoles: f.Consoles}
		if f.Unassigned != nil {
			inPlace := true
			sf.InPlace = &inPlace
		}
		out = append(out, sf)
	}
	return out, nil
}

// PlanJobCommit implements httpapi.StrictServerInterface.
func (h *Handler) PlanJobCommit(ctx context.Context, req httpapi.PlanJobCommitRequestObject) (httpapi.PlanJobCommitResponseObject, error) {
	if req.Body == nil {
		return httpapi.PlanJobCommit400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: badRequest("Falta el cuerpo de la petición.")}, nil
	}
	plan, err := h.commits.Plan(ctx, domain.JobID(req.Id), commitFiles(*req.Body))
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
	job, res, err := h.commits.Commit(ctx, domain.JobID(req.Id), commitFiles(*req.Body))
	if err == nil {
		return httpapi.CommitJob200JSONResponse{
			Job: ToAPI(*job), GameId: res.GameID, Path: res.Path,
			Stored: res.Stored, Replaced: res.Replaced, Skipped: res.Skipped,
		}, nil
	}
	status, p := commitProblem(err)
	if status == http.StatusInternalServerError && job != nil && job.Error != "" {
		// The commit failed and was undone (or could not be): say why.
		p = problem(http.StatusInternalServerError, "Internal Server Error", job.Error)
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

// ChangeJobConsole implements httpapi.StrictServerInterface.
func (h *Handler) ChangeJobConsole(ctx context.Context, req httpapi.ChangeJobConsoleRequestObject) (httpapi.ChangeJobConsoleResponseObject, error) {
	if req.Body == nil {
		return httpapi.ChangeJobConsole400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: badRequest("Falta el cuerpo de la petición.")}, nil
	}
	job, err := h.commits.ChangeConsole(ctx, domain.JobID(req.Id), req.Body.Console)
	if err == nil {
		return httpapi.ChangeJobConsole200JSONResponse(ToAPI(*job)), nil
	}
	switch status, p := commitProblem(err); status {
	case http.StatusBadRequest:
		return httpapi.ChangeJobConsole400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.ChangeJobConsole404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.ChangeJobConsole409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// ResolveJob implements httpapi.StrictServerInterface.
func (h *Handler) ResolveJob(ctx context.Context, req httpapi.ResolveJobRequestObject) (httpapi.ResolveJobResponseObject, error) {
	if req.Body == nil {
		return httpapi.ResolveJob400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: badRequest("Falta el cuerpo de la petición.")}, nil
	}
	job, err := h.commits.Resolve(ctx, domain.JobID(req.Id), application.Resolution(req.Body.Action))
	if err == nil {
		return httpapi.ResolveJob200JSONResponse(ToAPI(*job)), nil
	}
	switch status, p := commitProblem(err); status {
	case http.StatusBadRequest:
		return httpapi.ResolveJob400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.ResolveJob404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.ResolveJob409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	default:
		detail := "No se pudieron apartar los archivos y se deshizo el cambio: " + err.Error()
		return httpapi.ResolveJob500ApplicationProblemPlusJSONResponse{InternalErrorApplicationProblemPlusJSONResponse: httpapi.InternalErrorApplicationProblemPlusJSONResponse(problem(http.StatusInternalServerError, "Internal Server Error", detail))}, nil
	}
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
	case errors.Is(err, application.ErrNotConfirmable):
		return http.StatusConflict, problem(http.StatusConflict, "Conflict", "La subida no está esperando confirmación.")
	case errors.Is(err, application.ErrNotInvalid):
		return http.StatusConflict, problem(http.StatusConflict, "Conflict", "La subida encaja en su consola; no hay nada que apartar.")
	case errors.Is(err, domain.ErrInvalidTransition):
		return http.StatusConflict, problem(http.StatusConflict, "Conflict", "La subida cambió de estado; vuelve a intentarlo.")
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

func commitFiles(b httpapi.CommitRequest) application.CommitInput {
	in := application.CommitInput{Files: make([]application.CommitFile, 0, len(b.Files))}
	if b.Renumber != nil {
		for _, r := range *b.Renumber {
			in.Renumber = append(in.Renumber, application.Renumbering{Item: r.ItemId, Label: r.Label})
		}
	}
	for _, f := range b.Files {
		cf := application.CommitFile{Path: f.Path}
		if f.Kind != nil {
			cf.Kind = string(*f.Kind)
		}
		if f.Label != nil {
			cf.Label = *f.Label
		}
		if f.OnDuplicate != nil {
			cf.OnDuplicate = string(*f.OnDuplicate)
		}
		if f.Skip != nil {
			cf.Skip = *f.Skip
		}
		in.Files = append(in.Files, cf)
	}
	return in
}

func planToAPI(p application.CommitPlan) httpapi.CommitPlan {
	out := httpapi.CommitPlan{
		Console: p.Console, Title: p.Title, Folder: p.Folder,
		Existing:  make([]httpapi.LibraryItem, 0, len(p.Existing)),
		Files:     make([]httpapi.PlannedFile, 0, len(p.Files)),
		Discarded: append([]string{}, p.Discarded...),
	}
	if p.GameID != 0 {
		id := p.GameID
		out.GameId = &id
	}
	for _, e := range p.Existing {
		out.Existing = append(out.Existing, libraryItemToAPI(e))
	}
	for _, f := range p.Files {
		pf := httpapi.PlannedFile{Path: f.Path, File: f.File, Action: httpapi.PlanAction(f.Action)}
		if f.Duplicate != nil {
			d := libraryItemToAPI(*f.Duplicate)
			pf.Duplicate = &d
		}
		out.Files = append(out.Files, pf)
	}
	if len(p.Renamed) > 0 {
		renamed := make([]httpapi.RenamedItem, 0, len(p.Renamed))
		for _, r := range p.Renamed {
			renamed = append(renamed, httpapi.RenamedItem{Item: libraryItemToAPI(r.Item), File: r.File})
		}
		out.Renamed = &renamed
	}
	return out
}

func libraryItemToAPI(e application.ExistingItem) httpapi.LibraryItem {
	out := httpapi.LibraryItem{Id: e.ID, Kind: httpapi.ItemKind(e.Kind), File: e.File, Size: e.Size, CreatedAt: e.CreatedAt}
	if e.Label != "" {
		l := e.Label
		out.Label = &l
	}
	return out
}
