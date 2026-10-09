package consoles_test

import (
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain/consoles"
)

// TestRegistryIsConsistent guards every console definition, so adding one
// (docs/adding-a-console.md) cannot break the others.
func TestRegistryIsConsistent(t *testing.T) {
	t.Parallel()
	slugs, envs, platforms := map[domain.Slug]bool{}, map[string]bool{}, map[int64]bool{}
	for _, d := range consoles.All() {
		if _, err := domain.NewSlug(string(d.Slug)); err != nil {
			t.Errorf("%s: %v", d.Slug, err)
		}
		if d.Slug == "_unassigned" || slugs[d.Slug] || envs[d.ExtensionsEnv] || platforms[d.IGDBPlatformID] {
			t.Errorf("%s: slug, variable or IGDB platform repeated or reserved", d.Slug)
		}
		slugs[d.Slug], envs[d.ExtensionsEnv], platforms[d.IGDBPlatformID] = true, true, true
		if d.Name == "" || d.IGDBPlatformID <= 0 || d.ReleaseYear < 1970 || d.ExtensionsEnv == "" {
			t.Errorf("%s: name, IGDB platform, year and variable are required", d.Slug)
		}
		if len(d.DefaultExtensions) == 0 {
			t.Errorf("%s: no extensions", d.Slug)
		}
		for _, e := range d.DefaultExtensions {
			if n, err := domain.NormalizeExtension(e); err != nil || n != e {
				t.Errorf("%s: extension %q must be normalized (%q, %v)", d.Slug, e, n, err)
			}
		}
		switch {
		case len(d.Kinds) == 0:
			t.Errorf("%s: no kinds", d.Slug)
		case d.SingleKind() == domain.KindGame && d.MultipleFiles:
			t.Errorf("%s: one file per game cannot take several files per upload", d.Slug)
		case d.SingleKind() == "" && !d.Allows(domain.KindBase):
			t.Errorf("%s: a console with add-ons needs a base", d.Slug)
		}
	}
}
