package application_test

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
)

type fakeProvider struct {
	games     []domain.Game
	gotLimit  int
	gotQuery  string
	gotPlatID *int64
}

func (f *fakeProvider) SearchGames(_ context.Context, q string, p *int64, limit int) ([]domain.Game, error) {
	f.gotQuery, f.gotPlatID, f.gotLimit = q, p, limit
	return f.games, nil
}

func (f *fakeProvider) SearchPlatforms(context.Context, string, int) ([]domain.Platform, error) {
	return []domain.Platform{{ID: 41, Name: "Wii U"}}, nil
}

func (f *fakeProvider) GameByID(_ context.Context, id int64) (*domain.Game, error) {
	for _, g := range f.games {
		if g.ID == id {
			return &g, nil
		}
	}
	return nil, domain.ErrNotFound
}

func (f *fakeProvider) PlatformsByID(context.Context, []int64) ([]domain.Platform, error) {
	return nil, nil
}

type noImages struct{}

func (noImages) Open(context.Context, domain.ImageRef) (io.ReadCloser, int64, error) {
	return nil, 0, domain.ErrNotFound
}

func TestSearchGamesNormalizesRanksAndLimits(t *testing.T) {
	t.Parallel()

	p := &fakeProvider{games: []domain.Game{
		{ID: 1, Name: "Mario Kart 8 Deluxe: Booster Course Pass"},
		{ID: 2, Name: "Mario Kart 8 Deluxe"},
		{ID: 3, Name: "Mario Kart 8 Deluxe + Party Pack"},
	}}
	svc := application.NewService(p, noImages{})
	platform := int64(130)

	got, err := svc.SearchGames(t.Context(), "  Mario  Kart 8 Deluxe ", &platform, 2)
	if err != nil {
		t.Fatal(err)
	}
	if p.gotQuery != "Mario Kart 8 Deluxe" || p.gotPlatID == nil || *p.gotPlatID != 130 || p.gotLimit != 4 {
		t.Fatalf("provider called with q=%q platform=%v limit=%d", p.gotQuery, p.gotPlatID, p.gotLimit)
	}
	if len(got) != 2 || got[0].ID != 2 {
		t.Fatalf("result = %+v, want exact title first and 2 items", got)
	}
}

func TestSearchWithoutProvider(t *testing.T) {
	t.Parallel()

	svc := application.NewService(nil, noImages{})
	if svc.Configured() {
		t.Fatal("Configured() = true without provider")
	}
	if _, err := svc.SearchGames(t.Context(), "zelda", nil, 0); !errors.Is(err, domain.ErrNotConfigured) {
		t.Fatalf("err = %v", err)
	}
	if _, err := svc.SearchPlatforms(t.Context(), "wii"); !errors.Is(err, domain.ErrNotConfigured) {
		t.Fatalf("err = %v", err)
	}
}

func TestSearchRejectsShortQueries(t *testing.T) {
	t.Parallel()

	svc := application.NewService(&fakeProvider{}, noImages{})
	if _, err := svc.SearchGames(t.Context(), "z", nil, 0); !errors.Is(err, domain.ErrInvalidQuery) {
		t.Fatalf("err = %v", err)
	}
}

func TestImageValidatesReference(t *testing.T) {
	t.Parallel()

	svc := application.NewService(nil, noImages{})
	if _, _, _, err := svc.Image(t.Context(), "cover_big", "../../etc/passwd"); !errors.Is(err, domain.ErrInvalidImage) {
		t.Fatalf("err = %v", err)
	}
	if _, _, _, err := svc.Image(t.Context(), "cover_big", "co213p"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("err = %v", err)
	}
}
