package application

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// Browse errors.
var (
	// ErrInvalidSearch is returned for empty or too long search terms.
	ErrInvalidSearch = errors.New("invalid search")
	// ErrFileMissing means a recorded file is no longer on disk (deleted over
	// SMB and not yet noticed by the scan).
	ErrFileMissing = errors.New("file missing from the library")
)

// GameView is a game's detail: files in the library, base first.
type GameView struct {
	Game  *domain.Game
	Items []domain.GameItem
	Size  int64
}

// Download describes what to send for a download request: a single file
// (resumable with Range) or a zip built while it is sent.
type Download struct {
	// Name is the suggested file name.
	Name string
	// File is set for single files.
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
	consoles Consoles
	repo     domain.LibraryRepository
	files    Files
}

// NewBrowseService builds the service.
func NewBrowseService(consoles Consoles, repo domain.LibraryRepository, files Files) *BrowseService {
	return &BrowseService{consoles: consoles, repo: repo, files: files}
}

// ConsoleGames lists the games of one console, by title.
func (s *BrowseService) ConsoleGames(ctx context.Context, slug string) ([]domain.GameSummary, error) {
	if _, err := s.consoles.Console(ctx, slug); err != nil {
		return nil, err
	}
	return s.list(ctx, func(g domain.GameSummary) bool { return string(g.Console) == slug })
}

// Search finds games whose title contains every word of q, ignoring case and
// accents ("pokemon" finds "Pokémon").
func (s *BrowseService) Search(ctx context.Context, q string) ([]domain.GameSummary, error) {
	words := strings.Fields(fold(q))
	if len(words) == 0 || len([]rune(q)) > 100 {
		return nil, ErrInvalidSearch
	}
	return s.list(ctx, func(g domain.GameSummary) bool {
		title := fold(g.Title)
		for _, w := range words {
			if !strings.Contains(title, w) {
				return false
			}
		}
		return true
	})
}

func (s *BrowseService) list(ctx context.Context, keep func(domain.GameSummary) bool) ([]domain.GameSummary, error) {
	games, err := s.repo.ListGames(ctx)
	if err != nil {
		return nil, err
	}
	out := []domain.GameSummary{}
	for _, g := range games {
		if keep(g) {
			out = append(out, g)
		}
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

// Game returns a game with its files in the library. A game whose files
// are all in the trash is not found.
func (s *BrowseService) Game(ctx context.Context, id domain.GameID) (*GameView, error) {
	g, err := s.repo.GameByID(ctx, id)
	if err != nil {
		return nil, err
	}
	items := g.LiveItems()
	if len(items) == 0 {
		return nil, domain.ErrGameNotFound
	}
	slices.SortStableFunc(items, compareItems)
	v := &GameView{Game: g, Items: items}
	for _, it := range items {
		v.Size += it.Size
	}
	return v, nil
}

// compareItems orders a game's files: base (or game), updates by version,
// then DLC by name.
func compareItems(a, b domain.GameItem) int {
	rank := map[domain.ItemKind]int{domain.KindGame: 0, domain.KindBase: 0, domain.KindUpdate: 1, domain.KindDLC: 2}
	return cmp.Or(
		cmp.Compare(rank[a.Kind], rank[b.Kind]),
		compareVersions(a, b),
		strings.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label)),
		cmp.Compare(a.ID, b.ID),
	)
}

// compareVersions orders updates numerically ("1.10" after "1.9").
func compareVersions(a, b domain.GameItem) int {
	if a.Kind != domain.KindUpdate || b.Kind != domain.KindUpdate {
		return 0
	}
	pa, pb := strings.Split(a.Label, "."), strings.Split(b.Label, ".")
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			_, _ = fmt.Sscan(pa[i], &x)
		}
		if i < len(pb) {
			_, _ = fmt.Sscan(pb[i], &y)
		}
		if c := cmp.Compare(x, y); c != 0 {
			return c
		}
	}
	return 0
}

// ItemDownload prepares the download of one file.
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
	return Download{Name: it.File, File: &entries[0].TreeFile}, nil
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
		files, err := s.files.Tree(s.files.LibraryPath(string(v.Game.Console), v.Game.Folder, it.File))
		if errors.Is(err, fs.ErrNotExist) || (err == nil && len(files) != 1) {
			return nil, fmt.Errorf("%w: %s/%s/%s", ErrFileMissing, v.Game.Console, v.Game.Folder, it.File)
		}
		if err != nil {
			return nil, err
		}
		out = append(out, ZipFile{Name: v.Game.Folder + "/" + it.File, TreeFile: files[0]})
	}
	return out, nil
}

// FileDownload prepares the download of a single file at an absolute path.
func (s *BrowseService) FileDownload(p, name string) (Download, error) {
	files, err := s.files.Tree(p)
	if errors.Is(err, fs.ErrNotExist) || (err == nil && len(files) != 1) {
		return Download{}, fmt.Errorf("%w: %s", ErrFileMissing, name)
	}
	if err != nil {
		return Download{}, err
	}
	return Download{Name: name, File: &files[0]}, nil
}

// Open opens a file of a Download.
func (s *BrowseService) Open(f TreeFile) (io.ReadSeekCloser, error) {
	return s.files.Open(f.Path)
}
