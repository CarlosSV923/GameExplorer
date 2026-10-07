package igdb_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/infrastructure/igdb"
)

// fakeIGDB mimics the Twitch token endpoint and the IGDB API.
type fakeIGDB struct {
	tokens    atomic.Int32
	lastBody  atomic.Value
	reject401 atomic.Int32 // number of API calls to answer with 401
	response  string
}

func (f *fakeIGDB) server(t *testing.T) igdb.Config {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("client_secret") != "secret" || r.Form.Get("grant_type") != "client_credentials" {
			http.Error(w, `{"message":"invalid client secret"}`, http.StatusForbidden)
			return
		}
		n := f.tokens.Add(1)
		_, _ = io.WriteString(w, `{"access_token":"tok`+strconv.Itoa(int(n))+`","expires_in":5000000,"token_type":"bearer"}`)
	})
	mux.HandleFunc("POST /v4/{endpoint}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Client-ID") != "id" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer tok") {
			http.Error(w, "bad headers", http.StatusBadRequest)
			return
		}
		if f.reject401.Load() > 0 {
			f.reject401.Add(-1)
			http.Error(w, `{"message":"Authorization Failure"}`, http.StatusUnauthorized)
			return
		}
		b, _ := io.ReadAll(r.Body)
		f.lastBody.Store(r.PathValue("endpoint") + ": " + string(b))
		_, _ = io.WriteString(w, f.response)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return igdb.Config{ClientID: "id", ClientSecret: "secret", APIURL: srv.URL + "/v4", TokenURL: srv.URL + "/token"}
}

func TestSearchGamesQueryAndMapping(t *testing.T) {
	t.Parallel()

	f := &fakeIGDB{response: `[{"id":26764,"name":"Mario Kart 8 Deluxe","first_release_date":1493337600,
		"summary":" Race! ","cover":{"id":1,"image_id":"co213p"},"genres":[{"id":10,"name":"Racing"}],"platforms":[508,130]},
		{"id":9,"name":"No cover"}]`}
	cfg := f.server(t)
	platform := int64(130)

	games, err := igdb.New(cfg).SearchGames(t.Context(), `say "hi"`, &platform, 5)
	if err != nil {
		t.Fatal(err)
	}

	body := f.lastBody.Load().(string)
	for _, want := range []string{`games: search "say \"hi\"";`, "game_type = (0,4,8,9,10,11)", "platforms = (130)", "limit 5;"} {
		if !strings.Contains(body, want) {
			t.Errorf("query %q missing %q", body, want)
		}
	}

	g := games[0]
	if g.ID != 26764 || *g.ReleaseYear != 2017 || *g.CoverImageID != "co213p" || *g.Summary != "Race!" ||
		g.Genres[0] != "Racing" || len(g.PlatformIDs) != 2 {
		t.Fatalf("mapped game = %+v", g)
	}
	if games[1].CoverImageID != nil || games[1].ReleaseYear != nil || games[1].Genres == nil || games[1].PlatformIDs == nil {
		t.Fatalf("optional fields = %+v", games[1])
	}
}

func TestPlatformUsesLaunchModelLogoAndYear(t *testing.T) {
	t.Parallel()

	// Real IGDB data (phase 2 probe): the platform logo is the newest revision.
	f := &fakeIGDB{response: `[
		{"id":5,"name":"Wii","platform_logo":{"image_id":"pl92"},"versions":[
			{"name":"Wii mini","platform_logo":{"image_id":"pl92"},"platform_version_release_dates":[{"y":2012},{"y":2013}]},
			{"name":"Starlight Wii Gaming Station"},
			{"name":"Initial version","platform_logo":{"image_id":"pl70"},"platform_version_release_dates":[{"y":2006},{"y":2006}]}]},
		{"id":21,"name":"Nintendo GameCube","platform_logo":{"image_id":"pl7a"},"versions":[
			{"name":"Panasonic Q","platform_logo":{"image_id":"jtbb"},"platform_version_release_dates":[{"y":2001}]},
			{"name":"Initial version","platform_logo":{"image_id":"pl7a"},"platform_version_release_dates":[{"y":2001}]}]},
		{"id":99,"name":"No versions","platform_logo":{"image_id":"plx"}}]`}
	cfg := f.server(t)

	ps, err := igdb.New(cfg).PlatformsByID(t.Context(), []int64{5, 130})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.lastBody.Load().(string), "where id = (5,130); limit 2;") {
		t.Errorf("query = %s", f.lastBody.Load())
	}
	if *ps[0].ReleaseYear != 2006 || *ps[0].LogoImageID != "pl70" || ps[0].Abbreviation != nil {
		t.Fatalf("wii = %+v (want the 2006 launch logo, not Wii mini)", ps[0])
	}
	if *ps[1].LogoImageID != "pl7a" {
		t.Fatalf("gamecube logo = %s (same-year tie must prefer Initial version)", *ps[1].LogoImageID)
	}
	if *ps[2].LogoImageID != "plx" || ps[2].ReleaseYear != nil {
		t.Fatalf("fallback platform = %+v", ps[2])
	}
}

func TestTokenIsCachedAndRefreshedOn401(t *testing.T) {
	t.Parallel()

	f := &fakeIGDB{response: `[]`}
	cfg := f.server(t)
	c := igdb.New(cfg)

	for range 3 {
		if _, err := c.SearchPlatforms(t.Context(), "wii", 5); err != nil {
			t.Fatal(err)
		}
	}
	if n := f.tokens.Load(); n != 1 {
		t.Fatalf("token requests = %d, want 1 (cached)", n)
	}

	f.reject401.Store(1)
	if _, err := c.SearchPlatforms(t.Context(), "wii", 5); err != nil {
		t.Fatalf("after revoked token: %v", err)
	}
	if n := f.tokens.Load(); n != 2 {
		t.Fatalf("token requests = %d, want 2 (refreshed once)", n)
	}
}

func TestBadCredentialsAndUpstreamErrors(t *testing.T) {
	t.Parallel()

	f := &fakeIGDB{response: `[]`}
	cfg := f.server(t)
	cfg.ClientSecret = "wrong"
	_, err := igdb.New(cfg).SearchPlatforms(t.Context(), "wii", 5)
	if !errors.Is(err, domain.ErrUpstream) || !strings.Contains(err.Error(), "IGDB_CLIENT_SECRET") {
		t.Fatalf("err = %v", err)
	}

	f2 := &fakeIGDB{response: `not json`}
	cfg2 := f2.server(t)
	if _, err := igdb.New(cfg2).SearchPlatforms(t.Context(), "wii", 5); !errors.Is(err, domain.ErrUpstream) {
		t.Fatalf("decode err = %v", err)
	}
}
