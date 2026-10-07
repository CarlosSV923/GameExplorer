package domain_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
)

func TestRankByNamePutsTheTypedGameFirst(t *testing.T) {
	t.Parallel()

	// Real IGDB order for `search "mario kart 8"` on Switch (phase 2 probe).
	games := []domain.Game{
		{ID: 191419, Name: "Mario Kart 8 Deluxe: Booster Course Pass"},
		{ID: 203219, Name: "Mario Kart 8 Deluxe + Super Mario Party Double Pack"},
		{ID: 26764, Name: "Mario Kart 8 Deluxe"},
		{ID: 1, Name: "Super Mario Kart 8 Fan Edition"},
	}

	got := ids(domain.RankByName(games, "mario kart 8 deluxe"))
	want := []int64{26764, 191419, 203219, 1}
	if !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}

	if got := ids(domain.RankByName(games, "MARIO kart 8")); got[0] != 191419 {
		t.Fatalf("prefix matches keep provider order, got %v", got)
	}
	if got := ids(domain.RankByName([]domain.Game{{ID: 2, Name: "Limbo"}, {ID: 3, Name: "INSIDE"}}, "inside")); got[0] != 3 {
		t.Fatalf("case-insensitive exact match must win, got %v", got)
	}
}

func ids(gs []domain.Game) []int64 {
	out := make([]int64, len(gs))
	for i, g := range gs {
		out[i] = g.ID
	}
	return out
}

func TestNormalizeQuery(t *testing.T) {
	t.Parallel()

	got, err := domain.NormalizeQuery("  mario   kart\t8 ")
	if err != nil || got != "mario kart 8" {
		t.Fatalf("got %q %v", got, err)
	}
	for _, bad := range []string{"", " a ", string(make([]rune, 101))} {
		if _, err := domain.NormalizeQuery(bad); !errors.Is(err, domain.ErrInvalidQuery) {
			t.Errorf("%q: err = %v", bad, err)
		}
	}
}

func TestNewImageRef(t *testing.T) {
	t.Parallel()

	r, err := domain.NewImageRef("logo_med", "plgu")
	if err != nil || r.Extension() != ".png" || r.ContentType() != "image/png" {
		t.Fatalf("logo ref = %+v %v", r, err)
	}
	r, err = domain.NewImageRef("cover_big", "co213p")
	if err != nil || r.Extension() != ".jpg" {
		t.Fatalf("cover ref = %+v %v", r, err)
	}
	for _, bad := range [][2]string{{"original", "co213p"}, {"cover_big", "../etc"}, {"cover_big", "CO213P"}, {"cover_big", ""}} {
		if _, err := domain.NewImageRef(bad[0], bad[1]); !errors.Is(err, domain.ErrInvalidImage) {
			t.Errorf("%v: err = %v", bad, err)
		}
	}
}
