// Package application holds the catalog use cases.
package application

import (
	"context"
	"fmt"

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
