// Package http adapts the metadata use cases to the generated HTTP contract.
package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpapi"
)

// imageCacheControl: an IGDB image id always points to the same picture.
const imageCacheControl = "private, max-age=2592000, immutable"

// Handler implements the metadata operations of httpapi.StrictServerInterface.
type Handler struct {
	svc *application.Service
	log *slog.Logger
}

// NewHandler builds the handler.
func NewHandler(svc *application.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

// SearchGames implements httpapi.StrictServerInterface.
func (h *Handler) SearchGames(ctx context.Context, req httpapi.SearchGamesRequestObject) (httpapi.SearchGamesResponseObject, error) {
	limit := 0
	if req.Params.Limit != nil {
		limit = *req.Params.Limit
	}
	games, err := h.svc.SearchGames(ctx, req.Params.Q, req.Params.PlatformId, limit)
	if err != nil {
		status, p := h.problem(ctx, err)
		switch status {
		case http.StatusBadRequest:
			return httpapi.SearchGames400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusServiceUnavailable:
			return httpapi.SearchGames503ApplicationProblemPlusJSONResponse{ServiceUnavailableApplicationProblemPlusJSONResponse: httpapi.ServiceUnavailableApplicationProblemPlusJSONResponse(p)}, nil
		default:
			return httpapi.SearchGames502ApplicationProblemPlusJSONResponse{BadGatewayApplicationProblemPlusJSONResponse: httpapi.BadGatewayApplicationProblemPlusJSONResponse(p)}, nil
		}
	}
	out := make(httpapi.SearchGames200JSONResponse, 0, len(games))
	for _, g := range games {
		out = append(out, httpapi.MetadataGame{
			Id:           g.ID,
			Name:         g.Name,
			ReleaseYear:  g.ReleaseYear,
			CoverImageId: g.CoverImageID,
			Summary:      g.Summary,
			Genres:       g.Genres,
			PlatformIds:  g.PlatformIDs,
		})
	}
	return out, nil
}

// GetMetadataStatus implements httpapi.StrictServerInterface.
func (h *Handler) GetMetadataStatus(_ context.Context, _ httpapi.GetMetadataStatusRequestObject) (httpapi.GetMetadataStatusResponseObject, error) {
	return httpapi.GetMetadataStatus200JSONResponse{Configured: h.svc.Configured()}, nil
}

// GetImage implements httpapi.StrictServerInterface.
func (h *Handler) GetImage(ctx context.Context, req httpapi.GetImageRequestObject) (httpapi.GetImageResponseObject, error) {
	body, size, ref, err := h.svc.Image(ctx, string(req.Size), req.ImageId)
	if err != nil {
		status, p := h.problem(ctx, err)
		switch status {
		case http.StatusBadRequest:
			return httpapi.GetImage400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse(p)}, nil
		case http.StatusNotFound:
			return httpapi.GetImage404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
		default:
			return httpapi.GetImage502ApplicationProblemPlusJSONResponse{BadGatewayApplicationProblemPlusJSONResponse: httpapi.BadGatewayApplicationProblemPlusJSONResponse(p)}, nil
		}
	}
	cc := imageCacheControl
	headers := httpapi.GetImage200ResponseHeaders{CacheControl: &cc}
	if ref.ContentType() == "image/png" {
		return httpapi.GetImage200ImagepngResponse{Body: body, ContentLength: size, Headers: headers}, nil
	}
	return httpapi.GetImage200ImagejpegResponse{Body: body, ContentLength: size, Headers: headers}, nil
}

// problem maps domain errors to an HTTP status and a problem body.
func (h *Handler) problem(ctx context.Context, err error) (int, httpapi.Problem) {
	mk := func(status int, title, detail string) (int, httpapi.Problem) {
		return status, httpapi.Problem{Status: status, Title: title, Detail: &detail}
	}
	switch {
	case errors.Is(err, domain.ErrInvalidQuery):
		return mk(http.StatusBadRequest, "Bad Request", "Escribe entre 2 y 100 caracteres.")
	case errors.Is(err, domain.ErrInvalidImage):
		return mk(http.StatusBadRequest, "Bad Request", "Referencia de imagen inválida.")
	case errors.Is(err, domain.ErrNotFound):
		return mk(http.StatusNotFound, "Not Found", "La imagen no existe en IGDB.")
	case errors.Is(err, domain.ErrNotConfigured):
		return mk(http.StatusServiceUnavailable, "Service Unavailable",
			"IGDB no está configurado: define IGDB_CLIENT_ID e IGDB_CLIENT_SECRET.")
	default:
		h.log.ErrorContext(ctx, "metadata provider error", "error", err)
		return mk(http.StatusBadGateway, "Bad Gateway", "IGDB no respondió correctamente. Inténtalo de nuevo.")
	}
}
