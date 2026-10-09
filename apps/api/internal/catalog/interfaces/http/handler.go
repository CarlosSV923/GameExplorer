// Package http adapts the catalog use cases to the generated HTTP contract.
package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpapi"
)

// Handler implements the catalog operations of httpapi.StrictServerInterface.
type Handler struct {
	consoles *application.ConsoleService
	browse   *application.BrowseService
	log      *slog.Logger

	library   *application.LibraryService
	retention time.Duration
}

// NewHandler builds the handler.
func NewHandler(consoles *application.ConsoleService) *Handler {
	return &Handler{consoles: consoles}
}

// ListConsoles implements httpapi.StrictServerInterface.
func (h *Handler) ListConsoles(ctx context.Context, _ httpapi.ListConsolesRequestObject) (httpapi.ListConsolesResponseObject, error) {
	out, err := h.consoleList(ctx)
	if err != nil {
		return nil, err
	}
	return httpapi.ListConsoles200JSONResponse(out), nil
}

func (h *Handler) consoleList(ctx context.Context) ([]httpapi.Console, error) {
	consoles, err := h.consoles.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]httpapi.Console, 0, len(consoles))
	for _, c := range consoles {
		v, err := h.toAPI(ctx, c)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, nil
}

func (h *Handler) toAPI(ctx context.Context, c domain.Console) (httpapi.Console, error) {
	uses, err := h.consoles.ExtensionUse(ctx, c)
	if err != nil {
		return httpapi.Console{}, err
	}
	out := httpapi.Console{
		Slug:             string(c.Slug),
		DisplayName:      c.DisplayName,
		DefaultName:      c.Name,
		IgdbPlatformId:   c.IGDBPlatformID,
		LogoImageId:      c.LogoImageID,
		Extensions:       c.Extensions,
		CustomExtensions: make([]httpapi.CustomExtension, 0, len(c.Custom)),
		Kinds:            make([]httpapi.ItemKind, 0, len(c.Kinds)),
		MultipleFiles:    c.MultipleFiles,
		SortOrder:        c.SortOrder,
		GameCount:        c.GameCount,
	}
	if c.Year > 0 {
		y := c.Year
		out.ReleaseYear = &y
	}
	for _, e := range c.Custom {
		out.CustomExtensions = append(out.CustomExtensions, httpapi.CustomExtension{Extension: e, FileCount: uses[e]})
	}
	for _, k := range c.Kinds {
		out.Kinds = append(out.Kinds, httpapi.ItemKind(k))
	}
	return out, nil
}

// ReorderConsoles implements httpapi.StrictServerInterface.
func (h *Handler) ReorderConsoles(ctx context.Context, req httpapi.ReorderConsolesRequestObject) (httpapi.ReorderConsolesResponseObject, error) {
	var slugs []string
	if req.Body != nil {
		slugs = req.Body.Slugs
	}
	if _, err := h.consoles.Reorder(ctx, slugs); err != nil {
		if status, p, _ := problemFor(err); status == http.StatusBadRequest {
			return httpapi.ReorderConsoles400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
		}
		return nil, err
	}
	out, err := h.consoleList(ctx)
	if err != nil {
		return nil, err
	}
	return httpapi.ReorderConsoles200JSONResponse(out), nil
}

// UpdateConsole implements httpapi.StrictServerInterface.
func (h *Handler) UpdateConsole(ctx context.Context, req httpapi.UpdateConsoleRequestObject) (httpapi.UpdateConsoleResponseObject, error) {
	if req.Body == nil {
		return httpapi.UpdateConsole400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(badRequest("Falta el cuerpo de la petición."))}, nil
	}
	c, err := h.consoles.Rename(ctx, req.Slug, req.Body.DisplayName)
	if err == nil {
		v, err := h.toAPI(ctx, c)
		if err != nil {
			return nil, err
		}
		return httpapi.UpdateConsole200JSONResponse(v), nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusBadRequest:
		return httpapi.UpdateConsole400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.UpdateConsole404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// AddConsoleExtension implements httpapi.StrictServerInterface.
func (h *Handler) AddConsoleExtension(ctx context.Context, req httpapi.AddConsoleExtensionRequestObject) (httpapi.AddConsoleExtensionResponseObject, error) {
	if req.Body == nil {
		return httpapi.AddConsoleExtension400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(badRequest("Falta el cuerpo de la petición."))}, nil
	}
	c, err := h.consoles.AddExtension(ctx, req.Slug, req.Body.Extension)
	if err == nil {
		v, err := h.toAPI(ctx, c)
		if err != nil {
			return nil, err
		}
		return httpapi.AddConsoleExtension200JSONResponse(v), nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusBadRequest:
		return httpapi.AddConsoleExtension400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.AddConsoleExtension404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.AddConsoleExtension409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

// RemoveConsoleExtension implements httpapi.StrictServerInterface.
func (h *Handler) RemoveConsoleExtension(ctx context.Context, req httpapi.RemoveConsoleExtensionRequestObject) (httpapi.RemoveConsoleExtensionResponseObject, error) {
	c, err := h.consoles.RemoveExtension(ctx, req.Slug, req.Params.Extension)
	if err == nil {
		v, err := h.toAPI(ctx, c)
		if err != nil {
			return nil, err
		}
		return httpapi.RemoveConsoleExtension200JSONResponse(v), nil
	}
	switch status, p, _ := problemFor(err); status {
	case http.StatusBadRequest:
		return httpapi.RemoveConsoleExtension400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusNotFound:
		return httpapi.RemoveConsoleExtension404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	case http.StatusConflict:
		return httpapi.RemoveConsoleExtension409ApplicationProblemPlusJSONResponse{ConflictApplicationProblemPlusJSONResponse: httpapi.ConflictApplicationProblemPlusJSONResponse(p)}, nil
	}
	return nil, err
}

func badRequest(detail string) httpapi.Problem {
	return httpapi.Problem{Status: http.StatusBadRequest, Title: "Bad Request", Detail: &detail}
}
