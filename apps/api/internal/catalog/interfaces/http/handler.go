// Package http adapts the catalog use cases to the generated HTTP contract.
package http

import (
	"context"
	"log/slog"
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
	platforms application.PlatformDirectory
	retention time.Duration
}

// NewHandler builds the handler.
func NewHandler(consoles *application.ConsoleService) *Handler {
	return &Handler{consoles: consoles}
}

// ListConsoles implements httpapi.StrictServerInterface.
func (h *Handler) ListConsoles(ctx context.Context, _ httpapi.ListConsolesRequestObject) (httpapi.ListConsolesResponseObject, error) {
	consoles, err := h.consoles.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(httpapi.ListConsoles200JSONResponse, 0, len(consoles))
	for _, c := range consoles {
		out = append(out, toAPI(c))
	}
	return out, nil
}

func toAPI(c domain.Console) httpapi.Console {
	return httpapi.Console{
		Id:             int64(c.ID),
		Slug:           string(c.Slug),
		DisplayName:    c.DisplayName,
		IgdbPlatformId: c.IGDBPlatformID,
		ReleaseYear:    c.ReleaseYear,
		LogoImageId:    c.LogoImageID,
		Extensions:     c.Extensions,
		SortOrder:      c.SortOrder,
		GameCount:      c.GameCount,
		BuiltIn:        c.DetectorKey != "",
		Detection:      detection(c.DetectorKey),
	}
}

// detection tells how uploads for a console are recognized (by its detector).
func detection(key string) httpapi.ConsoleDetection {
	switch key {
	case "":
		return httpapi.ConsoleDetectionExtension
	case "switch":
		return httpapi.ConsoleDetectionTitleId
	case "ps3":
		return httpapi.ConsoleDetectionStructure
	default:
		return httpapi.ConsoleDetectionHeader
	}
}
