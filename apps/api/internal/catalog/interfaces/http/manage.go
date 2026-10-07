package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpapi"
)

// WithManagement sets the use cases that change the library and the consoles.
func (h *Handler) WithManagement(library *application.LibraryService, platforms application.PlatformDirectory, trashRetention time.Duration) *Handler {
	h.library, h.platforms, h.retention = library, platforms, trashRetention
	return h
}

// problemFor maps a management error to a status and a problem; ok is false
// for unexpected errors (answered as a bare 500 by the server).
func problemFor(err error) (int, httpapi.Problem, bool) {
	mk := func(status int, detail string) (int, httpapi.Problem, bool) {
		return status, httpapi.Problem{Status: status, Title: http.StatusText(status), Detail: &detail}, true
	}
	var rej *application.Rejection
	var invalid *application.InvalidConsoleError
	switch {
	case errors.Is(err, domain.ErrGameNotFound):
		return mk(http.StatusNotFound, "El juego no existe.")
	case errors.Is(err, domain.ErrItemNotFound):
		return mk(http.StatusNotFound, "El elemento no existe.")
	case errors.Is(err, domain.ErrTrashEntryNotFound):
		return mk(http.StatusNotFound, "La entrada de la papelera no existe.")
	case errors.Is(err, application.ErrConsoleNotFound):
		return mk(http.StatusNotFound, "La consola no existe.")
	case errors.As(err, &rej):
		status := map[application.RejectReason]int{
			application.RejectInvalid:     http.StatusBadRequest,
			application.RejectConflict:    http.StatusConflict,
			application.RejectUnavailable: http.StatusServiceUnavailable,
			application.RejectUpstream:    http.StatusBadGateway,
		}[rej.Reason]
		return mk(status, rej.Message)
	case errors.As(err, &invalid):
		return mk(http.StatusBadRequest, invalid.Message)
	case errors.Is(err, application.ErrNotMissing):
		return mk(http.StatusConflict, "Solo se puede olvidar un elemento cuyos archivos faltan en el disco.")
	case errors.Is(err, application.ErrConsoleInUse):
		return mk(http.StatusConflict, "La consola tiene juegos (también cuentan los de la papelera).")
	case errors.Is(err, application.ErrBuiltInConsole):
		return mk(http.StatusConflict, "Las consolas de fábrica no se pueden borrar.")
	case errors.Is(err, domain.ErrSlugTaken):
		return mk(http.StatusConflict, "Otra consola ya usa esa carpeta o esa plataforma de IGDB.")
	case errors.Is(err, application.ErrMetadataNotConfigured):
		return mk(http.StatusServiceUnavailable, "IGDB no está configurado (IGDB_CLIENT_ID / IGDB_CLIENT_SECRET).")
	case errors.Is(err, application.ErrMetadataUnavailable):
		return mk(http.StatusBadGateway, "IGDB no responde; inténtalo de nuevo.")
	case errors.Is(err, application.ErrUndoFailed):
		return mk(http.StatusInternalServerError, "No se pudo completar ni deshacer el cambio; revisa la carpeta por SMB. "+
			"Al reiniciar la app se intentará deshacer de nuevo.")
	}
	return 0, httpapi.Problem{}, false
}

// failed is problemFor for operations that may fail half-way: any other
// error was undone, and the user sees why.
func failed(err error) (int, httpapi.Problem) {
	if status, p, ok := problemFor(err); ok {
		return status, p
	}
	detail := "No se pudo completar el cambio y se deshizo: " + err.Error()
	return http.StatusInternalServerError, httpapi.Problem{Status: http.StatusInternalServerError, Title: "Internal Server Error", Detail: &detail}
}

// ---------- trash ----------

// TrashGame implements httpapi.StrictServerInterface.
func (h *Handler) TrashGame(ctx context.Context, req httpapi.TrashGameRequestObject) (httpapi.TrashGameResponseObject, error) {
	err := h.library.TrashGame(ctx, domain.GameID(req.Id))
	if err == nil {
		return httpapi.TrashGame204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.TrashGame404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.TrashGame409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// TrashItem implements httpapi.StrictServerInterface.
func (h *Handler) TrashItem(ctx context.Context, req httpapi.TrashItemRequestObject) (httpapi.TrashItemResponseObject, error) {
	err := h.library.TrashItem(ctx, domain.ItemID(req.Id))
	if err == nil {
		return httpapi.TrashItem204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.TrashItem404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.TrashItem409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// ListTrash implements httpapi.StrictServerInterface.
func (h *Handler) ListTrash(ctx context.Context, _ httpapi.ListTrashRequestObject) (httpapi.ListTrashResponseObject, error) {
	entries, err := h.library.Trash(ctx, h.retention)
	if err != nil {
		return nil, err
	}
	out := make(httpapi.ListTrash200JSONResponse, 0, len(entries))
	for _, e := range entries {
		v := httpapi.TrashEntry{
			Id: int64(e.ID), GameId: int64(e.GameID), Console: string(e.Console), Title: e.Title, Folder: e.Folder,
			WholeGame: e.WholeGame, Reason: httpapi.TrashEntryReason(e.Reason), TrashedAt: e.TrashedAt,
			ExpiresAt: e.ExpiresAt, Size: e.Size, Items: make([]httpapi.LibraryItem, 0, len(e.Items)),
		}
		for _, it := range e.Items {
			v.Items = append(v.Items, ItemToAPI(it))
		}
		out = append(out, v)
	}
	return out, nil
}

// EmptyTrash implements httpapi.StrictServerInterface.
func (h *Handler) EmptyTrash(ctx context.Context, _ httpapi.EmptyTrashRequestObject) (httpapi.EmptyTrashResponseObject, error) {
	if _, err := h.library.EmptyTrash(ctx); err != nil {
		return nil, err
	}
	return httpapi.EmptyTrash204Response{}, nil
}

// DeleteTrashEntry implements httpapi.StrictServerInterface.
func (h *Handler) DeleteTrashEntry(ctx context.Context, req httpapi.DeleteTrashEntryRequestObject) (httpapi.DeleteTrashEntryResponseObject, error) {
	err := h.library.DeleteTrashEntry(ctx, domain.TrashEntryID(req.Id))
	if err == nil {
		return httpapi.DeleteTrashEntry204Response{}, nil
	}
	if status, p, _ := problemFor(err); status == http.StatusNotFound {
		return httpapi.DeleteTrashEntry404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// RestoreTrashEntry implements httpapi.StrictServerInterface.
func (h *Handler) RestoreTrashEntry(ctx context.Context, req httpapi.RestoreTrashEntryRequestObject) (httpapi.RestoreTrashEntryResponseObject, error) {
	var onConflict domain.DuplicateAction
	if req.Body != nil && req.Body.OnConflict != nil {
		onConflict = domain.DuplicateAction(*req.Body.OnConflict)
	}
	res, err := h.library.Restore(ctx, domain.TrashEntryID(req.Id), onConflict)
	if err == nil {
		return httpapi.RestoreTrashEntry200JSONResponse{GameId: int64(res.GameID), Path: res.Path}, nil
	}
	switch status, p := failed(err); status {
	case http.StatusNotFound:
		return httpapi.RestoreTrashEntry404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.RestoreTrashEntry409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	default:
		return httpapi.RestoreTrashEntry500ApplicationProblemPlusJSONResponse{InternalErrorApplicationProblemPlusJSONResponse: httpapi.InternalErrorApplicationProblemPlusJSONResponse(p)}, nil
	}
}

// ---------- integrity ----------

// ForgetItem implements httpapi.StrictServerInterface.
func (h *Handler) ForgetItem(ctx context.Context, req httpapi.ForgetItemRequestObject) (httpapi.ForgetItemResponseObject, error) {
	err := h.library.Forget(ctx, domain.ItemID(req.Id))
	if err == nil {
		return httpapi.ForgetItem204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.ForgetItem404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.ForgetItem409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// CheckLibrary implements httpapi.StrictServerInterface.
func (h *Handler) CheckLibrary(ctx context.Context, _ httpapi.CheckLibraryRequestObject) (httpapi.CheckLibraryResponseObject, error) {
	rep, err := h.library.CheckIntegrity(ctx)
	if err != nil {
		return nil, err
	}
	return httpapi.CheckLibrary200JSONResponse{Checked: rep.Checked, Missing: rep.Missing, Found: rep.Found}, nil
}

// ---------- rematch ----------

func rematchRequest(id int64, b *httpapi.RematchRequest) application.RematchRequest {
	req := application.RematchRequest{GameID: domain.GameID(id), Decisions: map[domain.ItemID]domain.DuplicateAction{}}
	if b == nil {
		return req
	}
	req.IGDBGameID = b.IgdbGameId
	if b.Decisions != nil {
		for _, d := range *b.Decisions {
			req.Decisions[domain.ItemID(d.ItemId)] = domain.DuplicateAction(d.OnDuplicate)
		}
	}
	return req
}

// PlanRematch implements httpapi.StrictServerInterface.
func (h *Handler) PlanRematch(ctx context.Context, req httpapi.PlanRematchRequestObject) (httpapi.PlanRematchResponseObject, error) {
	p, err := h.library.PlanRematch(ctx, rematchRequest(req.Id, req.Body))
	if err == nil {
		out := httpapi.PlanRematch200JSONResponse{Console: p.Console, Title: p.Title, Folder: p.Folder, Items: make([]httpapi.RematchItem, 0, len(p.Items))}
		if p.MergeInto != 0 {
			id := int64(p.MergeInto)
			out.MergeInto = &id
		}
		for _, it := range p.Items {
			ri := httpapi.RematchItem{Item: ItemToAPI(it.Item), Files: it.Files, Action: httpapi.RematchItemAction(it.Action)}
			if it.Duplicate != nil {
				d := ItemToAPI(*it.Duplicate)
				ri.Duplicate = &d
			}
			out.Items = append(out.Items, ri)
		}
		return out, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusBadRequest:
		return httpapi.PlanRematch400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.PlanRematch404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusBadGateway:
		return httpapi.PlanRematch502ApplicationProblemPlusJSONResponse{BadGatewayApplicationProblemPlusJSONResponse: httpapi.BadGatewayApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusServiceUnavailable:
		return httpapi.PlanRematch503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse: httpapi.ServiceUnavailableApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// RematchGame implements httpapi.StrictServerInterface.
func (h *Handler) RematchGame(ctx context.Context, req httpapi.RematchGameRequestObject) (httpapi.RematchGameResponseObject, error) {
	// The renames must finish even if the browser goes away.
	res, err := h.library.Rematch(context.WithoutCancel(ctx), rematchRequest(req.Id, req.Body))
	if err == nil {
		return httpapi.RematchGame200JSONResponse{GameId: int64(res.GameID), Path: res.Path, Merged: res.Merged}, nil
	}
	switch status, p := failed(err); status {
	case http.StatusBadRequest:
		return httpapi.RematchGame400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.RematchGame404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.RematchGame409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusBadGateway:
		return httpapi.RematchGame502ApplicationProblemPlusJSONResponse{BadGatewayApplicationProblemPlusJSONResponse: httpapi.BadGatewayApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusServiceUnavailable:
		return httpapi.RematchGame503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse: httpapi.ServiceUnavailableApplicationProblemPlusJSONResponse(p)}, nil
	default:
		return httpapi.RematchGame500ApplicationProblemPlusJSONResponse{InternalErrorApplicationProblemPlusJSONResponse: httpapi.InternalErrorApplicationProblemPlusJSONResponse(p)}, nil
	}
}

// ---------- consoles ----------

// CreateConsole implements httpapi.StrictServerInterface.
func (h *Handler) CreateConsole(ctx context.Context, req httpapi.CreateConsoleRequestObject) (httpapi.CreateConsoleResponseObject, error) {
	if req.Body == nil {
		return httpapi.CreateConsole400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(badRequest("Falta el cuerpo de la petición."))}, nil
	}
	b := req.Body
	c, err := h.consoles.Create(ctx, b.IgdbPlatformId, application.ConsoleInput{Slug: b.Slug, DisplayName: b.DisplayName, Extensions: b.Extensions}, h.platforms)
	if err == nil {
		return httpapi.CreateConsole201JSONResponse(toAPI(c)), nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusBadRequest:
		return httpapi.CreateConsole400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.CreateConsole409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusBadGateway:
		return httpapi.CreateConsole502ApplicationProblemPlusJSONResponse{BadGatewayApplicationProblemPlusJSONResponse: httpapi.BadGatewayApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusServiceUnavailable:
		return httpapi.CreateConsole503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse: httpapi.ServiceUnavailableApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// UpdateConsole implements httpapi.StrictServerInterface.
func (h *Handler) UpdateConsole(ctx context.Context, req httpapi.UpdateConsoleRequestObject) (httpapi.UpdateConsoleResponseObject, error) {
	if req.Body == nil {
		return httpapi.UpdateConsole400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(badRequest("Falta el cuerpo de la petición."))}, nil
	}
	b := req.Body
	c, err := h.consoles.Update(ctx, domain.ConsoleID(req.Id), application.ConsoleInput{Slug: b.Slug, DisplayName: b.DisplayName, Extensions: b.Extensions})
	if err == nil {
		return httpapi.UpdateConsole200JSONResponse(toAPI(c)), nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusBadRequest:
		return httpapi.UpdateConsole400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.UpdateConsole404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.UpdateConsole409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// DeleteConsole implements httpapi.StrictServerInterface.
func (h *Handler) DeleteConsole(ctx context.Context, req httpapi.DeleteConsoleRequestObject) (httpapi.DeleteConsoleResponseObject, error) {
	err := h.consoles.Delete(ctx, domain.ConsoleID(req.Id))
	if err == nil {
		return httpapi.DeleteConsole204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.DeleteConsole404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.DeleteConsole409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// ReorderConsoles implements httpapi.StrictServerInterface.
func (h *Handler) ReorderConsoles(ctx context.Context, req httpapi.ReorderConsolesRequestObject) (httpapi.ReorderConsolesResponseObject, error) {
	var ids []domain.ConsoleID
	if req.Body != nil {
		for _, id := range req.Body.Ids {
			ids = append(ids, domain.ConsoleID(id))
		}
	}
	consoles, err := h.consoles.Reorder(ctx, ids)
	if err == nil {
		out := make(httpapi.ReorderConsoles200JSONResponse, 0, len(consoles))
		for _, c := range consoles {
			out = append(out, toAPI(c))
		}
		return out, nil
	}
	if status, p, _ := problemFor(err); status == http.StatusBadRequest {
		return httpapi.ReorderConsoles400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

func badRequest(detail string) httpapi.Problem {
	return httpapi.Problem{Status: http.StatusBadRequest, Title: "Bad Request", Detail: &detail}
}
