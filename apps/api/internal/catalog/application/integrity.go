package application

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

// IntegrityReport summarizes an integrity check.
type IntegrityReport struct {
	CheckedAt    time.Time
	Checked      int // items looked at
	Missing      int // items that went missing in this check
	Found        int // missing items whose files are back
	MissingTotal int // items still missing afterwards
}

// ErrNotMissing is returned when forgetting an item whose files are on disk.
var ErrNotMissing = errors.New("item is not missing")

// CheckIntegrity marks the items whose files were deleted outside the app
// (over SMB) as missing, and unmarks them when the files come back (RF-26).
func (s *LibraryService) CheckIntegrity(ctx context.Context) (IntegrityReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rep := IntegrityReport{CheckedAt: s.now()}
	games, err := s.repo.ListGames(ctx)
	if err != nil {
		return rep, err
	}
	consoles, err := s.consoles.List(ctx)
	if err != nil {
		return rep, err
	}
	slugs := map[domain.ConsoleID]string{}
	for _, c := range consoles {
		slugs[c.ID] = string(c.Slug)
	}

	type change struct {
		id    domain.ItemID
		since *time.Time
	}
	var changes []change
	now := s.now()
	for _, summary := range games {
		g, err := s.repo.GameByID(ctx, summary.ID)
		if err != nil {
			return rep, err
		}
		dir := s.files.LibraryPath(slugs[g.ConsoleID], g.Folder)
		for _, it := range g.LiveItems() {
			rep.Checked++
			complete := true
			for _, f := range it.Files {
				ok, err := s.files.Exists(filepath.Join(dir, f))
				if err != nil {
					return rep, err
				}
				complete = complete && ok
			}
			if !complete {
				rep.MissingTotal++
			}
			switch {
			case !complete && it.MissingSince == nil:
				rep.Missing++
				changes = append(changes, change{it.ID, &now})
				s.log.Warn("item missing from disk", "item", it.ID, "dir", dir)
			case complete && it.MissingSince != nil:
				rep.Found++
				changes = append(changes, change{it.ID, nil})
			}
		}
	}
	if len(changes) == 0 {
		s.lastCheck = &rep
		return rep, nil
	}
	err = s.repo.Apply(ctx, "", func(tx domain.LibraryTx) error {
		for _, c := range changes {
			if err := tx.SetMissing(ctx, c.id, c.since); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		s.lastCheck = &rep
	}
	return rep, err
}

// LastIntegrity returns the last integrity check, or nil before the first.
func (s *LibraryService) LastIntegrity() *IntegrityReport {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lastCheck == nil {
		return nil
	}
	rep := *s.lastCheck
	return &rep
}

// Forget removes a missing item from the library: there are no files left
// to move or delete.
func (s *LibraryService) Forget(ctx context.Context, id domain.ItemID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, err := s.repo.ItemByID(ctx, id)
	if err != nil {
		return err
	}
	if !it.Live() {
		return domain.ErrItemNotFound
	}
	if it.MissingSince == nil {
		return ErrNotMissing
	}
	return s.repo.Apply(ctx, "", func(tx domain.LibraryTx) error {
		if err := tx.DeleteItem(ctx, it.ID); err != nil {
			return err
		}
		return tx.DeleteGameIfEmpty(ctx, it.GameID)
	})
}

// RunMaintenance checks integrity and purges the expired trash at start and
// then every interval, until ctx ends.
func (s *LibraryService) RunMaintenance(ctx context.Context, interval, retention time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if rep, err := s.CheckIntegrity(ctx); err != nil && ctx.Err() == nil {
			s.log.Warn("integrity check failed", "error", err)
		} else if rep.Missing > 0 || rep.Found > 0 {
			s.log.Info("integrity check", "checked", rep.Checked, "missing", rep.Missing, "found", rep.Found)
		}
		if n, err := s.PurgeExpired(ctx, retention); err != nil && ctx.Err() == nil {
			s.log.Warn("trash purge failed", "error", err)
		} else if n > 0 {
			s.log.Info("trash purged", "entries", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
