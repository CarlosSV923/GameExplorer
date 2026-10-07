package domain_test

import (
	"errors"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

func TestNewSlug(t *testing.T) {
	t.Parallel()

	valid := []string{"switch", "ps2", "gc", "wiiu", "snes_jp", "n3ds-homebrew"}
	for _, s := range valid {
		if _, err := domain.NewSlug(s); err != nil {
			t.Errorf("NewSlug(%q) = %v, want ok", s, err)
		}
	}

	invalid := []string{"", "PS2", "../etc", "ps 2", "-ps2", "a/b", "ps2.", "abcdefghijklmnopqrstuvwxyz0123456"}
	for _, s := range invalid {
		if _, err := domain.NewSlug(s); !errors.Is(err, domain.ErrInvalidSlug) {
			t.Errorf("NewSlug(%q) = %v, want ErrInvalidSlug", s, err)
		}
	}
}

func TestNormalizeExtension(t *testing.T) {
	t.Parallel()

	tests := []struct{ in, want string }{
		{".ISO", ".iso"},
		{"nsp", ".nsp"},
		{" .Rvz ", ".rvz"}, // surrounding whitespace is trimmed
		{".7z", ".7z"},
	}
	for _, tt := range tests {
		got, err := domain.NormalizeExtension(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("NormalizeExtension(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}

	for _, in := range []string{"", ".", ".tar.gz", "../x", ".averyveryverylongext"} {
		if _, err := domain.NormalizeExtension(in); !errors.Is(err, domain.ErrInvalidExtension) {
			t.Errorf("NormalizeExtension(%q) = %v, want ErrInvalidExtension", in, err)
		}
	}
}
