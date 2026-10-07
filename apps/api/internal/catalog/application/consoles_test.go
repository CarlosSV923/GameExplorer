package application_test

import (
	"context"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
)

type fakeDirectory struct{ gotIDs []int64 }

func (f *fakeDirectory) PlatformsByID(_ context.Context, ids []int64) ([]application.PlatformInfo, error) {
	f.gotIDs = ids
	logo := "plgu"
	return []application.PlatformInfo{
		{IGDBPlatformID: 130, LogoImageID: &logo}, // no year: keep the seeded one
		{IGDBPlatformID: 999, LogoImageID: &logo}, // not one of ours: ignored
	}, nil
}

func TestSyncPlatformMetadata(t *testing.T) {
	t.Parallel()

	conn, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	svc := application.NewConsoleService(sqlite.NewConsoleRepository(conn))
	dir := &fakeDirectory{}

	n, err := svc.SyncPlatformMetadata(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(dir.gotIDs) != 6 {
		t.Fatalf("updated %d (want 1), asked for %v", n, dir.gotIDs)
	}

	consoles, _ := svc.List(t.Context())
	sw := consoles[0]
	if sw.LogoImageID == nil || *sw.LogoImageID != "plgu" || sw.ReleaseYear == nil || *sw.ReleaseYear != 2017 {
		t.Fatalf("switch after sync = %+v", sw)
	}
}
