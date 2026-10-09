package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpapi"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/httpx"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/zipstream"
)

// WithLibrary sets the library's read side.
func (h *Handler) WithLibrary(browse *application.BrowseService, log *slog.Logger) *Handler {
	h.browse, h.log = browse, log
	return h
}

// ListConsoleGames implements httpapi.StrictServerInterface.
func (h *Handler) ListConsoleGames(ctx context.Context, req httpapi.ListConsoleGamesRequestObject) (httpapi.ListConsoleGamesResponseObject, error) {
	games, err := h.browse.ConsoleGames(ctx, req.Slug)
	if errors.Is(err, application.ErrConsoleNotFound) {
		return httpapi.ListConsoleGames404ApplicationProblemPlusJSONResponse{
			NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(notFound("La consola no existe.")),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return httpapi.ListConsoleGames200JSONResponse(summaries(games)), nil
}

// SearchLibrary implements httpapi.StrictServerInterface.
func (h *Handler) SearchLibrary(ctx context.Context, req httpapi.SearchLibraryRequestObject) (httpapi.SearchLibraryResponseObject, error) {
	games, err := h.browse.Search(ctx, req.Params.Q)
	if errors.Is(err, application.ErrInvalidSearch) {
		detail := "Escribe entre 1 y 100 caracteres."
		return httpapi.SearchLibrary400ApplicationProblemPlusJSONResponse{
			BadRequestApplicationProblemPlusJSONResponse: httpapi.BadRequestApplicationProblemPlusJSONResponse{
				Status: http.StatusBadRequest, Title: "Bad Request", Detail: &detail,
			},
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return httpapi.SearchLibrary200JSONResponse(summaries(games)), nil
}

// GetGame implements httpapi.StrictServerInterface.
func (h *Handler) GetGame(ctx context.Context, req httpapi.GetGameRequestObject) (httpapi.GetGameResponseObject, error) {
	out, err := h.gameDetail(ctx, domain.GameID(req.Id))
	if errors.Is(err, domain.ErrGameNotFound) {
		return httpapi.GetGame404ApplicationProblemPlusJSONResponse{
			NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(notFound("El juego no existe.")),
		}, nil
	}
	if err != nil {
		return nil, err
	}
	return httpapi.GetGame200JSONResponse(out), nil
}

func (h *Handler) gameDetail(ctx context.Context, id domain.GameID) (httpapi.GameDetail, error) {
	v, err := h.browse.Game(ctx, id)
	if err != nil {
		return httpapi.GameDetail{}, err
	}
	g := v.Game
	out := httpapi.GameDetail{
		Id: int64(g.ID), Console: string(g.Console), Title: g.Title, Folder: g.Folder, ReleaseYear: g.ReleaseYear,
		CoverImageId: g.CoverImageID, ItemCount: len(v.Items), Size: v.Size, IgdbId: g.IGDBID,
		Path: string(g.Console) + "/" + g.Folder, Summary: g.Summary, Genres: g.Genres,
		Items: make([]httpapi.LibraryItem, 0, len(v.Items)),
	}
	if out.Genres == nil {
		out.Genres = []string{}
	}
	for _, it := range v.Items {
		out.Items = append(out.Items, ItemToAPI(it))
	}
	return out, nil
}

// ItemToAPI maps a stored file.
func ItemToAPI(it domain.GameItem) httpapi.LibraryItem {
	out := httpapi.LibraryItem{
		Id: int64(it.ID), Kind: httpapi.ItemKind(it.Kind), File: it.File, Size: it.Size, CreatedAt: it.CreatedAt,
	}
	if it.Label != "" {
		l := it.Label
		out.Label = &l
	}
	return out
}

// DownloadItem implements httpapi.StrictServerInterface.
func (h *Handler) DownloadItem(ctx context.Context, req httpapi.DownloadItemRequestObject) (httpapi.DownloadItemResponseObject, error) {
	d, err := h.browse.ItemDownload(ctx, domain.ItemID(req.Id))
	if p, ok := downloadProblem(err, "El archivo no existe."); ok {
		return httpapi.DownloadItem404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	}
	if err != nil {
		return nil, err
	}
	return h.download(ctx, d), nil
}

// DownloadGame implements httpapi.StrictServerInterface.
func (h *Handler) DownloadGame(ctx context.Context, req httpapi.DownloadGameRequestObject) (httpapi.DownloadGameResponseObject, error) {
	d, err := h.browse.GameDownload(ctx, domain.GameID(req.Id))
	if p, ok := downloadProblem(err, "El juego no existe."); ok {
		return httpapi.DownloadGame404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(p)}, nil
	}
	if err != nil {
		return nil, err
	}
	return h.download(ctx, d), nil
}

// DownloadUnassigned implements httpapi.StrictServerInterface.
func (h *Handler) DownloadUnassigned(ctx context.Context, req httpapi.DownloadUnassignedRequestObject) (httpapi.DownloadUnassignedResponseObject, error) {
	p, f, err := h.library.UnassignedFile(ctx, domain.UnassignedID(req.Id))
	var d application.Download
	if err == nil {
		d, err = h.browse.FileDownload(p, f.Name())
	}
	if prob, ok := downloadProblem(err, "El archivo no existe."); ok {
		return httpapi.DownloadUnassigned404ApplicationProblemPlusJSONResponse{NotFoundApplicationProblemPlusJSONResponse: httpapi.NotFoundApplicationProblemPlusJSONResponse(prob)}, nil
	}
	if err != nil {
		return nil, err
	}
	return h.download(ctx, d), nil
}

func downloadProblem(err error, missing string) (httpapi.Problem, bool) {
	switch {
	case errors.Is(err, domain.ErrGameNotFound), errors.Is(err, domain.ErrItemNotFound), errors.Is(err, domain.ErrUnassignedNotFound):
		return notFound(missing), true
	case errors.Is(err, application.ErrFileMissing):
		return notFound("Falta el archivo en el disco (¿se borró por SMB?): " + err.Error()), true
	}
	return httpapi.Problem{}, false
}

func notFound(detail string) httpapi.Problem {
	return httpapi.Problem{Status: http.StatusNotFound, Title: "Not Found", Detail: &detail}
}

func summaries(games []domain.GameSummary) []httpapi.GameSummary {
	out := make([]httpapi.GameSummary, 0, len(games))
	for _, g := range games {
		out = append(out, httpapi.GameSummary{
			Id: int64(g.ID), IgdbId: g.IGDBID, Console: string(g.Console), Title: g.Title, Folder: g.Folder,
			ReleaseYear: g.ReleaseYear, CoverImageId: g.CoverImageID, ItemCount: g.ItemCount, Size: g.Size,
		})
	}
	return out
}

// contentDisposition names a download. Non-ASCII names (Ōkami.zip) get an
// ASCII fallback (Okami.zip) for clients that ignore filename*.
func contentDisposition(name string) string {
	ascii := asciiName(name)
	if ascii == name {
		return mime.FormatMediaType("attachment", map[string]string{"filename": name})
	}
	return mime.FormatMediaType("attachment", map[string]string{"filename": ascii}) + "; filename*=UTF-8''" + url.PathEscape(name)
}

func asciiName(name string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(name) {
		switch {
		case unicode.Is(unicode.Mn, r): // combining accent
		case r < 0x20 || r == '"' || r == '\\' || r == 0x7f:
		case r < 0x80:
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

// downloadResponse sends a Download. It replaces the generated response
// types: single files go through http.ServeContent (Range, If-Range,
// sendfile), which needs the request, and zips are streamed with their exact
// size announced.
type downloadResponse struct {
	d      application.Download
	r      *http.Request
	browse *application.BrowseService
	log    *slog.Logger
}

func (h *Handler) download(ctx context.Context, d application.Download) downloadResponse {
	r, _ := httpx.Request(ctx)
	return downloadResponse{d: d, r: r, browse: h.browse, log: h.log}
}

func (d downloadResponse) VisitDownloadItemResponse(w http.ResponseWriter) error { return d.visit(w) }

func (d downloadResponse) VisitDownloadGameResponse(w http.ResponseWriter) error { return d.visit(w) }

func (d downloadResponse) VisitDownloadUnassignedResponse(w http.ResponseWriter) error {
	return d.visit(w)
}

func (d downloadResponse) visit(w http.ResponseWriter) error {
	w.Header().Set("Content-Disposition", contentDisposition(d.d.Name))
	w.Header().Set("Cache-Control", "no-store")
	if d.d.File != nil {
		return d.serveFile(w)
	}

	entries := make([]zipstream.Entry, 0, len(d.d.Entries))
	for _, e := range d.d.Entries {
		f := e.TreeFile
		entries = append(entries, zipstream.Entry{Name: e.Name, Size: f.Size, Modified: f.Modified, Open: func() (io.ReadCloser, error) {
			return d.browse.Open(f)
		}})
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Length", strconv.FormatInt(zipstream.Size(entries), 10))
	w.WriteHeader(http.StatusOK)
	if err := zipstream.Write(w, entries); err != nil {
		// Headers are gone: the client sees a truncated download.
		d.log.Warn("zip download interrupted", "name", d.d.Name, "error", err)
	}
	return nil
}

func (d downloadResponse) serveFile(w http.ResponseWriter) error {
	if d.r == nil {
		return errors.New("download: request missing from context (httpx.WithRequest)")
	}
	f, err := d.browse.Open(*d.d.File)
	if err != nil {
		return fmt.Errorf("open %s: %w", d.d.Name, err)
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, d.r, "", d.d.File.Modified, f)
	return nil
}
