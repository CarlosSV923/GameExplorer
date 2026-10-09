package main

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrashAndRestore(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, "switch", "Limbo", "b.nsp", "base", map[string]any{"kind": "base"})
	store(t, srv, cookie, "switch", "Limbo", "u.nsp", "update", map[string]any{"kind": "update", "label": "2"})
	g, _ := gameOf(t, srv, cookie, game)

	// One file.
	wantStatus(t, post(t, srv, cookie, "/api/items/"+id(g.Items[1].ID)+"/trash", ""), http.StatusNoContent, "trash item")
	if exists(filepath.Join(library, "switch", "Limbo", "Limbo [UPDATE v2].nsp")) {
		t.Fatal("trashed file still in the game folder")
	}
	trash := trashList(t, srv, cookie)
	if len(trash) != 1 || trash[0].Kind != "game" || trash[0].Title != "Limbo" || trash[0].WholeGame ||
		trash[0].ExpiresAt.Sub(trash[0].TrashedAt) != 30*24*time.Hour {
		t.Fatalf("trash = %+v", trash)
	}
	wantStatus(t, post(t, srv, cookie, "/api/trash/"+id(trash[0].ID)+"/restore", ""), http.StatusOK, "restore")
	if !exists(filepath.Join(library, "switch", "Limbo", "Limbo [UPDATE v2].nsp")) {
		t.Fatal("restored file missing")
	}

	// The whole game: its folder and the console folder go when empty (RF-25).
	wantStatus(t, post(t, srv, cookie, "/api/games/"+id(game)+"/trash", ""), http.StatusNoContent, "trash game")
	if exists(filepath.Join(library, "switch")) {
		t.Fatal("empty console folder left behind")
	}
	if _, status := gameOf(t, srv, cookie, game); status != http.StatusNotFound {
		t.Fatalf("trashed game = %d, want 404", status)
	}
	trash = trashList(t, srv, cookie)
	if len(trash) != 1 || !trash[0].WholeGame || len(trash[0].Items) != 2 {
		t.Fatalf("trash = %+v", trash)
	}

	// A copy stored meanwhile takes the place: restoring needs replace.
	store(t, srv, cookie, "switch", "Limbo", "b2.nsp", "new base", map[string]any{"kind": "base"})
	wantStatus(t, post(t, srv, cookie, "/api/trash/"+id(trash[0].ID)+"/restore", ""), http.StatusConflict, "restore into a taken place")
	wantStatus(t, post(t, srv, cookie, "/api/trash/"+id(trash[0].ID)+"/restore", `{"onConflict":"replace"}`), http.StatusOK, "restore with replace")
	if got := readFile(t, filepath.Join(library, "switch", "Limbo", "Limbo [BASE].nsp")); got != "base" {
		t.Fatalf("restored base = %q", got)
	}

	// Deleted for good.
	trash = trashList(t, srv, cookie)
	wantStatus(t, call(t, srv, cookie, http.MethodDelete, "/api/trash/"+id(trash[0].ID), ""), http.StatusNoContent, "delete entry")
	wantStatus(t, call(t, srv, cookie, http.MethodDelete, "/api/trash", ""), http.StatusNoContent, "empty trash")
	if len(trashList(t, srv, cookie)) != 0 {
		t.Fatal("trash not empty")
	}
	if entries, _ := os.ReadDir(filepath.Join(library, ".gameexplorer", "trash")); len(entries) != 0 {
		t.Fatalf("trash files left = %v", entries)
	}
}

func TestEditRenamesMovesAndMerges(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	daxter := store(t, srv, cookie, "psp", "Daxter", "d.iso", "daxter", map[string]any{})

	// Rename to an IGDB game: folder and file follow; the game is linked.
	res := post(t, srv, cookie, "/api/games/"+id(daxter)+"/edit", `{"console":"psp","title":"x","igdbId":427}`)
	wantStatus(t, res, http.StatusOK, "rename")
	if !exists(filepath.Join(library, "psp", "Final Fantasy VII", "Final Fantasy VII.iso")) || exists(filepath.Join(library, "psp", "Daxter")) {
		t.Fatal("rename did not move folder and file")
	}
	g, _ := gameOf(t, srv, cookie, daxter)
	if g.Title != "Final Fantasy VII" || g.IgdbID == nil || *g.IgdbID != 427 {
		t.Fatalf("game = %+v", g)
	}

	// Move to Wii (.iso is valid there); a name of the user's own unlinks it.
	res = post(t, srv, cookie, "/api/games/"+id(daxter)+"/edit", `{"console":"wii","title":"Daxter"}`)
	wantStatus(t, res, http.StatusOK, "move")
	if !exists(filepath.Join(library, "wii", "Daxter", "Daxter.iso")) || exists(filepath.Join(library, "psp")) {
		t.Fatal("move did not land in wii/ (and psp/ must go when empty)")
	}
	if g, _ = gameOf(t, srv, cookie, daxter); g.Console != "wii" || g.IgdbID != nil {
		t.Fatalf("game = %+v", g)
	}

	// A Switch game cannot move to a console that does not take .nsp.
	limbo := store(t, srv, cookie, "switch", "Limbo", "l.nsp", "limbo", map[string]any{"kind": "base"})
	wantStatus(t, post(t, srv, cookie, "/api/games/"+id(limbo)+"/edit", `{"console":"wii","title":"Limbo"}`), http.StatusBadRequest, "move nsp to wii")

	// Renaming onto another game of the console merges; collisions need decisions.
	other := store(t, srv, cookie, "wii", "Jak", "j.iso", "jak", map[string]any{})
	plan := decode[struct {
		MergeInto *int64 `json:"mergeInto"`
		Items     []struct {
			Action string   `json:"action"`
			Item   itemJSON `json:"item"`
		} `json:"items"`
	}](t, post(t, srv, cookie, "/api/games/"+id(other)+"/edit/plan", `{"console":"wii","title":"daxter"}`))
	if plan.MergeInto == nil || *plan.MergeInto != daxter || plan.Items[0].Action != "undecided" {
		t.Fatalf("plan = %+v", plan)
	}
	wantStatus(t, post(t, srv, cookie, "/api/games/"+id(other)+"/edit", `{"console":"wii","title":"daxter"}`), http.StatusConflict, "undecided merge")
	body := jsonBody(t, map[string]any{"console": "wii", "title": "daxter", "decisions": []map[string]any{{"itemId": plan.Items[0].Item.ID, "onDuplicate": "replace"}}})
	res = post(t, srv, cookie, "/api/games/"+id(other)+"/edit", body)
	wantStatus(t, res, http.StatusOK, "merge")
	if got := readFile(t, filepath.Join(library, "wii", "Daxter", "Daxter.iso")); got != "jak" {
		t.Fatalf("merged file = %q", got)
	}
	if _, status := gameOf(t, srv, cookie, other); status != http.StatusNotFound {
		t.Fatal("merged game must be gone")
	}
}

func TestEditAFileOfSwitch(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, "switch", "Limbo", "u.nsp", "update", map[string]any{"kind": "update", "label": "1.0"})
	store(t, srv, cookie, "switch", "Limbo", "u2.nsp", "update2", map[string]any{"kind": "update", "label": "2.0"})
	g, _ := gameOf(t, srv, cookie, game)

	res := call(t, srv, cookie, http.MethodPatch, "/api/items/"+id(g.Items[0].ID), `{"kind":"dlc","label":"Fuga Maestra"}`)
	wantStatus(t, res, http.StatusOK, "edit item")
	if !exists(filepath.Join(library, "switch", "Limbo", "Limbo [DLC Fuga Maestra].nsp")) {
		t.Fatal("file not renamed")
	}
	// Becoming the other update collides: replace sends that one to the trash.
	g, _ = gameOf(t, srv, cookie, game)
	dlc := g.Items[len(g.Items)-1].ID
	wantStatus(t, call(t, srv, cookie, http.MethodPatch, "/api/items/"+id(dlc), `{"kind":"update","label":"2.0"}`), http.StatusConflict, "collision")
	wantStatus(t, call(t, srv, cookie, http.MethodPatch, "/api/items/"+id(dlc), `{"kind":"update","label":"v2.0","onDuplicate":"replace"}`), http.StatusOK, "replace")
	if got := readFile(t, filepath.Join(library, "switch", "Limbo", "Limbo [UPDATE v2.0].nsp")); got != "update" {
		t.Fatalf("file = %q", got)
	}
	if len(trashList(t, srv, cookie)) != 1 {
		t.Fatal("replaced file must be in the trash")
	}
	wantStatus(t, call(t, srv, cookie, http.MethodPatch, "/api/items/"+id(dlc), `{"kind":"game"}`), http.StatusBadRequest, "kind of another console")
}

func TestUnassignedSection(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, "psp", "Daxter", "d.iso", "daxter", map[string]any{})

	// A game moved to the section keeps its place as a path.
	wantStatus(t, post(t, srv, cookie, "/api/games/"+id(game)+"/unassign", ""), http.StatusNoContent, "unassign")
	files := unassignedList(t, srv, cookie)
	if len(files) != 1 || files[0].Path != "psp/Daxter/Daxter.iso" || files[0].Reason != "manual" ||
		len(files[0].Consoles) != 2 || files[0].Archive {
		t.Fatalf("unassigned = %+v", files)
	}
	if exists(filepath.Join(library, "psp")) {
		t.Fatal("empty folders left behind")
	}
	res := get(t, srv, cookie, "/api/unassigned/"+id(files[0].ID)+"/download")
	if res.StatusCode != http.StatusOK || body(t, res) != "daxter" {
		t.Fatalf("download = %d", res.StatusCode)
	}

	// Assigning it starts a job; it lands in Wii with a new name.
	res = post(t, srv, cookie, "/api/unassigned/"+id(files[0].ID)+"/assign", `{"console":"wii","title":"Jak"}`)
	wantStatus(t, res, http.StatusCreated, "assign")
	job := decode[jobJSON](t, res)
	if !job.FromUnassigned {
		t.Fatalf("job = %+v", job)
	}
	waitStatus(t, srv, cookie, job.ID, "confirm")
	if len(unassignedList(t, srv, cookie)) != 0 {
		t.Fatal("assigned file must leave the section")
	}
	// Cancelling gives it back.
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job.ID+"/cancel", ""), http.StatusOK, "cancel")
	files = unassignedList(t, srv, cookie)
	if len(files) != 1 || files[0].Path != "psp/Daxter/Daxter.iso" {
		t.Fatalf("unassigned after cancel = %+v", files)
	}

	// Assign again and store it.
	res = post(t, srv, cookie, "/api/unassigned/"+id(files[0].ID)+"/assign", `{"console":"wii","title":"Jak"}`)
	job = decode[jobJSON](t, res)
	waitStatus(t, srv, cookie, job.ID, "confirm")
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job.ID+"/commit", commitBody(t, map[string]any{"path": "Daxter.iso"})), http.StatusOK, "commit")
	if got := readFile(t, filepath.Join(library, "wii", "Jak", "Jak.iso")); got != "daxter" {
		t.Fatalf("stored = %q", got)
	}

	// Trash and delete.
	writeFile(t, filepath.Join(library, "_unassigned", "a.z64"), "n64")
	writeFile(t, filepath.Join(library, "_unassigned", "b.rar"), "rar")
	wantStatus(t, post(t, srv, cookie, "/api/library/scan", ""), http.StatusOK, "scan")
	files = unassignedList(t, srv, cookie)
	if len(files) != 2 {
		t.Fatalf("unassigned = %+v", files)
	}
	for _, f := range files {
		switch f.Name {
		case "a.z64":
			if len(f.Consoles) != 0 || f.Archive {
				t.Errorf("a.z64 = %+v", f)
			}
			wantStatus(t, post(t, srv, cookie, "/api/unassigned/"+id(f.ID)+"/trash", ""), http.StatusNoContent, "trash")
		case "b.rar":
			if !f.Archive {
				t.Errorf("b.rar = %+v", f)
			}
			wantStatus(t, call(t, srv, cookie, http.MethodDelete, "/api/unassigned/"+id(f.ID), ""), http.StatusNoContent, "delete")
		}
	}
	if len(unassignedList(t, srv, cookie)) != 0 || exists(filepath.Join(library, "_unassigned", "b.rar")) {
		t.Fatal("section not empty")
	}
	if trash := trashList(t, srv, cookie); len(trash) != 1 || trash[0].Files[0].Name != "a.z64" {
		t.Fatalf("trash = %+v", trash)
	}
}

func TestScanFindsChangesMadeOverSMB(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	keep := store(t, srv, cookie, "switch", "Limbo", "l.nsp", "base", map[string]any{"kind": "base"})
	gone := store(t, srv, cookie, "psp", "Daxter", "d.iso", "daxter", map[string]any{})

	// Deleted over SMB: the game leaves the library.
	if err := os.Remove(filepath.Join(library, "psp", "Daxter", "Daxter.iso")); err != nil {
		t.Fatal(err)
	}
	// Unknown files: one in a game folder, one in the root, one in an unknown
	// folder and junk that is ignored.
	writeFile(t, filepath.Join(library, "switch", "Limbo", "extra.nsp"), "extra")
	writeFile(t, filepath.Join(library, "readme.txt"), "hi")
	writeFile(t, filepath.Join(library, "n64", "Mario.z64"), "mario")
	writeFile(t, filepath.Join(library, "switch", ".DS_Store"), "junk")

	type report struct{ Unassigned, Removed, Pending int }
	scan := func() map[string]int {
		res := post(t, srv, cookie, "/api/library/scan", "")
		wantStatus(t, res, http.StatusOK, "scan")
		r := decode[report](t, res)
		return map[string]int{"unassigned": r.Unassigned, "removed": r.Removed, "pending": r.Pending}
	}
	first := scan()
	if first["removed"] != 1 || first["pending"] != 3 || first["unassigned"] != 0 {
		t.Fatalf("first scan = %+v (unknown files wait for a second look)", first)
	}
	if _, status := gameOf(t, srv, cookie, gone); status != http.StatusNotFound {
		t.Fatal("game deleted over SMB must be gone")
	}

	// A file still being copied changes between scans and waits again.
	writeFile(t, filepath.Join(library, "n64", "Mario.z64"), "mario, bigger now")
	second := scan()
	if second["unassigned"] != 2 || second["pending"] != 1 {
		t.Fatalf("second scan = %+v", second)
	}
	if !exists(filepath.Join(library, "_unassigned", "switch", "Limbo", "extra.nsp")) || !exists(filepath.Join(library, "_unassigned", "readme.txt")) {
		t.Fatal("unknown files must move to _unassigned keeping their path")
	}
	if !exists(filepath.Join(library, "switch", ".DS_Store")) {
		t.Fatal("junk must be left alone")
	}
	third := scan()
	if third["unassigned"] != 1 || exists(filepath.Join(library, "n64")) {
		t.Fatalf("third scan = %+v (the n64 folder goes when empty)", third)
	}
	if g, _ := gameOf(t, srv, cookie, keep); len(g.Items) != 1 {
		t.Fatalf("known game = %+v", g)
	}
	status := decode[struct {
		LastScan *struct {
			Unassigned int `json:"unassigned"`
		} `json:"lastScan"`
	}](t, get(t, srv, cookie, "/api/library/scan"))
	if status.LastScan == nil || status.LastScan.Unassigned != 1 {
		t.Fatalf("last scan = %+v", status)
	}
	files := unassignedList(t, srv, cookie)
	if len(files) != 3 {
		t.Fatalf("unassigned = %d, want 3", len(files))
	}

	// An assignment that is cancelled gives the file back as it arrived.
	var extra unassignedJSON
	for _, f := range files {
		if f.Name == "extra.nsp" {
			extra = f
		}
	}
	if extra.Reason != "samba" {
		t.Fatalf("extra.nsp = %+v", extra)
	}
	res := post(t, srv, cookie, "/api/unassigned/"+id(extra.ID)+"/assign", `{"console":"switch","title":"Extra"}`)
	wantStatus(t, res, http.StatusCreated, "assign")
	job := decode[jobJSON](t, res)
	waitStatus(t, srv, cookie, job.ID, "confirm")
	wantStatus(t, post(t, srv, cookie, "/api/jobs/"+job.ID+"/cancel", ""), http.StatusOK, "cancel")
	for _, f := range unassignedList(t, srv, cookie) {
		if f.Name == "extra.nsp" && (f.Reason != extra.Reason || f.Origin != extra.Origin || f.Path != extra.Path) {
			t.Fatalf("given back as %+v, want %+v", f, extra)
		}
	}
}

func TestConsoleSettings(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)

	type consoleJSON struct {
		Slug             string   `json:"slug"`
		DisplayName      string   `json:"displayName"`
		DefaultName      string   `json:"defaultName"`
		Extensions       []string `json:"extensions"`
		CustomExtensions []struct {
			Extension string `json:"extension"`
			FileCount int    `json:"fileCount"`
		} `json:"customExtensions"`
	}
	res := call(t, srv, cookie, http.MethodPatch, "/api/consoles/switch", `{"displayName":" Switch "}`)
	wantStatus(t, res, http.StatusOK, "rename")
	if c := decode[consoleJSON](t, res); c.DisplayName != "Switch" || c.DefaultName != "Nintendo Switch" {
		t.Fatalf("console = %+v", c)
	}
	res = call(t, srv, cookie, http.MethodPut, "/api/consoles/order", `{"slugs":["psp","switch","wii"]}`)
	wantStatus(t, res, http.StatusOK, "reorder")
	if list := decode[[]consoleJSON](t, res); list[0].Slug != "psp" || list[1].DisplayName != "Switch" {
		t.Fatalf("consoles = %+v", list)
	}
	wantStatus(t, call(t, srv, cookie, http.MethodPut, "/api/consoles/order", `{"slugs":["psp"]}`), http.StatusBadRequest, "incomplete order")

	wantStatus(t, post(t, srv, cookie, "/api/consoles/switch/extensions", `{"extension":"XCZ"}`), http.StatusOK, "add extension")
	wantStatus(t, post(t, srv, cookie, "/api/consoles/switch/extensions", `{"extension":".xcz"}`), http.StatusConflict, "add twice")
	wantStatus(t, post(t, srv, cookie, "/api/consoles/switch/extensions", `{"extension":"x y"}`), http.StatusBadRequest, "bad extension")
	ext := url.QueryEscape(".nsp")
	wantStatus(t, call(t, srv, cookie, http.MethodDelete, "/api/consoles/switch/extensions?extension="+ext, ""), http.StatusConflict, "remove fixed")

	// A file with the custom extension keeps it from being removed.
	store(t, srv, cookie, "switch", "Limbo", "l.xcz", "xcz", map[string]any{"kind": "base"})
	wantStatus(t, call(t, srv, cookie, http.MethodDelete, "/api/consoles/switch/extensions?extension=.xcz", ""), http.StatusConflict, "remove in use")
	list := decode[[]consoleJSON](t, get(t, srv, cookie, "/api/consoles"))
	for _, c := range list {
		if c.Slug == "switch" && (len(c.CustomExtensions) != 1 || c.CustomExtensions[0].FileCount != 1) {
			t.Fatalf("switch = %+v", c)
		}
	}
}
