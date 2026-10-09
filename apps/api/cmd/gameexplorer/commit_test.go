package main

import (
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCommitSwitchFilesWithTheirKinds(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv, cookie, library := libraryServer(t)

	archive := zipOf(t, map[string]string{
		"pack/base.xci":   "base",
		"pack/update.nsp": "update",
		"pack/dlc.nsp":    "dlc",
		"pack/info.nfo":   "junk",
	})
	job := upload(t, srv, cookie, "pack.zip", "switch", "Limbo", archive)
	waitStatus(t, srv, cookie, job, "confirm")

	files := []map[string]any{
		{"path": "pack/base.xci", "kind": "base"},
		{"path": "pack/update.nsp", "kind": "update", "label": "v1.0.4"},
		{"path": "pack/dlc.nsp", "kind": "dlc", "label": "Fuga: Maestra"},
	}
	res := post(t, srv, cookie, "/api/jobs/"+job+"/plan", commitBody(t, files...))
	wantStatus(t, res, http.StatusOK, "plan")
	plan := decode[planJSON](t, res)
	names := map[string]string{}
	for _, f := range plan.Files {
		names[f.Path] = f.File
	}
	if plan.Folder != "Limbo" || plan.GameID != nil || names["pack/base.xci"] != "Limbo [BASE].xci" ||
		names["pack/update.nsp"] != "Limbo [UPDATE v1.0.4].nsp" || names["pack/dlc.nsp"] != "Limbo [DLC Fuga - Maestra].nsp" ||
		!slices.Equal(plan.Discarded, []string{"pack/info.nfo"}) {
		t.Fatalf("plan = %+v", plan)
	}

	res = post(t, srv, cookie, "/api/jobs/"+job+"/commit", commitBody(t, files...))
	wantStatus(t, res, http.StatusOK, "commit")
	c := decode[commitJSON](t, res)
	if c.Stored != 3 || c.Path != "switch/Limbo" || c.Job.Status != "done" {
		t.Fatalf("commit = %+v", c)
	}
	for _, name := range []string{"Limbo [BASE].xci", "Limbo [UPDATE v1.0.4].nsp", "Limbo [DLC Fuga - Maestra].nsp"} {
		if !exists(filepath.Join(library, "switch", "Limbo", name)) {
			t.Errorf("%s not stored", name)
		}
	}
	if exists(filepath.Join(stagingDir(library), job)) {
		t.Error("staging left behind (the .nfo is discarded with it)")
	}
	g, _ := gameOf(t, srv, cookie, c.GameID)
	if g.IgdbID != nil || len(g.Items) != 3 || g.Items[0].Kind != "base" || *g.Items[1].Label != "1.0.4" {
		t.Fatalf("game = %+v (base first, version stored without v)", g)
	}
}

func TestCommitWiiKeepsTheDoubleExtension(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)

	// One kind: the commit needs no kind, and a name picked from IGDB links the game.
	game := store(t, srv, cookie, "wii", "okami", "Ookami (USA).nkit.iso", "nkit", map[string]any{}, "igdbId", "26764")
	g, _ := gameOf(t, srv, cookie, game)
	if g.Title != "Mario Kart 8 Deluxe" || g.IgdbID == nil || *g.IgdbID != 26764 || g.CoverImageID == nil || g.Items[0].Kind != "game" {
		t.Fatalf("game = %+v (the IGDB name wins over what was typed)", g)
	}
	if got := readFile(t, filepath.Join(library, "wii", "Mario Kart 8 Deluxe", "Mario Kart 8 Deluxe.nkit.iso")); got != "nkit" {
		t.Fatalf("stored = %q", got)
	}
}

func TestCommitIntoAnExistingGameAndDuplicates(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)

	first := store(t, srv, cookie, "switch", "Inside", "inside.nsp", "v1", map[string]any{"kind": "base"})
	// The same name, in another case, joins the game.
	job := upload(t, srv, cookie, "inside2.nsp", "switch", "INSIDE", []byte("v2"))
	waitStatus(t, srv, cookie, job, "confirm")
	plan := decode[planJSON](t, post(t, srv, cookie, "/api/jobs/"+job+"/plan", commitBody(t, map[string]any{"path": "inside2.nsp", "kind": "base"})))
	if plan.GameID == nil || *plan.GameID != first || plan.Files[0].Action != "undecided" || plan.Files[0].Duplicate == nil {
		t.Fatalf("plan = %+v", plan)
	}
	res := post(t, srv, cookie, "/api/jobs/"+job+"/commit", commitBody(t, map[string]any{"path": "inside2.nsp", "kind": "base"}))
	wantStatus(t, res, http.StatusConflict, "undecided duplicate")
	if j := waitStatus(t, srv, cookie, job, "confirm"); j.Status != "confirm" {
		t.Fatal("job must stay waiting for confirmation")
	}
	res = post(t, srv, cookie, "/api/jobs/"+job+"/commit", commitBody(t, map[string]any{"path": "inside2.nsp", "kind": "base", "onDuplicate": "replace"}))
	wantStatus(t, res, http.StatusOK, "replace")
	if c := decode[commitJSON](t, res); c.GameID != first || c.Replaced != 1 {
		t.Fatalf("commit = %+v", c)
	}
	if got := readFile(t, filepath.Join(library, "switch", "Inside", "Inside [BASE].nsp")); got != "v2" {
		t.Fatalf("stored = %q", got)
	}
	trash := trashList(t, srv, cookie)
	if len(trash) != 1 || trash[0].Reason != "replaced" || trash[0].Items[0].File != "Inside [BASE].nsp" {
		t.Fatalf("trash = %+v", trash)
	}
}

func TestCommitValidation(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)

	job := upload(t, srv, cookie, "game.nsp", "switch", "Game", []byte("x"))
	waitStatus(t, srv, cookie, job, "confirm")
	cases := map[string]string{
		"missing file":     commitBody(t),
		"unknown file":     commitBody(t, map[string]any{"path": "other.nsp", "kind": "base"}),
		"no kind":          commitBody(t, map[string]any{"path": "game.nsp"}),
		"kind of wii":      commitBody(t, map[string]any{"path": "game.nsp", "kind": "game"}),
		"update without v": commitBody(t, map[string]any{"path": "game.nsp", "kind": "update"}),
		"bad version":      commitBody(t, map[string]any{"path": "game.nsp", "kind": "update", "label": "beta"}),
		"dlc without name": commitBody(t, map[string]any{"path": "game.nsp", "kind": "dlc", "label": "  "}),
	}
	for name, b := range cases {
		res := post(t, srv, cookie, "/api/jobs/"+job+"/commit", b)
		if res.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400: %s", name, res.StatusCode, body(t, res))
		}
	}
	res := post(t, srv, cookie, "/api/jobs/"+job+"/commit", commitBody(t, map[string]any{"path": "game.nsp", "kind": "update", "label": "beta"}))
	if !strings.Contains(body(t, res), "números y puntos") {
		t.Error("the version error must say what is expected")
	}
	if res := post(t, srv, cookie, "/api/jobs/nope/commit", commitBody(t)); res.StatusCode != http.StatusNotFound {
		t.Errorf("unknown job = %d, want 404", res.StatusCode)
	}
}
