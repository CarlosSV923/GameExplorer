// Package application holds the metadata use cases and the ports they need.
package application

import (
	"context"
	"io"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
)

// Provider is the external metadata source (IGDB).
type Provider interface {
	SearchGames(ctx context.Context, query string, platformID *int64, limit int) ([]domain.Game, error)
	SearchPlatforms(ctx context.Context, query string, limit int) ([]domain.Platform, error)
	PlatformsByID(ctx context.Context, ids []int64) ([]domain.Platform, error)
}

// ImageStore returns images, fetching and caching them on first use.
type ImageStore interface {
	// Open returns domain.ErrNotFound if the image does not exist upstream.
	Open(ctx context.Context, ref domain.ImageRef) (io.ReadCloser, int64, error)
}

// Service implements the metadata use cases. A nil provider means IGDB is not
// configured: searches fail with domain.ErrNotConfigured, while images that
// are already cached keep working.
type Service struct {
	provider Provider
	images   ImageStore
}

// NewService builds the service. provider may be nil.
func NewService(provider Provider, images ImageStore) *Service {
	return &Service{provider: provider, images: images}
}

// Configured reports whether a provider is available.
func (s *Service) Configured() bool { return s.provider != nil }

const (
	defaultLimit = 10
	maxLimit     = 25
)

// SearchGames returns the best matches for query, optionally on one platform.
func (s *Service) SearchGames(ctx context.Context, query string, platformID *int64, limit int) ([]domain.Game, error) {
	if s.provider == nil {
		return nil, domain.ErrNotConfigured
	}
	q, err := domain.NormalizeQuery(query)
	if err != nil {
		return nil, err
	}
	limit = clampLimit(limit)
	// Over-fetch so re-ranking can promote the exact title past bundles/DLC.
	games, err := s.provider.SearchGames(ctx, q, platformID, min(limit*2, 50))
	if err != nil {
		return nil, err
	}
	ranked := domain.RankByName(games, q)
	return ranked[:min(limit, len(ranked))], nil
}

// SearchPlatforms finds platforms by name.
func (s *Service) SearchPlatforms(ctx context.Context, query string) ([]domain.Platform, error) {
	if s.provider == nil {
		return nil, domain.ErrNotConfigured
	}
	q, err := domain.NormalizeQuery(query)
	if err != nil {
		return nil, err
	}
	return s.provider.SearchPlatforms(ctx, q, defaultLimit)
}

// PlatformsByID loads platforms by IGDB id.
func (s *Service) PlatformsByID(ctx context.Context, ids []int64) ([]domain.Platform, error) {
	if s.provider == nil {
		return nil, domain.ErrNotConfigured
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return s.provider.PlatformsByID(ctx, ids)
}

// Image opens a cached (or freshly downloaded) image.
func (s *Service) Image(ctx context.Context, size, id string) (io.ReadCloser, int64, domain.ImageRef, error) {
	ref, err := domain.NewImageRef(size, id)
	if err != nil {
		return nil, 0, domain.ImageRef{}, err
	}
	rc, n, err := s.images.Open(ctx, ref)
	return rc, n, ref, err
}

func clampLimit(n int) int {
	switch {
	case n <= 0:
		return defaultLimit
	case n > maxLimit:
		return maxLimit
	default:
		return n
	}
}
