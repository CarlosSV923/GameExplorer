package main

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

type gameSummaryJSON struct {
	ID        int64  `json:"id"`
	IgdbID    int64  `json:"igdbId"`
	Console   string `json:"console"`
	Title     string `json:"title"`
	Folder    string `json:"folder"`
	ItemCount int    `json:"itemCount"`
	Size      int64  `json:"size"`
}

type gameDetailJSON struct {
	gameSummaryJSON
	Path         string   `json:"path"`
	Genres       []string `json:"genres"`
	CoverImageID *string  `json:"coverImageId"`
	Items        []struct {
		ID    int64    `json:"id"`
		Kind  string   `json:"kind"`
		Shape string   `json:"shape"`
		Files []string `json:"files"`
		Size  int64    `json:"size"`
	} `json:"items"`
}

// libraryServer is a test server with the fake IGDB and its library path.
func libraryServer(t *testing.T) (*httptest.Server, *http.Cookie, string) {
	t.Helper()
	library := t.TempDir()
	srv := newTestServer(t, func(c *config.Config) {
		c.LibraryPath = library
		withFakeIGDB(fakeIGDBServer(t))(c)
	})
	return srv, login(t, srv), library
}

// store uploads a raw file and commits it as one Switch item; it returns the game id.
func store(t *testing.T, srv *httptest.Server, cookie *http.Cookie, game int64, name, content string, item map[string]any) int64 {
	t.Helper()
	id := upload(t, srv, cookie, name, []byte(content))
	waitStatus(t, srv, cookie, id, "review")
	item["path"] = name
	res := post(t, srv.URL, cookie, id, "commit", commitBody(t, "switch", game, item))
	if res.StatusCode != http.StatusOK {
		t.Fatalf("commit %s = %d %s", name, res.StatusCode, problemDetail(t, res))
	}
	return decode[commitJSON](t, res).GameID
}

func get(t *testing.T, srv *httptest.Server, cookie *http.Cookie, path string, headers ...string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(cookie)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = res.Body.Close() })
	return res
}

func TestLibraryListsConsolesGamesAndDetail(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)

	mk8 := store(t, srv, cookie, 26764, "mk8.nsp", "PFS0 base", map[string]any{"kind": "base"})
	store(t, srv, cookie, 26764, "mk8u.nsp", "PFS0 update!", map[string]any{"kind": "update", "label": "v3.0.1"})
	store(t, srv, cookie, 119388, "totk.nsp", "PFS0 zelda", map[string]any{"kind": "base"})

	consoles := decode[[]struct {
		Slug      string `json:"slug"`
		GameCount int    `json:"gameCount"`
		BuiltIn   bool   `json:"builtIn"`
		Detection string `json:"detection"`
	}](t, get(t, srv, cookie, "/api/consoles"))
	counts := map[string]int{}
	detections := map[string]string{}
	for _, c := range consoles {
		counts[c.Slug] = c.GameCount
		detections[c.Slug] = c.Detection
		if !c.BuiltIn {
			t.Errorf("%s is not built in", c.Slug)
		}
	}
	if counts["switch"] != 2 || counts["psx"] != 0 {
		t.Fatalf("game counts = %v", counts)
	}
	if detections["switch"] != "titleId" || detections["ps3"] != "structure" || detections["gc"] != "header" {
		t.Fatalf("detections = %v", detections)
	}

	games := decode[[]gameSummaryJSON](t, get(t, srv, cookie, "/api/consoles/switch/games"))
	if len(games) != 2 || games[0].Title != "Mario Kart 8 Deluxe" || games[0].IgdbID != 26764 || games[0].ItemCount != 2 ||
		games[0].Size != int64(len("PFS0 base")+len("PFS0 update!")) || games[1].Folder != "The Legend of Zelda - Tears of the Kingdom" {
		t.Fatalf("switch games = %+v", games)
	}
	if res := get(t, srv, cookie, "/api/consoles/psx/games"); res.StatusCode != http.StatusOK {
		t.Fatalf("empty console = %d", res.StatusCode)
	}
	if res := get(t, srv, cookie, "/api/consoles/n64/games"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown console = %d, want 404", res.StatusCode)
	}

	g := decode[gameDetailJSON](t, get(t, srv, cookie, "/api/games/"+strconv.FormatInt(mk8, 10)))
	if g.Path != "switch/Mario Kart 8 Deluxe" || !slices.Equal(g.Genres, []string{"Racing"}) || g.CoverImageID == nil ||
		len(g.Items) != 2 || g.Items[0].Kind != "base" || g.Items[1].Kind != "update" || g.Items[0].Shape != "file" {
		t.Fatalf("detail = %+v", g)
	}
	if res := get(t, srv, cookie, "/api/games/999"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown game = %d, want 404", res.StatusCode)
	}
}

func TestLibrarySearchIgnoresCaseAndAccents(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)
	store(t, srv, cookie, 166778, "arceus.nsp", "PFS0 a", map[string]any{"kind": "base"})
	store(t, srv, cookie, 119388, "totk.nsp", "PFS0 z", map[string]any{"kind": "base"})

	tests := map[string][]string{
		"pokemon":       {"Pokémon Legends: Arceus"},
		"ARCEUS legend": {"Pokémon Legends: Arceus"},
		"zelda kingdom": {"The Legend of Zelda: Tears of the Kingdom"},
		"legend":        {"Pokémon Legends: Arceus", "The Legend of Zelda: Tears of the Kingdom"},
		"mario":         {},
	}
	for q, want := range tests {
		games := decode[[]gameSummaryJSON](t, get(t, srv, cookie, "/api/games?q="+url.QueryEscape(q)))
		var titles []string
		for _, g := range games {
			titles = append(titles, g.Title)
		}
		if !slices.Equal(titles, want) {
			t.Errorf("search %q = %v, want %v", q, titles, want)
		}
	}
	if res := get(t, srv, cookie, "/api/games?q="+url.QueryEscape("  ")); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("blank search = %d, want 400", res.StatusCode)
	}
}

func TestDownloadSingleFileIsResumable(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)
	game := store(t, srv, cookie, 166778, "arceus.nsp", "0123456789", map[string]any{"kind": "base"})
	g := decode[gameDetailJSON](t, get(t, srv, cookie, "/api/games/"+strconv.FormatInt(game, 10)))
	item := "/api/items/" + strconv.FormatInt(g.Items[0].ID, 10) + "/download"

	res := get(t, srv, cookie, item)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || string(body) != "0123456789" || res.Header.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("download = %d %q %v", res.StatusCode, body, res.Header)
	}
	if cd := res.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, "Pok%C3%A9mon") {
		t.Fatalf("Content-Disposition = %q", cd)
	}

	res = get(t, srv, cookie, item, "Range", "bytes=2-5")
	body, _ = io.ReadAll(res.Body)
	if res.StatusCode != http.StatusPartialContent || string(body) != "2345" {
		t.Fatalf("range = %d %q", res.StatusCode, body)
	}
	if res := get(t, srv, cookie, item, "Range", "bytes=50-"); res.StatusCode != http.StatusRequestedRangeNotSatisfiable {
		t.Fatalf("bad range = %d, want 416", res.StatusCode)
	}

	// A replaced item is in the trash: no longer downloadable.
	store(t, srv, cookie, 166778, "arceus.xci", "HEAD new", map[string]any{"kind": "base", "onDuplicate": "replace"})
	if res := get(t, srv, cookie, item); res.StatusCode != http.StatusNotFound {
		t.Fatalf("trashed item = %d, want 404", res.StatusCode)
	}
	if res := get(t, srv, cookie, "/api/items/999/download"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown item = %d, want 404", res.StatusCode)
	}
}

func readZip(t *testing.T, res *http.Response) map[string]string {
	t.Helper()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "application/zip" {
		t.Fatalf("zip download = %d %s", res.StatusCode, body)
	}
	if res.ContentLength != int64(len(body)) {
		t.Fatalf("Content-Length %d, body %d", res.ContentLength, len(body))
	}
	r, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
		out[f.Name] = string(b)
	}
	return out
}

func TestDownloadDiscsAndGamesAsZip(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv, cookie, library := libraryServer(t)

	id := upload(t, srv, cookie, "ff7.zip", zipOf(t, map[string]string{
		"d1/ff7.cue":   "FILE \"ff7.bin\" BINARY\n",
		"d1/ff7.bin":   "disc one",
		"d2/ff7-2.cue": "FILE \"ff7-2.bin\" BINARY\n",
		"d2/ff7-2.bin": "disc two",
	}))
	waitStatus(t, srv, cookie, id, "review")
	res := post(t, srv.URL, cookie, id, "commit", commitBody(t, "psx", 427,
		map[string]any{"path": "d1/ff7.cue", "kind": "disc", "discNumber": 1},
		map[string]any{"path": "d2/ff7-2.cue", "kind": "disc", "discNumber": 2}))
	game := decode[commitJSON](t, res).GameID
	g := decode[gameDetailJSON](t, get(t, srv, cookie, "/api/games/"+strconv.FormatInt(game, 10)))
	if len(g.Items) != 2 || g.Items[0].Shape != "disc" {
		t.Fatalf("items = %+v", g.Items)
	}

	disc := get(t, srv, cookie, "/api/items/"+strconv.FormatInt(g.Items[0].ID, 10)+"/download")
	if cd := disc.Header.Get("Content-Disposition"); !strings.Contains(cd, "Final Fantasy VII (Disc 1).zip") {
		t.Fatalf("Content-Disposition = %q", cd)
	}
	files := readZip(t, disc)
	want := map[string]string{
		"Final Fantasy VII/Final Fantasy VII (Disc 1).cue": "FILE \"Final Fantasy VII (Disc 1).bin\" BINARY\n",
		"Final Fantasy VII/Final Fantasy VII (Disc 1).bin": "disc one",
	}
	if len(files) != len(want) {
		t.Fatalf("disc zip = %v", files)
	}
	for name, content := range want {
		if files[name] != content {
			t.Fatalf("disc zip %s = %q", name, files[name])
		}
	}

	whole := readZip(t, get(t, srv, cookie, "/api/games/"+strconv.FormatInt(game, 10)+"/download"))
	if len(whole) != 4 || whole["Final Fantasy VII/Final Fantasy VII (Disc 2).bin"] != "disc two" {
		t.Fatalf("game zip = %v", whole)
	}

	// A file deleted over SMB makes the download fail clearly, before any byte.
	if err := os.Remove(filepath.Join(library, "psx", "Final Fantasy VII", "Final Fantasy VII (Disc 2).bin")); err != nil {
		t.Fatal(err)
	}
	res = get(t, srv, cookie, "/api/games/"+strconv.FormatInt(game, 10)+"/download")
	if detail := problemDetail(t, res); res.StatusCode != http.StatusNotFound || !strings.Contains(detail, "SMB") {
		t.Fatalf("missing file = %d %s", res.StatusCode, detail)
	}
}

func TestDownloadFolderGameAsZip(t *testing.T) {
	t.Parallel()
	require7zz(t)
	srv, cookie, _ := libraryServer(t)

	id := upload(t, srv, cookie, "ps3.zip", zipOf(t, map[string]string{
		"BLUS/PS3_GAME/PARAM.SFO":         "\x00PSF params",
		"BLUS/PS3_GAME/USRDIR/EBOOT.BIN":  "eboot",
		"BLUS/PS3_GAME/USRDIR/data/a.dat": "data",
	}))
	waitStatus(t, srv, cookie, id, "review")
	got := items(t, srv, cookie, id)
	if len(got) != 1 || got[0].Shape != "folder" {
		t.Fatalf("items = %+v", got)
	}
	res := post(t, srv.URL, cookie, id, "commit", commitBody(t, "ps3", 119388, map[string]any{"path": got[0].Path, "kind": "base"}))
	game := decode[commitJSON](t, res).GameID
	g := decode[gameDetailJSON](t, get(t, srv, cookie, "/api/games/"+strconv.FormatInt(game, 10)))

	files := readZip(t, get(t, srv, cookie, "/api/items/"+strconv.FormatInt(g.Items[0].ID, 10)+"/download"))
	prefix := "The Legend of Zelda - Tears of the Kingdom/The Legend of Zelda - Tears of the Kingdom/"
	if files[prefix+"PS3_GAME/USRDIR/data/a.dat"] != "data" || len(files) != 3 {
		t.Fatalf("folder zip = %v", files)
	}
}
