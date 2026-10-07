// Package application holds the catalog use cases.
package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// PlatformInfo is what the catalog needs to know about a console's platform.
type PlatformInfo struct {
	IGDBPlatformID int64
	LogoImageID    *string
	ReleaseYear    *int
}

// PlatformDirectory looks up platform information by IGDB id. It is the
// catalog's port to the metadata context (wired in the composition root).
type PlatformDirectory interface {
	PlatformsByID(ctx context.Context, ids []int64) ([]PlatformInfo, error)
}

// ConsoleService exposes the console use cases.
type ConsoleService struct {
	repo domain.ConsoleRepository
}

// NewConsoleService builds the service.
func NewConsoleService(repo domain.ConsoleRepository) *ConsoleService {
	return &ConsoleService{repo: repo}
}

// List returns every console in carousel order.
func (s *ConsoleService) List(ctx context.Context) ([]domain.Console, error) {
	consoles, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list consoles: %w", err)
	}
	return consoles, nil
}

// SyncPlatformMetadata refreshes logo and release year of every console linked
// to an IGDB platform. It returns how many consoles were updated.
func (s *ConsoleService) SyncPlatformMetadata(ctx context.Context, dir PlatformDirectory) (int, error) {
	consoles, err := s.repo.List(ctx)
	if err != nil {
		return 0, fmt.Errorf("sync platforms: %w", err)
	}
	byPlatform := map[int64]domain.Console{}
	ids := make([]int64, 0, len(consoles))
	for _, c := range consoles {
		if c.IGDBPlatformID != nil {
			byPlatform[*c.IGDBPlatformID] = c
			ids = append(ids, *c.IGDBPlatformID)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}

	infos, err := dir.PlatformsByID(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("sync platforms: %w", err)
	}
	updated := 0
	for _, info := range infos {
		c, ok := byPlatform[info.IGDBPlatformID]
		if !ok {
			continue
		}
		if err := s.repo.UpdatePlatformMetadata(ctx, c.ID, info.LogoImageID, info.ReleaseYear); err != nil {
			return updated, fmt.Errorf("sync platform %d: %w", info.IGDBPlatformID, err)
		}
		updated++
	}
	return updated, nil
}

// Console management errors.
var (
	// ErrConsoleInUse is returned when a console with games would lose its
	// folder (slug change, deletion).
	ErrConsoleInUse = errors.New("console has games")
	// ErrBuiltInConsole is returned when deleting one of the six seeded consoles.
	ErrBuiltInConsole = errors.New("built-in consoles cannot be deleted")
)

// InvalidConsoleError is a rejected console field; Message is shown to the user.
type InvalidConsoleError struct{ Message string }

func (e *InvalidConsoleError) Error() string { return e.Message }

// ConsoleInput is what the user defines for a console (RF-41).
type ConsoleInput struct {
	Slug        string
	DisplayName string
	Extensions  []string
}

func (in ConsoleInput) validate() (domain.Slug, string, []string, error) {
	slug, err := domain.NewSlug(strings.TrimSpace(in.Slug))
	if err != nil {
		return "", "", nil, &InvalidConsoleError{"La carpeta debe usar minúsculas, números, '-' o '_' (máximo 32)."}
	}
	name := strings.Join(strings.Fields(in.DisplayName), " ")
	if n := len([]rune(name)); n < 1 || n > 60 {
		return "", "", nil, &InvalidConsoleError{"El nombre debe tener entre 1 y 60 caracteres."}
	}
	var exts []string
	for _, e := range in.Extensions {
		ext, err := domain.NormalizeExtension(e)
		if err != nil {
			return "", "", nil, &InvalidConsoleError{fmt.Sprintf("Extensión no válida: %q.", e)}
		}
		if !slices.Contains(exts, ext) {
			exts = append(exts, ext)
		}
	}
	if len(exts) == 0 {
		return "", "", nil, &InvalidConsoleError{"Indica al menos una extensión (por ejemplo .z64)."}
	}
	return slug, name, exts, nil
}

// Create adds a console for an IGDB platform; it goes last in the carousel.
func (s *ConsoleService) Create(ctx context.Context, igdbPlatformID int64, in ConsoleInput, dir PlatformDirectory) (domain.Console, error) {
	slug, name, exts, err := in.validate()
	if err != nil {
		return domain.Console{}, err
	}
	infos, err := dir.PlatformsByID(ctx, []int64{igdbPlatformID})
	if err != nil {
		return domain.Console{}, fmt.Errorf("look up platform: %w", err)
	}
	i := slices.IndexFunc(infos, func(p PlatformInfo) bool { return p.IGDBPlatformID == igdbPlatformID })
	if i < 0 {
		return domain.Console{}, &InvalidConsoleError{fmt.Sprintf("La plataforma %d no existe en IGDB.", igdbPlatformID)}
	}
	c := domain.Console{
		Slug: slug, DisplayName: name, Extensions: exts, IGDBPlatformID: &igdbPlatformID,
		LogoImageID: infos[i].LogoImageID, ReleaseYear: infos[i].ReleaseYear,
	}
	if c.ID, err = s.repo.Create(ctx, c); err != nil {
		return domain.Console{}, err
	}
	return s.byID(ctx, c.ID)
}

// Update changes a console's name, extensions and, while it has no games,
// its folder (slug).
func (s *ConsoleService) Update(ctx context.Context, id domain.ConsoleID, in ConsoleInput) (domain.Console, error) {
	c, err := s.byID(ctx, id)
	if err != nil {
		return c, err
	}
	slug, name, exts, err := in.validate()
	if err != nil {
		return c, err
	}
	if slug != c.Slug {
		inUse, err := s.repo.HasGames(ctx, id)
		if err != nil {
			return c, err
		}
		if inUse {
			return c, ErrConsoleInUse
		}
	}
	c.Slug, c.DisplayName, c.Extensions = slug, name, exts
	if err := s.repo.Update(ctx, c); err != nil {
		return c, err
	}
	return s.byID(ctx, id)
}

// Delete removes a console added by the user that has no games (not even
// in the trash).
func (s *ConsoleService) Delete(ctx context.Context, id domain.ConsoleID) error {
	c, err := s.byID(ctx, id)
	if err != nil {
		return err
	}
	if c.DetectorKey != "" {
		return ErrBuiltInConsole
	}
	inUse, err := s.repo.HasGames(ctx, id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrConsoleInUse
	}
	return s.repo.Delete(ctx, id)
}

// Reorder sets the carousel order; ids must list every console once.
func (s *ConsoleService) Reorder(ctx context.Context, ids []domain.ConsoleID) ([]domain.Console, error) {
	consoles, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	want := make([]domain.ConsoleID, 0, len(consoles))
	for _, c := range consoles {
		want = append(want, c.ID)
	}
	got := slices.Clone(ids)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		return nil, &InvalidConsoleError{"El orden debe incluir cada consola exactamente una vez."}
	}
	if err := s.repo.SetOrder(ctx, ids); err != nil {
		return nil, err
	}
	return s.List(ctx)
}

func (s *ConsoleService) byID(ctx context.Context, id domain.ConsoleID) (domain.Console, error) {
	consoles, err := s.List(ctx)
	if err != nil {
		return domain.Console{}, err
	}
	for _, c := range consoles {
		if c.ID == id {
			return c, nil
		}
	}
	return domain.Console{}, ErrConsoleNotFound
}
