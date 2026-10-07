// Package application holds the catalog use cases.
package application

import (
	"context"
	"fmt"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

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
