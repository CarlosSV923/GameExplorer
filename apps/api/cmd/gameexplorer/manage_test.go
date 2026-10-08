package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

type trashJSON struct {
	ID        int64     `json:"id"`
	GameID    int64     `json:"gameId"`
	Title     string    `json:"title"`
	WholeGame bool      `json:"wholeGame"`
	Reason    string    `json:"reason"`
	TrashedAt time.Time `json:"trashedAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Size      int64     `json:"size"`
	Items     []struct {
		ID    int64    `json:"id"`
		Files []string `json:"files"`
	} `json:"items"`
}

type itemDetailJSON struct {
	ID           int64      `json:"id"`
	Kind         string     `json:"kind"`
	Files        []string   `json:"files"`
	MissingSince *time.Time `json:"missingSince"`
}

type gameJSON struct {
	ID           int64            `json:"id"`
	Title        string           `json:"title"`
	Path         string           `json:"path"`
	MissingCount int              `json:"missingCount"`
	Items        []itemDetailJSON `json:"items"`
}

func gameOf(t *testing.T, srv *httptest.Server, id int64) (gameJSON, int) {
	t.Helper()
	cookie := login(t, srv)
	res := get(t, srv, cookie, "/api/games/"+strconv.FormatInt(id, 10))
	if res.StatusCode != http.StatusOK {
		return gameJSON{}, res.StatusCode
	}
	return decode[gameJSON](t, res), res.StatusCode
}

func trashList(t *testing.T, srv *httptest.Server, cookie *http.Cookie) []trashJSON {
	t.Helper()
	return decode[[]trashJSON](t, get(t, srv, cookie, "/api/trash"))
}

func call(t *testing.T, srv *httptest.Server, cookie *http.Cookie, method, path, body string) *http.Response {
	t.Helper()
	return do(t, method, srv.URL+path, body, cookie)
}

func id(n int64) string { return strconv.FormatInt(n, 10) }

func TestTrashAnItemAndRestoreIt(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, 26764, "mk8.nsp", "base", map[string]any{"kind": "base"})
	store(t, srv, cookie, 26764, "mk8u.nsp", "update", map[string]any{"kind": "update", "label": "v3.0.1"})
	dir := filepath.Join(library, "switch", "Mario Kart 8 Deluxe")
	g, _ := gameOf(t, srv, game)
	update := g.Items[1]

	if res := call(t, srv, cookie, http.MethodPost, "/api/items/"+id(update.ID)+"/trash", ""); res.StatusCode != http.StatusNoContent {
		t.Fatalf("trash item = %d %s", res.StatusCode, problemDetail(t, res))
	}
	if _, err := os.Stat(filepath.Join(dir, update.Files[0])); !os.IsNotExist(err) {
		t.Fatalf("trashed file still in the game folder: %v", err)
	}
	if g, _ := gameOf(t, srv, game); len(g.Items) != 1 || g.Items[0].Kind != "base" {
		t.Fatalf("detail after trash = %+v", g.Items)
	}
	entries := trashList(t, srv, cookie)
	if len(entries) != 1 || entries[0].Reason != "deleted" || entries[0].WholeGame || entries[0].Size != int64(len("update")) ||
		entries[0].ExpiresAt.Sub(entries[0].TrashedAt) != 30*24*time.Hour || entries[0].Title != "Mario Kart 8 Deluxe" {
		t.Fatalf("trash = %+v", entries)
	}
	if res := call(t, srv, cookie, http.MethodPost, "/api/items/"+id(update.ID)+"/trash", ""); res.StatusCode != http.StatusNotFound {
		t.Fatalf("trashing twice = %d, want 404", res.StatusCode)
	}

	res := call(t, srv, cookie, http.MethodPost, "/api/trash/"+id(entries[0].ID)+"/restore", "")
	if res.StatusCode != http.StatusOK {
		t.Fatalf("restore = %d %s", res.StatusCode, problemDetail(t, res))
	}
	if got := readFile(t, filepath.Join(dir, update.Files[0])); got != "update" {
		t.Fatalf("restored file = %q", got)
	}
	if len(trashList(t, srv, cookie)) != 0 {
		t.Fatal("the entry must leave the trash")
	}
	if names, _ := os.ReadDir(trashDir(library)); len(names) != 0 {
		t.Fatalf("trash folder not cleaned: %v", names)
	}
}

func TestTrashAWholeGame(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, 26764, "mk8.nsp", "base", map[string]any{"kind": "base"})
	store(t, srv, cookie, 26764, "mk8u.nsp", "update", map[string]any{"kind": "update", "label": "v3.0.1"})
	dir := filepath.Join(library, "switch", "Mario Kart 8 Deluxe")

	if res := call(t, srv, cookie, http.MethodPost, "/api/games/"+id(game)+"/trash", ""); res.StatusCode != http.StatusNoContent {
		t.Fatalf("trash game = %d %s", res.StatusCode, problemDetail(t, res))
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("empty game folder must go: %v", err)
	}
	if _, status := gameOf(t, srv, game); status != http.StatusNotFound {
		t.Fatalf("trashed game detail = %d, want 404", status)
	}
	if games := decode[[]gameSummaryJSON](t, get(t, srv, cookie, "/api/consoles/switch/games")); len(games) != 0 {
		t.Fatalf("switch games = %+v", games)
	}
	entries := trashList(t, srv, cookie)
	if len(entries) != 1 || !entries[0].WholeGame || len(entries[0].Items) != 2 {
		t.Fatalf("trash = %+v", entries)
	}

	if res := call(t, srv, cookie, http.MethodPost, "/api/trash/"+id(entries[0].ID)+"/restore", "{}"); res.StatusCode != http.StatusOK {
		t.Fatalf("restore = %d %s", res.StatusCode, problemDetail(t, res))
	}
	if g, _ := gameOf(t, srv, game); len(g.Items) != 2 {
		t.Fatalf("restored game = %+v", g)
	}
	if readFile(t, filepath.Join(dir, "Mario Kart 8 Deluxe.nsp")) != "base" {
		t.Fatal("base not restored")
	}
}

func TestRestoreIntoAnOccupiedPlaceNeedsReplace(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, 26764, "first.nsp", "first", map[string]any{"kind": "base"})
	g, _ := gameOf(t, srv, game)
	call(t, srv, cookie, http.MethodPost, "/api/items/"+id(g.Items[0].ID)+"/trash", "")
	// A new base game is not a duplicate of the trashed one.
	store(t, srv, cookie, 26764, "second.nsp", "second", map[string]any{"kind": "base"})
	entry := trashList(t, srv, cookie)[0]

	res := call(t, srv, cookie, http.MethodPost, "/api/trash/"+id(entry.ID)+"/restore", "")
	if detail := problemDetail(t, res); res.StatusCode != http.StatusConflict || !strings.Contains(detail, "Mario Kart 8 Deluxe.nsp") {
		t.Fatalf("restore over a duplicate = %d %s", res.StatusCode, detail)
	}
	res = call(t, srv, cookie, http.MethodPost, "/api/trash/"+id(entry.ID)+"/restore", `{"onConflict":"replace"}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("restore with replace = %d %s", res.StatusCode, problemDetail(t, res))
	}
	if got := readFile(t, filepath.Join(library, "switch", "Mario Kart 8 Deluxe", "Mario Kart 8 Deluxe.nsp")); got != "first" {
		t.Fatalf("library has %q, want the restored item", got)
	}
	entries := trashList(t, srv, cookie)
	if len(entries) != 1 || entries[0].Reason != "replaced" || entries[0].Size != int64(len("second")) {
		t.Fatalf("trash = %+v, want the swapped-out item", entries)
	}
}

func TestDeleteFromTheTrashForGood(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	mk8 := store(t, srv, cookie, 26764, "mk8.nsp", "base", map[string]any{"kind": "base"})
	totk := store(t, srv, cookie, 119388, "totk.nsp", "zelda", map[string]any{"kind": "base"})
	call(t, srv, cookie, http.MethodPost, "/api/games/"+id(mk8)+"/trash", "")
	call(t, srv, cookie, http.MethodPost, "/api/games/"+id(totk)+"/trash", "")
	entries := trashList(t, srv, cookie)
	if len(entries) != 2 {
		t.Fatalf("trash = %+v", entries)
	}

	if res := call(t, srv, cookie, http.MethodDelete, "/api/trash/"+id(entries[0].ID), ""); res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete entry = %d", res.StatusCode)
	}
	if res := call(t, srv, cookie, http.MethodDelete, "/api/trash/"+id(entries[0].ID), ""); res.StatusCode != http.StatusNotFound {
		t.Fatalf("delete twice = %d, want 404", res.StatusCode)
	}
	if res := call(t, srv, cookie, http.MethodDelete, "/api/trash", ""); res.StatusCode != http.StatusNoContent {
		t.Fatalf("empty trash = %d", res.StatusCode)
	}
	if len(trashList(t, srv, cookie)) != 0 {
		t.Fatal("trash not empty")
	}
	if names, _ := os.ReadDir(trashDir(library)); len(names) != 0 {
		t.Fatalf("trash files left: %v", names)
	}
	// The games had nothing else: they are gone, and could be stored again.
	store(t, srv, cookie, 26764, "mk8.nsp", "again", map[string]any{"kind": "base"})
}

func TestRematchRenamesFolderAndFiles(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, 26764, "mk8.nsp", "base", map[string]any{"kind": "base"})
	store(t, srv, cookie, 26764, "mk8u.nsp", "update", map[string]any{"kind": "update", "label": "v3.0.1"})
	oldDir := filepath.Join(library, "switch", "Mario Kart 8 Deluxe")
	// A file added over SMB travels with the folder.
	if err := os.WriteFile(filepath.Join(oldDir, "notes.txt"), []byte("mine"), 0o600); err != nil {
		t.Fatal(err)
	}

	plan := decode[struct {
		Folder    string `json:"folder"`
		MergeInto *int64 `json:"mergeInto"`
		Items     []struct {
			Files  []string `json:"files"`
			Action string   `json:"action"`
		} `json:"items"`
	}](t, call(t, srv, cookie, http.MethodPost, "/api/games/"+id(game)+"/rematch/plan", `{"igdbGameId":119388}`))
	if plan.Folder != "The Legend of Zelda - Tears of the Kingdom" || plan.MergeInto != nil || len(plan.Items) != 2 ||
		plan.Items[1].Files[0] != "The Legend of Zelda - Tears of the Kingdom [Update v3.0.1].nsp" {
		t.Fatalf("plan = %+v", plan)
	}

	res := call(t, srv, cookie, http.MethodPost, "/api/games/"+id(game)+"/rematch", `{"igdbGameId":119388}`)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("rematch = %d %s", res.StatusCode, problemDetail(t, res))
	}
	newDir := filepath.Join(library, "switch", "The Legend of Zelda - Tears of the Kingdom")
	entries, _ := os.ReadDir(newDir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{"The Legend of Zelda - Tears of the Kingdom [Update v3.0.1].nsp", "The Legend of Zelda - Tears of the Kingdom.nsp", "notes.txt"}
	if !slices.Equal(names, want) {
		t.Fatalf("files = %q, want %q", names, want)
	}
	if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
		t.Fatalf("old folder still there: %v", err)
	}
	if g, _ := gameOf(t, srv, game); g.Title != "The Legend of Zelda: Tears of the Kingdom" || g.Path != "switch/The Legend of Zelda - Tears of the Kingdom" {
		t.Fatalf("game = %+v", g)
	}
}

func TestRematchIntoAGameInTheLibraryMerges(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	zelda := store(t, srv, cookie, 119388, "totk.nsp", "zelda base", map[string]any{"kind": "base"})
	mk8 := store(t, srv, cookie, 26764, "mk8.nsp", "wrong base", map[string]any{"kind": "base"})
	store(t, srv, cookie, 26764, "mk8u.nsp", "update", map[string]any{"kind": "update", "label": "v1.1.0"})
	g, _ := gameOf(t, srv, mk8)
	base := g.Items[0]

	plan := decode[struct {
		MergeInto *int64 `json:"mergeInto"`
		Items     []struct {
			Action string `json:"action"`
		} `json:"items"`
	}](t, call(t, srv, cookie, http.MethodPost, "/api/games/"+id(mk8)+"/rematch/plan", `{"igdbGameId":119388}`))
	if plan.MergeInto == nil || *plan.MergeInto != zelda || plan.Items[0].Action != "undecided" || plan.Items[1].Action != "store" {
		t.Fatalf("plan = %+v", plan)
	}
	if res := call(t, srv, cookie, http.MethodPost, "/api/games/"+id(mk8)+"/rematch", `{"igdbGameId":119388}`); res.StatusCode != http.StatusConflict {
		t.Fatalf("undecided merge = %d, want 409", res.StatusCode)
	}

	body := `{"igdbGameId":119388,"decisions":[{"itemId":` + id(base.ID) + `,"onDuplicate":"skip"}]}`
	res := decode[struct {
		GameID int64 `json:"gameId"`
		Merged bool  `json:"merged"`
	}](t, call(t, srv, cookie, http.MethodPost, "/api/games/"+id(mk8)+"/rematch", body))
	if res.GameID != zelda || !res.Merged {
		t.Fatalf("rematch = %+v", res)
	}
	dir := filepath.Join(library, "switch", "The Legend of Zelda - Tears of the Kingdom")
	if readFile(t, filepath.Join(dir, "The Legend of Zelda - Tears of the Kingdom.nsp")) != "zelda base" ||
		readFile(t, filepath.Join(dir, "The Legend of Zelda - Tears of the Kingdom [Update v1.1.0].nsp")) != "update" {
		t.Fatal("merged files are wrong")
	}
	if _, status := gameOf(t, srv, mk8); status != http.StatusNotFound {
		t.Fatalf("merged game still exists: %d", status)
	}
	if _, err := os.Stat(filepath.Join(library, "switch", "Mario Kart 8 Deluxe")); !os.IsNotExist(err) {
		t.Fatalf("old folder still there: %v", err)
	}
	entries := trashList(t, srv, cookie)
	if len(entries) != 1 || entries[0].GameID != zelda || entries[0].Size != int64(len("wrong base")) {
		t.Fatalf("trash = %+v, want the skipped base under the merged game", entries)
	}
}

func TestRematchRewritesDiscSheets(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv, cookie, library := libraryServer(t)
	jobID := upload(t, srv, cookie, "ff7.zip", zipOf(t, map[string]string{
		"ff7.cue": "FILE \"a.bin\" BINARY\nFILE \"b.bin\" AUDIO\n", "a.bin": "data", "b.bin": "audio",
	}))
	waitStatus(t, srv, cookie, jobID, "review")
	game := decode[commitJSON](t, post(t, srv.URL, cookie, jobID, "commit",
		commitBody(t, "psx", 427, map[string]any{"path": "ff7.cue", "kind": "disc", "discNumber": 1}))).GameID

	if res := call(t, srv, cookie, http.MethodPost, "/api/games/"+id(game)+"/rematch", `{"igdbGameId":26764}`); res.StatusCode != http.StatusOK {
		t.Fatalf("rematch = %d %s", res.StatusCode, problemDetail(t, res))
	}
	dir := filepath.Join(library, "psx", "Mario Kart 8 Deluxe")
	want := "FILE \"Mario Kart 8 Deluxe (Disc 1) (Track 1).bin\" BINARY\nFILE \"Mario Kart 8 Deluxe (Disc 1) (Track 2).bin\" AUDIO\n"
	if got := readFile(t, filepath.Join(dir, "Mario Kart 8 Deluxe (Disc 1).cue")); got != want {
		t.Fatalf("cue =\n%s", got)
	}
	if readFile(t, filepath.Join(dir, "Mario Kart 8 Deluxe (Disc 1) (Track 2).bin")) != "audio" {
		t.Fatal("track 2 not renamed")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 3 {
		t.Fatalf("leftovers in the game folder: %v", entries)
	}
}

func TestIntegrityCheckMarksMissingItems(t *testing.T) {
	t.Parallel()
	srv, cookie, library := libraryServer(t)
	game := store(t, srv, cookie, 26764, "mk8.nsp", "base", map[string]any{"kind": "base"})
	store(t, srv, cookie, 26764, "mk8u.nsp", "update", map[string]any{"kind": "update", "label": "v3.0.1"})
	dir := filepath.Join(library, "switch", "Mario Kart 8 Deluxe")
	g, _ := gameOf(t, srv, game)
	base, update := g.Items[0], g.Items[1]

	check := func() struct{ Checked, Missing, Found int } {
		t.Helper()
		return decode[struct{ Checked, Missing, Found int }](t, call(t, srv, cookie, http.MethodPost, "/api/library/check", ""))
	}
	last := func() *struct {
		CheckedAt    time.Time
		MissingTotal int
	} {
		t.Helper()
		return decode[struct {
			LastCheck *struct {
				CheckedAt    time.Time
				MissingTotal int
			}
		}](t, get(t, srv, cookie, "/api/library/check")).LastCheck
	}
	if rep := check(); rep.Checked != 2 || rep.Missing != 0 {
		t.Fatalf("clean check = %+v", rep)
	}
	if l := last(); l == nil || l.CheckedAt.IsZero() || l.MissingTotal != 0 {
		t.Fatalf("last check = %+v", l)
	}

	// Deleted over SMB.
	_ = os.Remove(filepath.Join(dir, update.Files[0]))
	_ = os.Rename(filepath.Join(dir, base.Files[0]), filepath.Join(library, "elsewhere.nsp"))
	if rep := check(); rep.Missing != 2 {
		t.Fatalf("check = %+v", rep)
	}
	if g, _ := gameOf(t, srv, game); g.MissingCount != 2 || g.Items[1].MissingSince == nil {
		t.Fatalf("detail = %+v", g)
	}

	// The base comes back; the update is forgotten.
	_ = os.Rename(filepath.Join(library, "elsewhere.nsp"), filepath.Join(dir, base.Files[0]))
	if rep := check(); rep.Found != 1 {
		t.Fatalf("check after restore = %+v", rep)
	}
	if l := last(); l == nil || l.MissingTotal != 1 {
		t.Fatalf("last check = %+v, want the update still missing", l)
	}
	if res := call(t, srv, cookie, http.MethodPost, "/api/items/"+id(base.ID)+"/forget", ""); res.StatusCode != http.StatusConflict {
		t.Fatalf("forget a present item = %d, want 409", res.StatusCode)
	}
	if res := call(t, srv, cookie, http.MethodPost, "/api/items/"+id(update.ID)+"/forget", ""); res.StatusCode != http.StatusNoContent {
		t.Fatalf("forget = %d %s", res.StatusCode, problemDetail(t, res))
	}
	if g, _ := gameOf(t, srv, game); len(g.Items) != 1 || g.MissingCount != 0 {
		t.Fatalf("detail after forget = %+v", g)
	}
}

type consoleJSON struct {
	ID          int64    `json:"id"`
	Slug        string   `json:"slug"`
	DisplayName string   `json:"displayName"`
	Extensions  []string `json:"extensions"`
	LogoImageID *string  `json:"logoImageId"`
	ReleaseYear *int     `json:"releaseYear"`
	SortOrder   int      `json:"sortOrder"`
	BuiltIn     bool     `json:"builtIn"`
	Detection   string   `json:"detection"`
}

func TestConsoleManagement(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)

	res := call(t, srv, cookie, http.MethodPost, "/api/consoles",
		`{"igdbPlatformId":4,"slug":"n64","displayName":"  Nintendo   64 ","extensions":[".z64","N64",".z64"]}`)
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("create = %d %s", res.StatusCode, problemDetail(t, res))
	}
	n64 := decode[consoleJSON](t, res)
	if n64.DisplayName != "Nintendo 64" || !slices.Equal(n64.Extensions, []string{".z64", ".n64"}) ||
		n64.LogoImageID == nil || *n64.LogoImageID != "pl6n" || n64.ReleaseYear == nil || n64.SortOrder <= 60 || // last in the carousel
		n64.BuiltIn || n64.Detection != "extension" {
		t.Fatalf("n64 = %+v", n64)
	}
	for name, body := range map[string]string{
		"taken slug":       `{"igdbPlatformId":4,"slug":"psx","displayName":"X","extensions":[".x"]}`,
		"taken platform":   `{"igdbPlatformId":4,"slug":"n64b","displayName":"X","extensions":[".x"]}`,
		"bad slug":         `{"igdbPlatformId":4,"slug":"N 64","displayName":"X","extensions":[".x"]}`,
		"no extensions":    `{"igdbPlatformId":4,"slug":"n64c","displayName":"X","extensions":[]}`,
		"unknown platform": `{"igdbPlatformId":999,"slug":"n64d","displayName":"X","extensions":[".x"]}`,
	} {
		res := call(t, srv, cookie, http.MethodPost, "/api/consoles", body)
		if want := map[bool]int{true: http.StatusConflict, false: http.StatusBadRequest}[strings.HasPrefix(name, "taken")]; res.StatusCode != want {
			t.Errorf("%s = %d, want %d", name, res.StatusCode, want)
		}
	}

	// A file with the new console's extension is detected as n64.
	jobID := upload(t, srv, cookie, "Mario Kart 64.z64", []byte("\x80\x37\x12\x40 rom"))
	waitStatus(t, srv, cookie, jobID, "review")
	if got := items(t, srv, cookie, jobID); len(got) != 1 || !slices.Equal(got[0].Consoles, []string{"n64"}) {
		t.Fatalf("detection = %+v", got)
	}
	res = post(t, srv.URL, cookie, jobID, "commit", commitBody(t, "n64", 26764, map[string]any{"path": "Mario Kart 64.z64", "kind": "base"}))
	game := decode[commitJSON](t, res).GameID

	path := "/api/consoles/" + id(n64.ID)
	if res := call(t, srv, cookie, http.MethodPatch, path, `{"slug":"nintendo64","displayName":"N64","extensions":[".z64"]}`); res.StatusCode != http.StatusConflict {
		t.Fatalf("slug change with games = %d, want 409", res.StatusCode)
	}
	if res := call(t, srv, cookie, http.MethodPatch, path, `{"slug":"n64","displayName":"N64","extensions":[".z64",".v64"]}`); res.StatusCode != http.StatusOK {
		t.Fatalf("rename = %d %s", res.StatusCode, problemDetail(t, res))
	}
	if res := call(t, srv, cookie, http.MethodDelete, path, ""); res.StatusCode != http.StatusConflict {
		t.Fatalf("delete with games = %d, want 409", res.StatusCode)
	}

	consoles := decode[[]consoleJSON](t, get(t, srv, cookie, "/api/consoles"))
	ids := []int64{n64.ID}
	for _, c := range consoles {
		if c.ID != n64.ID {
			ids = append(ids, c.ID)
		}
	}
	order := `{"ids":[` + strings.Join(func() []string {
		var s []string
		for _, i := range ids {
			s = append(s, id(i))
		}
		return s
	}(), ",") + `]}`
	if got := decode[[]consoleJSON](t, call(t, srv, cookie, http.MethodPut, "/api/consoles/order", order)); got[0].Slug != "n64" || got[1].Slug != "switch" {
		t.Fatalf("order = %+v", got)
	}
	if res := call(t, srv, cookie, http.MethodPut, "/api/consoles/order", `{"ids":[1,2]}`); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("partial order = %d, want 400", res.StatusCode)
	}

	// Once its game is gone for good, the console can be deleted.
	call(t, srv, cookie, http.MethodPost, "/api/games/"+id(game)+"/trash", "")
	call(t, srv, cookie, http.MethodDelete, "/api/trash", "")
	if res := call(t, srv, cookie, http.MethodDelete, path, ""); res.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d %s", res.StatusCode, problemDetail(t, res))
	}
	if res := call(t, srv, cookie, http.MethodDelete, "/api/consoles/1", ""); res.StatusCode != http.StatusConflict {
		t.Fatalf("delete built-in = %d, want 409", res.StatusCode)
	}
}
