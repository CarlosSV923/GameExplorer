package main

import (
	"net/http"
	"path/filepath"
	"testing"
)

func TestGamesOfSeveralDiscs(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv, cookie, library := libraryServer(t)

	archive := zipOf(t, map[string]string{"RE4 (Disc 1).iso": "one", "RE4 (Disc 2).iso": "two"})
	job := upload(t, srv, cookie, "re4.zip", "gc", "Resident Evil 4", archive)
	waitStatus(t, srv, cookie, job, "confirm")

	// Several files of one game must be numbered discs (spec §5).
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job+"/commit", commitBody(t,
		map[string]any{"path": "RE4 (Disc 1).iso"}, map[string]any{"path": "RE4 (Disc 2).iso"},
	)), http.StatusBadRequest, "two games in one folder")
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job+"/commit", commitBody(t,
		map[string]any{"path": "RE4 (Disc 1).iso", "kind": "disc", "label": "1"},
		map[string]any{"path": "RE4 (Disc 2).iso", "kind": "disc", "label": "02"},
	)), http.StatusOK, "commit discs")
	dir := filepath.Join(library, "gc", "Resident Evil 4")
	if readFile(t, filepath.Join(dir, "Resident Evil 4 (Disc 1).iso")) != "one" || readFile(t, filepath.Join(dir, "Resident Evil 4 (Disc 2).iso")) != "two" {
		t.Fatal("discs not stored under their numbers")
	}
}

func TestAnotherDiscRenumbersTheStoredOne(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, "ps2", "Xenosaga", "x1.iso", "one", map[string]any{})
	if !exists(filepath.Join(library, "ps2", "Xenosaga", "Xenosaga.iso")) {
		t.Fatal("a game of one disc has no tag")
	}
	g, _ := gameOf(t, srv, cookie, game)

	job := upload(t, srv, cookie, "x2.iso", "ps2", "Xenosaga", []byte("two"))
	waitStatus(t, srv, cookie, job, "confirm")
	disc2 := map[string]any{"path": "x2.iso", "kind": "disc", "label": "2"}
	// The preview shows the stored disc; storing needs its number too.
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job+"/plan", commitBody(t, disc2)), http.StatusOK, "plan without renumbering")
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job+"/commit", commitBody(t, disc2)), http.StatusBadRequest, "the stored disc needs its number")

	body := jsonBody(t, map[string]any{
		"files":    []map[string]any{disc2},
		"renumber": []map[string]any{{"itemId": g.Items[0].ID, "label": "1"}},
	})
	res := post(t, srv, cookie, "/api/jobs/"+job+"/plan", body)
	wantStatus(t, res, http.StatusOK, "plan")
	plan := decode[struct {
		Renamed []struct {
			File string `json:"file"`
		} `json:"renamed"`
	}](t, res)
	if len(plan.Renamed) != 1 || plan.Renamed[0].File != "Xenosaga (Disc 1).iso" {
		t.Fatalf("plan = %+v", plan)
	}
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job+"/commit", body), http.StatusOK, "commit")
	dir := filepath.Join(library, "ps2", "Xenosaga")
	if readFile(t, filepath.Join(dir, "Xenosaga (Disc 1).iso")) != "one" || readFile(t, filepath.Join(dir, "Xenosaga (Disc 2).iso")) != "two" || exists(filepath.Join(dir, "Xenosaga.iso")) {
		t.Fatal("the stored disc must be renamed with the new one")
	}
	g, _ = gameOf(t, srv, cookie, game)
	if len(g.Items) != 2 || g.Items[0].Kind != "disc" || g.Items[1].Kind != "disc" {
		t.Fatalf("game = %+v", g)
	}
}

func TestNintendo64TakesItsThreeDumpFormats(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	store(t, srv, cookie, "n64", "Super Mario 64", "mario.v64", "rom", map[string]any{})
	if readFile(t, filepath.Join(library, "n64", "Super Mario 64", "Super Mario 64.v64")) != "rom" {
		t.Fatal("rom not stored")
	}
}

func TestEditAndMoveAGameOfDiscs(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, "gc", "Tales", "t1.iso", "one", map[string]any{"kind": "disc", "label": "1"})
	store(t, srv, cookie, "gc", "Tales", "t2.iso", "two", map[string]any{"kind": "disc", "label": "2"})
	g, _ := gameOf(t, srv, cookie, game)

	// The number of a disc can change (RF-24).
	wantStatus(t, call(t, srv, cookie, http.MethodPatch, "/api/items/"+id(g.Items[1].ID), `{"kind":"disc","label":"3"}`), http.StatusOK, "renumber")
	if !exists(filepath.Join(library, "gc", "Tales", "Tales (Disc 3).iso")) {
		t.Fatal("disc not renamed")
	}
	// Discs move between consoles with discs only.
	wantStatus(t, post(t, srv, cookie, "/api/games/"+id(game)+"/edit", `{"console":"wii","title":"Tales"}`), http.StatusBadRequest, "discs to wii")
	wantStatus(t, post(t, srv, cookie, "/api/games/"+id(game)+"/edit", `{"console":"ps2","title":"Tales"}`), http.StatusOK, "discs to ps2")
	if !exists(filepath.Join(library, "ps2", "Tales", "Tales (Disc 1).iso")) {
		t.Fatal("game not moved")
	}
}
