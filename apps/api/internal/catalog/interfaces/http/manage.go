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

// WithManagement sets the use cases that change the library.
func (h *Handler) WithManagement(library *application.LibraryService, trashRetention time.Duration) *Handler {
	h.library, h.retention = library, trashRetention
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
		return mk(http.StatusNotFound, "El archivo no existe.")
	case errors.Is(err, domain.ErrTrashEntryNotFound):
		return mk(http.StatusNotFound, "La entrada de la papelera no existe.")
	case errors.Is(err, domain.ErrUnassignedNotFound):
		return mk(http.StatusNotFound, "El archivo no existe en No asignados.")
	case errors.Is(err, application.ErrConsoleNotFound):
		return mk(http.StatusNotFound, "La consola o la extensión no existe.")
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
	case errors.Is(err, application.ErrExtensionExists):
		return mk(http.StatusConflict, "La consola ya tiene esa extensión.")
	case errors.Is(err, application.ErrFixedExtension):
		return mk(http.StatusConflict, "Esa extensión viene de la variable de entorno o del código y no se puede quitar desde la app.")
	case errors.Is(err, application.ErrExtensionInUse):
		return mk(http.StatusConflict, "Hay archivos con esa extensión: muévelos o bórralos antes de quitarla.")
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
	err := h.library.TrashGame(context.WithoutCancel(ctx), domain.GameID(req.Id))
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
	err := h.library.TrashItem(context.WithoutCancel(ctx), domain.ItemID(req.Id))
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
	consoles, err := h.consoles.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(httpapi.ListTrash200JSONResponse, 0, len(entries))
	for _, e := range entries {
		v := httpapi.TrashEntry{
			Id: int64(e.ID), Kind: httpapi.TrashEntryKindUnassigned, WholeGame: e.WholeGame,
			Reason: httpapi.TrashEntryReason(e.Reason), TrashedAt: e.TrashedAt, ExpiresAt: e.ExpiresAt, Size: e.Size,
			Items: make([]httpapi.LibraryItem, 0, len(e.Items)), Files: make([]httpapi.UnassignedFile, 0, len(e.Files)),
		}
		if g := e.Game; g != nil {
			id, console, folder := int64(g.ID), string(g.Console), g.Folder
			v.Kind, v.GameId, v.Console, v.Title, v.Folder = httpapi.TrashEntryKindGame, &id, &console, g.Title, &folder
		}
		for _, it := range e.Items {
			v.Items = append(v.Items, ItemToAPI(it))
		}
		for _, f := range e.Files {
			v.Files = append(v.Files, unassignedToAPI(application.UnassignedView{
				UnassignedFile: f, Consoles: application.Accepting(consoles, f.Name()),
			}))
		}
		out = append(out, v)
	}
	return out, nil
}

// EmptyTrash implements httpapi.StrictServerInterface.
func (h *Handler) EmptyTrash(ctx context.Context, _ httpapi.EmptyTrashRequestObject) (httpapi.EmptyTrashResponseObject, error) {
	if _, err := h.library.EmptyTrash(context.WithoutCancel(ctx)); err != nil {
		return nil, err
	}
	return httpapi.EmptyTrash204Response{}, nil
}

// DeleteTrashEntry implements httpapi.StrictServerInterface.
func (h *Handler) DeleteTrashEntry(ctx context.Context, req httpapi.DeleteTrashEntryRequestObject) (httpapi.DeleteTrashEntryResponseObject, error) {
	err := h.library.DeleteTrashEntry(context.WithoutCancel(ctx), domain.TrashEntryID(req.Id))
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
	res, err := h.library.Restore(context.WithoutCancel(ctx), domain.TrashEntryID(req.Id), onConflict)
	if err == nil {
		out := httpapi.RestoreTrashEntry200JSONResponse{Path: res.Path}
		if res.GameID != nil {
			id := int64(*res.GameID)
			out.GameId = &id
		}
		return out, nil
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

// ---------- editing ----------

func editRequest(id int64, b *httpapi.GameEdit) application.EditRequest {
	req := application.EditRequest{GameID: domain.GameID(id), Decisions: map[domain.ItemID]domain.DuplicateAction{}}
	if b == nil {
		return req
	}
	req.Console, req.Name = b.Console, application.GameName{Title: b.Title, IGDBID: b.IgdbId}
	if b.Decisions != nil {
		for _, d := range *b.Decisions {
			req.Decisions[domain.ItemID(d.ItemId)] = domain.DuplicateAction(d.OnDuplicate)
		}
	}
	return req
}

// PlanGameEdit implements httpapi.StrictServerInterface.
func (h *Handler) PlanGameEdit(ctx context.Context, req httpapi.PlanGameEditRequestObject) (httpapi.PlanGameEditResponseObject, error) {
	p, err := h.library.PlanEdit(ctx, editRequest(req.Id, req.Body))
	if err == nil {
		out := httpapi.PlanGameEdit200JSONResponse{Console: p.Console, Title: p.Title, Folder: p.Folder, Items: make([]httpapi.GameEditItem, 0, len(p.Items))}
		if p.MergeInto != 0 {
			id := int64(p.MergeInto)
			out.MergeInto = &id
		}
		for _, it := range p.Items {
			ei := httpapi.GameEditItem{Item: ItemToAPI(it.Item), File: it.File, Action: httpapi.PlanAction(it.Action)}
			if it.Duplicate != nil {
				d := ItemToAPI(*it.Duplicate)
				ei.Duplicate = &d
			}
			out.Items = append(out.Items, ei)
		}
		return out, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusBadRequest:
		return httpapi.PlanGameEdit400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.PlanGameEdit404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusBadGateway:
		return httpapi.PlanGameEdit502ApplicationProblemPlusJSONResponse{BadGatewayApplicationProblemPlusJSONResponse: httpapi.BadGatewayApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusServiceUnavailable:
		return httpapi.PlanGameEdit503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse: httpapi.ServiceUnavailableApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// EditGame implements httpapi.StrictServerInterface.
func (h *Handler) EditGame(ctx context.Context, req httpapi.EditGameRequestObject) (httpapi.EditGameResponseObject, error) {
	// The renames must finish even if the browser goes away.
	res, err := h.library.Edit(context.WithoutCancel(ctx), editRequest(req.Id, req.Body))
	if err == nil {
		return httpapi.EditGame200JSONResponse{GameId: int64(res.GameID), Path: res.Path, Merged: res.Merged}, nil
	}
	switch status, p := failed(err); status {
	case http.StatusBadRequest:
		return httpapi.EditGame400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.EditGame404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.EditGame409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusBadGateway:
		return httpapi.EditGame502ApplicationProblemPlusJSONResponse{BadGatewayApplicationProblemPlusJSONResponse: httpapi.BadGatewayApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusServiceUnavailable:
		return httpapi.EditGame503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse: httpapi.ServiceUnavailableApplicationProblemPlusJSONResponse(p)}, nil
	default:
		return httpapi.EditGame500ApplicationProblemPlusJSONResponse{InternalErrorApplicationProblemPlusJSONResponse: httpapi.InternalErrorApplicationProblemPlusJSONResponse(p)}, nil
	}
}

// EditItem implements httpapi.StrictServerInterface.
func (h *Handler) EditItem(ctx context.Context, req httpapi.EditItemRequestObject) (httpapi.EditItemResponseObject, error) {
	if req.Body == nil {
		return httpapi.EditItem400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(badRequest("Falta el cuerpo de la petición."))}, nil
	}
	in := application.ItemEditRequest{ItemID: domain.ItemID(req.Id), Kind: domain.ItemKind(req.Body.Kind)}
	if req.Body.Label != nil {
		in.Label = *req.Body.Label
	}
	if req.Body.OnDuplicate != nil {
		in.OnDuplicate = domain.DuplicateAction(*req.Body.OnDuplicate)
	}
	game, err := h.library.EditFile(context.WithoutCancel(ctx), in)
	if err == nil {
		out, err := h.gameDetail(ctx, game)
		if err != nil {
			return nil, err
		}
		return httpapi.EditItem200JSONResponse(out), nil
	}
	switch status, p := failed(err); status {
	case http.StatusBadRequest:
		return httpapi.EditItem400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.EditItem404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.EditItem409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	default:
		return httpapi.EditItem500ApplicationProblemPlusJSONResponse{InternalErrorApplicationProblemPlusJSONResponse: httpapi.InternalErrorApplicationProblemPlusJSONResponse(p)}, nil
	}
}

// ---------- unassigned section ----------

// UnassignGame implements httpapi.StrictServerInterface.
func (h *Handler) UnassignGame(ctx context.Context, req httpapi.UnassignGameRequestObject) (httpapi.UnassignGameResponseObject, error) {
	err := h.library.UnassignGame(context.WithoutCancel(ctx), domain.GameID(req.Id))
	if err == nil {
		return httpapi.UnassignGame204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.UnassignGame404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.UnassignGame409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// UnassignItem implements httpapi.StrictServerInterface.
func (h *Handler) UnassignItem(ctx context.Context, req httpapi.UnassignItemRequestObject) (httpapi.UnassignItemResponseObject, error) {
	err := h.library.UnassignItem(context.WithoutCancel(ctx), domain.ItemID(req.Id))
	if err == nil {
		return httpapi.UnassignItem204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.UnassignItem404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.UnassignItem409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// ListUnassigned implements httpapi.StrictServerInterface.
func (h *Handler) ListUnassigned(ctx context.Context, _ httpapi.ListUnassignedRequestObject) (httpapi.ListUnassignedResponseObject, error) {
	entries, err := h.library.Unassigned(ctx)
	if err != nil {
		return nil, err
	}
	out := make(httpapi.ListUnassigned200JSONResponse, 0, len(entries))
	for _, e := range entries {
		v := httpapi.UnassignedEntry{
			Id: int64(e.ID), Name: e.Name, Folder: e.Folder, Size: e.Size, IgdbId: e.IGDBID,
			ArrivedAt: e.ArrivedAt, Copying: e.Copying, Busy: e.Busy,
			Files: make([]httpapi.UnassignedFile, 0, len(e.Files)),
		}
		if e.Console != "" {
			c := string(e.Console)
			v.Console = &c
		}
		for _, f := range e.Files {
			v.Files = append(v.Files, unassignedToAPI(f))
		}
		out = append(out, v)
	}
	return out, nil
}

func unassignedToAPI(f application.UnassignedView) httpapi.UnassignedFile {
	reason, origin := f.Reason, f.Origin
	if f.Copying {
		reason, origin = domain.UnassignedSamba, "_unassigned/"+f.Path
	}
	out := httpapi.UnassignedFile{
		Id: int64(f.ID), Path: f.Path, Name: f.Name(), Origin: origin, Reason: httpapi.UnassignedFileReason(reason),
		Size: f.Size, ArrivedAt: f.ArrivedAt, Consoles: f.Consoles, Archive: f.Archive, IgdbId: f.IGDBID,
	}
	if f.Copying {
		out.Copying = &f.Copying
	}
	if f.Console != "" {
		c := string(f.Console)
		out.Console = &c
	}
	return out
}

// DeleteUnassigned implements httpapi.StrictServerInterface.
func (h *Handler) DeleteUnassigned(ctx context.Context, req httpapi.DeleteUnassignedRequestObject) (httpapi.DeleteUnassignedResponseObject, error) {
	err := h.library.DeleteUnassigned(context.WithoutCancel(ctx), domain.UnassignedID(req.Id))
	if err == nil {
		return httpapi.DeleteUnassigned204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.DeleteUnassigned404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.DeleteUnassigned409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// DeleteUnassignedEntry implements httpapi.StrictServerInterface.
func (h *Handler) DeleteUnassignedEntry(ctx context.Context, req httpapi.DeleteUnassignedEntryRequestObject) (httpapi.DeleteUnassignedEntryResponseObject, error) {
	err := h.library.DeleteUnassignedEntry(context.WithoutCancel(ctx), domain.UnassignedID(req.Id))
	if err == nil {
		return httpapi.DeleteUnassignedEntry204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.DeleteUnassignedEntry404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.DeleteUnassignedEntry409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// TrashUnassignedEntry implements httpapi.StrictServerInterface.
func (h *Handler) TrashUnassignedEntry(ctx context.Context, req httpapi.TrashUnassignedEntryRequestObject) (httpapi.TrashUnassignedEntryResponseObject, error) {
	err := h.library.TrashUnassignedEntry(context.WithoutCancel(ctx), domain.UnassignedID(req.Id))
	if err == nil {
		return httpapi.TrashUnassignedEntry204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.TrashUnassignedEntry404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.TrashUnassignedEntry409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// TrashUnassigned implements httpapi.StrictServerInterface.
func (h *Handler) TrashUnassigned(ctx context.Context, req httpapi.TrashUnassignedRequestObject) (httpapi.TrashUnassignedResponseObject, error) {
	err := h.library.TrashUnassigned(context.WithoutCancel(ctx), domain.UnassignedID(req.Id))
	if err == nil {
		return httpapi.TrashUnassigned204Response{}, nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusNotFound:
		return httpapi.TrashUnassigned404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.TrashUnassigned409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// ---------- scan ----------

// ScanLibrary implements httpapi.StrictServerInterface.
func (h *Handler) ScanLibrary(ctx context.Context, _ httpapi.ScanLibraryRequestObject) (httpapi.ScanLibraryResponseObject, error) {
	rep, err := h.library.Scan(context.WithoutCancel(ctx))
	if err != nil {
		return nil, err
	}
	return httpapi.ScanLibrary200JSONResponse(scanToAPI(rep)), nil
}

// GetLibraryScan implements httpapi.StrictServerInterface.
func (h *Handler) GetLibraryScan(_ context.Context, _ httpapi.GetLibraryScanRequestObject) (httpapi.GetLibraryScanResponseObject, error) {
	out := httpapi.ScanStatus{}
	if rep := h.library.LastScan(); rep != nil {
		last := scanToAPI(*rep)
		out.LastScan = &last
	}
	return httpapi.GetLibraryScan200JSONResponse(out), nil
}

func scanToAPI(rep application.ScanReport) httpapi.ScanReport {
	return httpapi.ScanReport{ScannedAt: rep.ScannedAt, Unassigned: rep.Unassigned, Removed: rep.Removed, Pending: rep.Pending}
}
