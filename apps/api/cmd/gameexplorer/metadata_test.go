package main

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/platform/config"
)

// fakeGames are the games the fake IGDB returns by id.
var fakeGames = map[string]string{
	"26764":  `{"id":26764,"name":"Mario Kart 8 Deluxe","first_release_date":1493337600,"cover":{"image_id":"co213p"},"genres":[{"name":"Racing"}],"platforms":[130]}`,
	"427":    `{"id":427,"name":"Final Fantasy VII","first_release_date":854668800,"summary":"Cloud.","platforms":[7]}`,
	"166778": `{"id":166778,"name":"Pokémon Legends: Arceus","first_release_date":1643328000,"platforms":[130]}`,
	"119388": `{"id":119388,"name":"The Legend of Zelda: Tears of the Kingdom","first_release_date":1683849600,"platforms":[130]}`,
}

// fakePlatforms are the platforms the fake IGDB returns by id.
var fakePlatforms = map[string]string{
	"130": `{"id":130,"name":"Nintendo Switch","platform_logo":{"image_id":"plgu"},"versions":[{"platform_version_release_dates":[{"y":2017}]}]}`,
	"38":  `{"id":38,"name":"PlayStation Portable","platform_logo":{"image_id":"pl6q"},"versions":[{"platform_version_release_dates":[{"y":2004}]}]}`,
}

// fakeIGDBServer answers the token endpoint, the API and the image CDN.
func fakeIGDBServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"access_token":"tok","expires_in":3600}`)
	})
	mux.HandleFunc("POST /v4/games", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if _, id, ok := strings.Cut(string(body), "where id = "); ok {
			id, _, _ = strings.Cut(id, ";")
			_, _ = io.WriteString(w, "["+fakeGames[id]+"]")
			return
		}
		_, _ = io.WriteString(w, `[
			{"id":191419,"name":"Mario Kart 8 Deluxe: Booster Course Pass","platforms":[130]},
			{"id":26764,"name":"Mario Kart 8 Deluxe","first_release_date":1493337600,"cover":{"image_id":"co213p"},"genres":[{"name":"Racing"}],"platforms":[130]}]`)
	})
	mux.HandleFunc("POST /v4/platforms", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_, ids, _ := strings.Cut(string(body), "where id = (")
		ids, _, _ = strings.Cut(ids, ")")
		var out []string
		for _, id := range strings.Split(ids, ",") {
			if p, ok := fakePlatforms[id]; ok {
				out = append(out, p)
			}
		}
		_, _ = io.WriteString(w, "["+strings.Join(out, ",")+"]")
	})
	mux.HandleFunc("GET /img/t_logo_med/plgu.png", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("\x89PNG fake"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func withFakeIGDB(srv *httptest.Server) func(*config.Config) {
	return func(c *config.Config) {
		c.IGDBClientID, c.IGDBClientSecret = "id", "secret"
		c.IGDBAPIURL, c.IGDBTokenURL, c.IGDBImageURL = srv.URL+"/v4", srv.URL+"/token", srv.URL+"/img"
	}
}

func TestMetadataWithoutIGDBAnswers503(t *testing.T) {
	t.Parallel()
	srv := newTestServer(t, nil)
	cookie := login(t, srv)

	status := decode[struct {
		Configured bool `json:"configured"`
	}](t, do(t, http.MethodGet, srv.URL+"/api/metadata/status", "", cookie))
	if status.Configured {
		t.Fatal("IGDB must not be configured")
	}

	res := do(t, http.MethodGet, srv.URL+"/api/metadata/games?q=zelda", "", cookie)
	if res.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", res.StatusCode)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "IGDB_CLIENT_ID") {
		t.Fatalf("body = %s", body)
	}
}

func TestMetadataSearchAndImagesThroughIGDB(t *testing.T) {
	t.Parallel()
	igdbSrv := fakeIGDBServer(t)
	srv := newTestServer(t, withFakeIGDB(igdbSrv))
	cookie := login(t, srv)

	games := decode[[]struct {
		ID           int64   `json:"id"`
		Name         string  `json:"name"`
		ReleaseYear  *int    `json:"releaseYear"`
		CoverImageID *string `json:"coverImageId"`
	}](t, do(t, http.MethodGet, srv.URL+"/api/metadata/games?q=mario+kart+8+deluxe&platformId=130", "", cookie))
	if len(games) != 2 || games[0].ID != 26764 || *games[0].ReleaseYear != 2017 || *games[0].CoverImageID != "co213p" {
		t.Fatalf("games = %+v (exact title must rank first)", games)
	}

	if res := do(t, http.MethodGet, srv.URL+"/api/metadata/games?q=m", "", cookie); res.StatusCode != http.StatusBadRequest {
		t.Errorf("short query status = %d, want 400", res.StatusCode)
	}

	res := do(t, http.MethodGet, srv.URL+"/api/images/logo_med/plgu", "", cookie)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" ||
		!strings.Contains(res.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("image = %d %q %q", res.StatusCode, res.Header.Get("Content-Type"), res.Header.Get("Cache-Control"))
	}
	if res := do(t, http.MethodGet, srv.URL+"/api/images/original/plgu", "", cookie); res.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown size status = %d, want 400", res.StatusCode)
	}
	if res := do(t, http.MethodGet, srv.URL+"/api/images/cover_big/missing", "", cookie); res.StatusCode != http.StatusNotFound {
		t.Errorf("missing image status = %d, want 404", res.StatusCode)
	}
	if res := do(t, http.MethodGet, srv.URL+"/api/images/logo_med/plgu", ""); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("image without session = %d, want 401", res.StatusCode)
	}
}

func TestConsoleSyncFromIGDB(t *testing.T) {
	t.Parallel()
	igdbSrv := fakeIGDBServer(t)
	cfg := config.Config{
		Port: 8080, LibraryPath: t.TempDir(), DataPath: t.TempDir(), Password: testPassword,
		SessionSecret: strings.Repeat("s", config.MinSessionSecretBytes), SessionTTL: 3600e9, ExtractConcurrency: 1, ScanInterval: 3600e9,
	}
	withFakeIGDB(igdbSrv)(&cfg)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := newApp(t.Context(), cfg, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })

	a.syncConsoles(t.Context(), log)

	sw, err := a.consoles.Console(t.Context(), "switch")
	if err != nil {
		t.Fatal(err)
	}
	if sw.LogoImageID == nil || *sw.LogoImageID != "plgu" {
		t.Fatalf("switch logo after sync = %v", sw.LogoImageID)
	}
}
