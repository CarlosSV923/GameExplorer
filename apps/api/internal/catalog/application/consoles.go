// Package application holds the catalog use cases.
package application

import (
	"cmp"
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

// Console errors.
var (
	ErrConsoleNotFound = errors.New("console not found")
	// ErrExtensionInUse is returned when removing an extension that files use.
	ErrExtensionInUse = errors.New("extension in use")
	// ErrExtensionExists is returned when adding an extension the console has.
	ErrExtensionExists = errors.New("extension already present")
	// ErrFixedExtension is returned when removing an extension that comes
	// from the environment or the code.
	ErrFixedExtension = errors.New("fixed extension")
)

// InvalidConsoleError is a rejected console field; Message is shown to the user.
type InvalidConsoleError struct{ Message string }

func (e *InvalidConsoleError) Error() string { return e.Message }

// ConsoleService exposes the consoles: the ones defined in code with what the
// user and IGDB changed about them (RF-40..RF-42).
type ConsoleService struct {
	defs []domain.ConsoleDefinition
	// fixed are each console's extensions from the environment or the code.
	fixed map[domain.Slug][]string
	repo  domain.ConsoleRepository
}

// NewConsoleService builds the service. env holds the extension variables
// (ConsoleDefinition.ExtensionsEnv → comma-separated list); a console without
// one keeps its default extensions.
func NewConsoleService(defs []domain.ConsoleDefinition, repo domain.ConsoleRepository, env map[string]string) (*ConsoleService, error) {
	fixed := map[domain.Slug][]string{}
	for _, d := range defs {
		list := d.DefaultExtensions
		if v, ok := env[d.ExtensionsEnv]; ok && strings.TrimSpace(v) != "" {
			list = strings.Split(v, ",")
		}
		var exts []string
		for _, e := range list {
			ext, err := domain.NormalizeExtension(e)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", d.ExtensionsEnv, err)
			}
			if !slices.Contains(exts, ext) {
				exts = append(exts, ext)
			}
		}
		fixed[d.Slug] = exts
	}
	return &ConsoleService{defs: defs, fixed: fixed, repo: repo}, nil
}

// List returns every console in carousel order.
func (s *ConsoleService) List(ctx context.Context) ([]domain.Console, error) {
	settings, err := s.repo.Settings(ctx)
	if err != nil {
		return nil, fmt.Errorf("list consoles: %w", err)
	}
	custom, err := s.repo.CustomExtensions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list consoles: %w", err)
	}
	counts, err := s.repo.GameCounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list consoles: %w", err)
	}
	out := make([]domain.Console, 0, len(s.defs))
	for i, d := range s.defs {
		c := domain.Console{
			ConsoleDefinition: d, DisplayName: d.Name, SortOrder: (i + 1) * 10, Year: d.ReleaseYear,
			Extensions: s.fixed[d.Slug], GameCount: counts[d.Slug],
		}
		for _, ext := range custom[d.Slug] {
			if !slices.Contains(c.Extensions, ext) {
				c.Custom = append(c.Custom, ext)
			}
		}
		if st, ok := settings[d.Slug]; ok {
			c.SortOrder, c.LogoImageID = st.SortOrder, st.LogoImageID
			if st.DisplayName != nil {
				c.DisplayName = *st.DisplayName
			}
			if st.ReleaseYear != nil {
				c.Year = *st.ReleaseYear
			}
		}
		out = append(out, c)
	}
	slices.SortStableFunc(out, func(a, b domain.Console) int { return cmp.Compare(a.SortOrder, b.SortOrder) })
	return out, nil
}

// Console returns one console (ErrConsoleNotFound).
func (s *ConsoleService) Console(ctx context.Context, slug string) (domain.Console, error) {
	consoles, err := s.List(ctx)
	if err != nil {
		return domain.Console{}, err
	}
	for _, c := range consoles {
		if string(c.Slug) == slug {
			return c, nil
		}
	}
	return domain.Console{}, ErrConsoleNotFound
}

// KnownExtensions returns every extension of every console: the set a
// file's extension is looked up in (domain.ExtensionOf).
func KnownExtensions(consoles []domain.Console) []string {
	var out []string
	for _, c := range consoles {
		for _, e := range c.AllExtensions() {
			if !slices.Contains(out, e) {
				out = append(out, e)
			}
		}
	}
	return out
}

// Accepting returns the slugs of the consoles that accept a file name.
func Accepting(consoles []domain.Console, name string) []string {
	ext := domain.ExtensionOf(name, KnownExtensions(consoles))
	out := []string{}
	for _, c := range consoles {
		if c.Accepts(ext) {
			out = append(out, string(c.Slug))
		}
	}
	return out
}

// Rename changes a console's display name; the code's name resets it.
func (s *ConsoleService) Rename(ctx context.Context, slug, name string) (domain.Console, error) {
	c, err := s.Console(ctx, slug)
	if err != nil {
		return c, err
	}
	name = strings.Join(strings.Fields(name), " ")
	if n := len([]rune(name)); n < 1 || n > 60 {
		return c, &InvalidConsoleError{"El nombre debe tener entre 1 y 60 caracteres."}
	}
	var stored *string
	if name != c.Name {
		stored = &name
	}
	if err := s.seed(ctx); err != nil {
		return c, err
	}
	if err := s.repo.SetDisplayName(ctx, c.Slug, stored); err != nil {
		return c, err
	}
	return s.Console(ctx, slug)
}

// Reorder sets the carousel order; slugs must list every console once.
func (s *ConsoleService) Reorder(ctx context.Context, slugs []string) ([]domain.Console, error) {
	want := make([]string, 0, len(s.defs))
	for _, d := range s.defs {
		want = append(want, string(d.Slug))
	}
	got := slices.Clone(slugs)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		return nil, &InvalidConsoleError{"El orden debe incluir cada consola exactamente una vez."}
	}
	order := make([]domain.Slug, len(slugs))
	for i, sl := range slugs {
		order[i] = domain.Slug(sl)
	}
	if err := s.repo.SetOrder(ctx, order); err != nil {
		return nil, err
	}
	return s.List(ctx)
}

// AddExtension adds a custom extension to a console (RF-41).
func (s *ConsoleService) AddExtension(ctx context.Context, slug, ext string) (domain.Console, error) {
	c, err := s.Console(ctx, slug)
	if err != nil {
		return c, err
	}
	e, err := domain.NormalizeExtension(ext)
	if err != nil {
		return c, &InvalidConsoleError{fmt.Sprintf("Extensión no válida: %q. Usa un punto y letras o números, como .xcz o .nkit.iso.", ext)}
	}
	if slices.Contains(c.AllExtensions(), e) {
		return c, ErrExtensionExists
	}
	if err := s.repo.AddExtension(ctx, c.Slug, e); err != nil {
		return c, err
	}
	return s.Console(ctx, slug)
}

// RemoveExtension removes a custom extension that no file of the console
// uses (RF-41).
func (s *ConsoleService) RemoveExtension(ctx context.Context, slug, ext string) (domain.Console, error) {
	c, err := s.Console(ctx, slug)
	if err != nil {
		return c, err
	}
	e, err := domain.NormalizeExtension(ext)
	if err != nil {
		return c, &InvalidConsoleError{fmt.Sprintf("Extensión no válida: %q.", ext)}
	}
	if slices.Contains(c.Extensions, e) {
		return c, ErrFixedExtension
	}
	if !slices.Contains(c.Custom, e) {
		return c, ErrConsoleNotFound
	}
	uses, err := s.ExtensionUse(ctx, c)
	if err != nil {
		return c, err
	}
	if uses[e] > 0 {
		return c, ErrExtensionInUse
	}
	if err := s.repo.RemoveExtension(ctx, c.Slug, e); err != nil {
		return c, err
	}
	return s.Console(ctx, slug)
}

// ExtensionUse counts the console's files (trashed ones included) by
// custom extension.
func (s *ConsoleService) ExtensionUse(ctx context.Context, c domain.Console) (map[string]int, error) {
	out := map[string]int{}
	if len(c.Custom) == 0 {
		return out, nil
	}
	consoles, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	known := KnownExtensions(consoles)
	files, err := s.repo.ItemFiles(ctx, c.Slug)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		if ext := domain.ExtensionOf(f, known); slices.Contains(c.Custom, ext) {
			out[ext]++
		}
	}
	return out, nil
}

// SyncPlatformMetadata refreshes logo and release year of every console
// from IGDB. It returns how many consoles were updated.
func (s *ConsoleService) SyncPlatformMetadata(ctx context.Context, dir PlatformDirectory) (int, error) {
	bySlug := map[int64]domain.Slug{}
	ids := make([]int64, 0, len(s.defs))
	for _, d := range s.defs {
		bySlug[d.IGDBPlatformID] = d.Slug
		ids = append(ids, d.IGDBPlatformID)
	}
	infos, err := dir.PlatformsByID(ctx, ids)
	if err != nil {
		return 0, fmt.Errorf("sync platforms: %w", err)
	}
	if err := s.seed(ctx); err != nil {
		return 0, err
	}
	updated := 0
	for _, info := range infos {
		slug, ok := bySlug[info.IGDBPlatformID]
		if !ok {
			continue
		}
		if err := s.repo.SetPlatformMetadata(ctx, slug, info.LogoImageID, info.ReleaseYear); err != nil {
			return updated, fmt.Errorf("sync platform %d: %w", info.IGDBPlatformID, err)
		}
		updated++
	}
	return updated, nil
}

// seed stores the current order for every console, so later updates of a
// console's settings find its row (SetOrder creates the missing ones).
func (s *ConsoleService) seed(ctx context.Context) error {
	settings, err := s.repo.Settings(ctx)
	if err != nil {
		return err
	}
	if len(settings) == len(s.defs) {
		return nil
	}
	consoles, err := s.List(ctx)
	if err != nil {
		return err
	}
	order := make([]domain.Slug, len(consoles))
	for i, c := range consoles {
		order[i] = c.Slug
	}
	return s.repo.SetOrder(ctx, order)
}
