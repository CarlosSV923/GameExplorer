package domain_test

import (
	"errors"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

func TestNewSlug(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"switch", "psp", "wiiu", "snes_jp", "n3ds-homebrew"} {
		if _, err := domain.NewSlug(s); err != nil {
			t.Errorf("NewSlug(%q) = %v, want ok", s, err)
		}
	}
	for _, s := range []string{"", "PSP", "../etc", "ps 2", "-ps2", "a/b", "ps2.", "abcdefghijklmnopqrstuvwxyz0123456"} {
		if _, err := domain.NewSlug(s); !errors.Is(err, domain.ErrInvalidSlug) {
			t.Errorf("NewSlug(%q) = %v, want ErrInvalidSlug", s, err)
		}
	}
}

func TestNormalizeExtension(t *testing.T) {
	t.Parallel()
	ok := map[string]string{"iso": ".iso", ".XCI": ".xci", "nkit.iso": ".nkit.iso", ".7z": ".7z"}
	for in, want := range ok {
		if got, err := domain.NormalizeExtension(in); err != nil || got != want {
			t.Errorf("NormalizeExtension(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", ".", "..iso", ".i so", ".a.b.c.d", ".toolongextension", ".is/o"} {
		if _, err := domain.NormalizeExtension(bad); !errors.Is(err, domain.ErrInvalidExtension) {
			t.Errorf("NormalizeExtension(%q) = %v, want ErrInvalidExtension", bad, err)
		}
	}
}

func TestConsoleAcceptsItsExtensions(t *testing.T) {
	t.Parallel()
	c := domain.Console{
		ConsoleDefinition: domain.ConsoleDefinition{Kinds: []domain.ItemKind{domain.KindGame}},
		Extensions:        []string{".iso"}, Custom: []string{".pbp"},
	}
	if !c.Accepts(".iso") || !c.Accepts(".pbp") || c.Accepts(".nkit.iso") || c.Accepts("") {
		t.Fatal("Accepts")
	}
	if c.SingleKind() != domain.KindGame || !c.Allows(domain.KindGame) || c.Allows(domain.KindBase) {
		t.Fatal("kinds")
	}
}
