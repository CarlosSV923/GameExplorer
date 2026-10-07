package application

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// Browse errors.
var (
	ErrConsoleNotFound = errors.New("console not found")
	// ErrInvalidSearch is returned for empty or too long search terms.
	ErrInvalidSearch = errors.New("invalid search")
	// ErrFileMissing means a recorded file is no longer on disk (deleted over SMB).
	ErrFileMissing = errors.New("file missing from the library")
)

// GameListing is a game in a list, with its console slug.
type GameListing struct {
	domain.GameSummary
	Console domain.Slug
}

// GameView is a game's detail: items in the library, base first.
type GameView struct {
	Game    *domain.Game
	Console domain.Slug
	Items   []domain.GameItem
	Size    int64
	Missing int
}

// Download describes what to send for a download request: a single file
// (resumable with Range) or a zip built while it is sent.
type Download struct {
	// Name is the suggested file name.
	Name string
	// File is set for single-file items.
	File *TreeFile
	// Entries are set for zips; Name is "<game folder>/<file>".
	Entries []ZipFile
}

// ZipFile is one file of a zip download.
type ZipFile struct {
	Name string
	TreeFile
}

// BrowseService implements the library's read side (RF-20..RF-23).
type BrowseService struct {
	consoles domain.ConsoleRepository
	repo     domain.LibraryRepository
	files    Files
}

// NewBrowseService builds the service.
func NewBrowseService(consoles domain.ConsoleRepository, repo domain.LibraryRepository, files Files) *BrowseService {
	return &BrowseService{consoles: consoles, repo: repo, files: files}
}

// ConsoleGames lists the games of one console, by title.
func (s *BrowseService) ConsoleGames(ctx context.Context, slug string) ([]GameListing, error) {
	slugs, err := s.slugs(ctx)
	if err != nil {
		return nil, err
	}
	var id domain.ConsoleID
	found := false
	for cid, sl := range slugs {
		if string(sl) == slug {
			id, found = cid, true
		}
	}
	if !found {
		return nil, ErrConsoleNotFound
	}
	return s.list(ctx, slugs, func(g domain.GameSummary) bool { return g.ConsoleID == id })
}

// Search finds games whose title contains every word of q, ignoring case and
// accents ("pokemon" finds "Pokémon").
func (s *BrowseService) Search(ctx context.Context, q string) ([]GameListing, error) {
	words := strings.Fields(fold(q))
	if len(words) == 0 || len([]rune(q)) > 100 {
		return nil, ErrInvalidSearch
	}
	slugs, err := s.slugs(ctx)
	if err != nil {
		return nil, err
	}
	return s.list(ctx, slugs, func(g domain.GameSummary) bool {
		title := fold(g.Title)
		for _, w := range words {
			if !strings.Contains(title, w) {
				return false
			}
		}
		return true
	})
}

func (s *BrowseService) list(ctx context.Context, slugs map[domain.ConsoleID]domain.Slug, keep func(domain.GameSummary) bool) ([]GameListing, error) {
	games, err := s.repo.ListGames(ctx)
	if err != nil {
		return nil, err
	}
	out := []GameListing{}
	for _, g := range games {
		if keep(g) {
			out = append(out, GameListing{GameSummary: g, Console: slugs[g.ConsoleID]})
		}
	}
	return out, nil
}

func (s *BrowseService) slugs(ctx context.Context) (map[domain.ConsoleID]domain.Slug, error) {
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.ConsoleID]domain.Slug, len(consoles))
	for _, c := range consoles {
		out[c.ID] = c.Slug
	}
	return out, nil
}

// fold lower-cases and strips accents and punctuation for matching.
func fold(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(strings.ToLower(s)) {
		switch {
		case unicode.Is(unicode.Mn, r): // combining accent
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteRune(' ')
		}
	}
	return b.String()
}

// Game returns a game with its items in the library. A game whose items are
// all in the trash is not found.
func (s *BrowseService) Game(ctx context.Context, id domain.GameID) (*GameView, error) {
	g, err := s.repo.GameByID(ctx, id)
	if err != nil {
		return nil, err
	}
	items := g.LiveItems()
	if len(items) == 0 {
		return nil, domain.ErrGameNotFound
	}
	slugs, err := s.slugs(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(items, compareItems)
	v := &GameView{Game: g, Console: slugs[g.ConsoleID], Items: items}
	for _, it := range items {
		v.Size += it.Size
		if it.MissingSince != nil {
			v.Missing++
		}
	}
	return v, nil
}

// compareItems orders a game's items: base, discs by number, updates by
// date, then DLC by name.
func compareItems(a, b domain.GameItem) int {
	rank := map[domain.ItemKind]int{domain.KindBase: 0, domain.KindDisc: 1, domain.KindUpdate: 2, domain.KindDLC: 3}
	return cmp.Or(
		cmp.Compare(rank[a.Kind], rank[b.Kind]),
		cmp.Compare(a.DiscNumber, b.DiscNumber),
		a.CreatedAt.Compare(b.CreatedAt),
		strings.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label)),
		cmp.Compare(a.ID, b.ID),
	)
}

// ItemDownload prepares the download of one item: single files as they are,
// discs and folder games as a zip.
func (s *BrowseService) ItemDownload(ctx context.Context, id domain.ItemID) (Download, error) {
	it, err := s.repo.ItemByID(ctx, id)
	if err != nil {
		return Download{}, err
	}
	if !it.Live() {
		return Download{}, domain.ErrItemNotFound
	}
	v, err := s.Game(ctx, it.GameID)
	if err != nil {
		return Download{}, err
	}
	entries, err := s.zipEntries(v, []domain.GameItem{it})
	if err != nil {
		return Download{}, err
	}
	if it.Shape == domain.ShapeFile && len(entries) == 1 {
		return Download{Name: it.Files[0], File: &entries[0].TreeFile}, nil
	}
	return Download{Name: strings.TrimSuffix(it.Files[0], path.Ext(it.Files[0])) + ".zip", Entries: entries}, nil
}

// GameDownload prepares the whole game as a zip.
func (s *BrowseService) GameDownload(ctx context.Context, id domain.GameID) (Download, error) {
	v, err := s.Game(ctx, id)
	if err != nil {
		return Download{}, err
	}
	entries, err := s.zipEntries(v, v.Items)
	if err != nil {
		return Download{}, err
	}
	return Download{Name: v.Game.Folder + ".zip", Entries: entries}, nil
}

func (s *BrowseService) zipEntries(v *GameView, items []domain.GameItem) ([]ZipFile, error) {
	var out []ZipFile
	for _, it := range items {
		for _, f := range it.Files {
			files, err := s.files.Tree(s.files.LibraryPath(string(v.Console), v.Game.Folder, f))
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("%w: %s/%s/%s", ErrFileMissing, v.Console, v.Game.Folder, f)
			}
			if err != nil {
				return nil, err
			}
			for _, tf := range files {
				name := v.Game.Folder + "/" + f
				if tf.Rel != "" {
					name += "/" + tf.Rel
				}
				out = append(out, ZipFile{Name: name, TreeFile: tf})
			}
		}
	}
	return out, nil
}

// Open opens a file of a Download.
func (s *BrowseService) Open(f TreeFile) (io.ReadSeekCloser, error) {
	return s.files.Open(f.Path)
}
