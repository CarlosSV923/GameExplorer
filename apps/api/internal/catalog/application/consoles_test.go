package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain/consoles"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/infrastructure/sqlite"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/database"
)

type fakeDirectory struct{ gotIDs []int64 }

func (f *fakeDirectory) PlatformsByID(_ context.Context, ids []int64) ([]application.PlatformInfo, error) {
	f.gotIDs = ids
	logo := "plgu"
	return []application.PlatformInfo{
		{IGDBPlatformID: 130, LogoImageID: &logo}, // no year: keep the code's one
		{IGDBPlatformID: 999, LogoImageID: &logo}, // not one of ours: ignored
	}, nil
}

func consoleService(t *testing.T, env map[string]string) *application.ConsoleService {
	t.Helper()
	conn, err := database.Open(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	svc, err := application.NewConsoleService(consoles.All(), sqlite.NewConsoleRepository(conn), env)
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestSyncPlatformMetadata(t *testing.T) {
	t.Parallel()
	svc := consoleService(t, nil)
	dir := &fakeDirectory{}

	n, err := svc.SyncPlatformMetadata(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 || len(dir.gotIDs) != 3 {
		t.Fatalf("updated %d (want 1), asked for %v", n, dir.gotIDs)
	}
	list, _ := svc.List(t.Context())
	sw := list[0]
	if sw.LogoImageID == nil || *sw.LogoImageID != "plgu" || sw.Year != 2017 {
		t.Fatalf("switch after sync = %+v", sw)
	}
}

func TestExtensionsFromTheEnvironment(t *testing.T) {
	t.Parallel()
	svc := consoleService(t, map[string]string{"SWITCH_EXTENSIONS": " NSP, .xci ,.nsz", "WII_EXTENSIONS": ""})
	sw, _ := svc.Console(t.Context(), "switch")
	wii, _ := svc.Console(t.Context(), "wii")
	if !slices.Equal(sw.Extensions, []string{".nsp", ".xci", ".nsz"}) || !slices.Equal(wii.Extensions, []string{".iso", ".wbfs", ".rvz", ".nkit.iso"}) {
		t.Fatalf("switch %v, wii %v (an empty variable keeps the defaults)", sw.Extensions, wii.Extensions)
	}

	conn, _ := database.Open(t.Context(), t.TempDir())
	t.Cleanup(func() { _ = conn.Close() })
	if _, err := application.NewConsoleService(consoles.All(), sqlite.NewConsoleRepository(conn), map[string]string{"PSP_EXTENSIONS": "iso,c s o"}); err == nil {
		t.Fatal("a bad extension in the environment must stop the start")
	}
}

func TestConsoleSettingsAndCustomExtensions(t *testing.T) {
	t.Parallel()
	svc := consoleService(t, nil)
	ctx := t.Context()

	if _, err := svc.Rename(ctx, "wii", "Nintendo Wii"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reorder(ctx, []string{"psp", "wii", "switch"}); err != nil {
		t.Fatal(err)
	}
	list, _ := svc.List(ctx)
	if list[0].Slug != "psp" || list[1].DisplayName != "Nintendo Wii" || list[2].DisplayName != "Nintendo Switch" {
		t.Fatalf("consoles = %+v", list)
	}
	// Back to the code's name.
	if c, _ := svc.Rename(ctx, "wii", "Wii"); c.DisplayName != "Wii" {
		t.Fatalf("wii = %+v", c)
	}
	var invalid *application.InvalidConsoleError
	if _, err := svc.Rename(ctx, "wii", "  "); !errors.As(err, &invalid) {
		t.Fatalf("blank name err = %v", err)
	}

	c, err := svc.AddExtension(ctx, "psp", "PBP")
	if err != nil || !slices.Equal(c.Custom, []string{".pbp"}) || !c.Accepts(".pbp") {
		t.Fatalf("add = %+v, %v", c, err)
	}
	if _, err := svc.AddExtension(ctx, "psp", ".iso"); !errors.Is(err, application.ErrExtensionExists) {
		t.Fatalf("add a fixed one err = %v", err)
	}
	if _, err := svc.RemoveExtension(ctx, "psp", ".cso"); !errors.Is(err, application.ErrFixedExtension) {
		t.Fatalf("remove a fixed one err = %v", err)
	}
	if c, err := svc.RemoveExtension(ctx, "psp", ".pbp"); err != nil || len(c.Custom) != 0 {
		t.Fatalf("remove = %+v, %v", c, err)
	}
	if _, err := svc.Console(ctx, "ps2"); !errors.Is(err, application.ErrConsoleNotFound) {
		t.Fatalf("unknown console err = %v", err)
	}
}
