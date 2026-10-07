// Package igdb implements the metadata Provider on the IGDB v4 API
// (https://api-docs.igdb.com). It owns everything IGDB-specific: OAuth tokens,
// the Apicalypse query language, rate limits and the response schema.
package igdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
)

// Default endpoints.
const (
	DefaultAPIURL   = "https://api.igdb.com/v4"
	DefaultTokenURL = "https://id.twitch.tv/oauth2/token" //nolint:gosec // an endpoint URL, not a credential
)

// playableGameTypes are IGDB game_type ids worth storing a file for:
// main game (0), standalone expansion (4), remake (8), remaster (9),
// expanded game (10) and port (11). Bundles, DLC, mods, seasons, episodes,
// updates and packs are excluded. Note "Mario Kart 8 Deluxe" is type 10, so
// filtering on main games only would hide it.
const playableGameTypes = "(0,4,8,9,10,11)"

// Config configures the client.
type Config struct {
	ClientID     string
	ClientSecret string
	APIURL       string
	TokenURL     string
	HTTPClient   *http.Client
}

// Client is a concurrency-safe IGDB client.
type Client struct {
	cfg     Config
	http    *http.Client
	limiter *rate.Limiter

	mu      sync.Mutex
	token   string
	expires time.Time
}

var _ application.Provider = (*Client)(nil)

// New builds a client.
func New(cfg Config) *Client {
	if cfg.APIURL == "" {
		cfg.APIURL = DefaultAPIURL
	}
	if cfg.TokenURL == "" {
		cfg.TokenURL = DefaultTokenURL
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: 15 * time.Second}
	}
	return &Client{
		cfg:  cfg,
		http: hc,
		// IGDB allows 4 requests per second per client.
		limiter: rate.NewLimiter(rate.Limit(4), 4),
	}
}

// ---------- Provider ----------

type apiGame struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	FirstReleaseDate *int64 `json:"first_release_date"`
	Summary          string `json:"summary"`
	Cover            *struct {
		ImageID string `json:"image_id"`
	} `json:"cover"`
	Genres []struct {
		Name string `json:"name"`
	} `json:"genres"`
	Platforms []int64 `json:"platforms"`
}

const gameFields = "fields name,first_release_date,summary,cover.image_id,genres.name,platforms;"

// SearchGames implements application.Provider.
func (c *Client) SearchGames(ctx context.Context, query string, platformID *int64, limit int) ([]domain.Game, error) {
	where := "where game_type = " + playableGameTypes
	if platformID != nil {
		where += " & platforms = (" + strconv.FormatInt(*platformID, 10) + ")"
	}
	body := fmt.Sprintf("search %s; %s %s; limit %d;", quote(query), gameFields, where, limit)

	var raw []apiGame
	if err := c.query(ctx, "games", body, &raw); err != nil {
		return nil, err
	}
	return mapGames(raw), nil
}

// TopGames returns the most rated playable games of a platform. Used by the
// demo catalog generator, not by the app.
func (c *Client) TopGames(ctx context.Context, platformID int64, limit int) ([]domain.Game, error) {
	body := fmt.Sprintf("%s where platforms = (%d) & game_type = %s & cover != null & total_rating_count > 20; sort total_rating_count desc; limit %d;",
		gameFields, platformID, playableGameTypes, limit)
	var raw []apiGame
	if err := c.query(ctx, "games", body, &raw); err != nil {
		return nil, err
	}
	return mapGames(raw), nil
}

type apiPlatform struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	PlatformLogo *struct {
		ImageID string `json:"image_id"`
	} `json:"platform_logo"`
	Versions []apiPlatformVersion `json:"versions"`
}

type apiPlatformVersion struct {
	Name         string `json:"name"`
	PlatformLogo *struct {
		ImageID string `json:"image_id"`
	} `json:"platform_logo"`
	ReleaseDates []struct {
		Y int `json:"y"`
	} `json:"platform_version_release_dates"`
}

func (v apiPlatformVersion) firstYear() int {
	first := 0
	for _, d := range v.ReleaseDates {
		if d.Y > 0 && (first == 0 || d.Y < first) {
			first = d.Y
		}
	}
	return first
}

const platformFields = "fields name,abbreviation,platform_logo.image_id,versions.name,versions.platform_logo.image_id,versions.platform_version_release_dates.y;"

// SearchPlatforms implements application.Provider.
func (c *Client) SearchPlatforms(ctx context.Context, query string, limit int) ([]domain.Platform, error) {
	body := fmt.Sprintf("search %s; %s limit %d;", quote(query), platformFields, limit)
	var raw []apiPlatform
	if err := c.query(ctx, "platforms", body, &raw); err != nil {
		return nil, err
	}
	return mapPlatforms(raw), nil
}

// PlatformsByID implements application.Provider.
func (c *Client) PlatformsByID(ctx context.Context, ids []int64) ([]domain.Platform, error) {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	body := fmt.Sprintf("%s where id = (%s); limit %d;", platformFields, strings.Join(parts, ","), len(ids))
	var raw []apiPlatform
	if err := c.query(ctx, "platforms", body, &raw); err != nil {
		return nil, err
	}
	return mapPlatforms(raw), nil
}

// ---------- mapping (IGDB schema -> domain) ----------

func mapGames(raw []apiGame) []domain.Game {
	out := make([]domain.Game, 0, len(raw))
	for _, g := range raw {
		game := domain.Game{ID: g.ID, Name: g.Name, PlatformIDs: g.Platforms, Genres: []string{}}
		if game.PlatformIDs == nil {
			game.PlatformIDs = []int64{}
		}
		if g.FirstReleaseDate != nil {
			y := time.Unix(*g.FirstReleaseDate, 0).UTC().Year()
			game.ReleaseYear = &y
		}
		if g.Cover != nil && g.Cover.ImageID != "" {
			id := g.Cover.ImageID
			game.CoverImageID = &id
		}
		if s := strings.TrimSpace(g.Summary); s != "" {
			game.Summary = &s
		}
		for _, genre := range g.Genres {
			game.Genres = append(game.Genres, genre.Name)
		}
		out = append(out, game)
	}
	return out
}

func mapPlatforms(raw []apiPlatform) []domain.Platform {
	out := make([]domain.Platform, 0, len(raw))
	for _, p := range raw {
		platform := domain.Platform{ID: p.ID, Name: p.Name}
		if p.Abbreviation != "" {
			a := p.Abbreviation
			platform.Abbreviation = &a
		}
		if logo := launchLogo(p); logo != "" {
			platform.LogoImageID = &logo
		}
		// The platform's launch year is the earliest release of any version
		// (e.g. Wii: 2006, although later revisions shipped in 2011-2013).
		var years []int
		for _, v := range p.Versions {
			if y := v.firstYear(); y > 0 {
				years = append(years, y)
			}
		}
		if len(years) > 0 {
			y := slices.Min(years)
			platform.ReleaseYear = &y
		}
		out = append(out, platform)
	}
	return out
}

// launchLogo picks the logo of the platform's original model. IGDB's
// platform-level logo is usually the latest revision ("Nintendo Switch OLED
// Model", "Wii mini", "PS one", PS2 Slim...), so prefer the version released
// first, breaking ties towards "Initial version"/"Original", and fall back to
// the platform logo.
func launchLogo(p apiPlatform) string {
	best, bestYear, bestInitial := "", 0, false
	for _, v := range p.Versions {
		if v.PlatformLogo == nil || v.PlatformLogo.ImageID == "" {
			continue
		}
		year := v.firstYear()
		if year == 0 {
			continue
		}
		name := strings.ToLower(v.Name)
		initial := strings.Contains(name, "initial") || strings.Contains(name, "original")
		if best == "" || year < bestYear || (year == bestYear && initial && !bestInitial) {
			best, bestYear, bestInitial = v.PlatformLogo.ImageID, year, initial
		}
	}
	if best == "" && p.PlatformLogo != nil {
		best = p.PlatformLogo.ImageID
	}
	return best
}

// quote escapes a search term for Apicalypse.
func quote(s string) string {
	s = strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s)
	return `"` + s + `"`
}

// ---------- transport ----------

// query POSTs an Apicalypse body to endpoint and decodes the JSON result.
// On 401 it refreshes the token once (tokens can be revoked before expiry).
func (c *Client) query(ctx context.Context, endpoint, body string, out any) error {
	for attempt := range 2 {
		token, err := c.accessToken(ctx, attempt > 0)
		if err != nil {
			return err
		}
		if err := c.limiter.Wait(ctx); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.APIURL+"/"+endpoint, strings.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Client-ID", c.cfg.ClientID)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")

		res, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("%w: %v", domain.ErrUpstream, err) //nolint:errorlint // wrap the domain error, keep the cause as text
		}
		data, readErr := io.ReadAll(io.LimitReader(res.Body, 4<<20))
		_ = res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("%w: read %s: %v", domain.ErrUpstream, endpoint, readErr) //nolint:errorlint // see above
		}
		switch {
		case res.StatusCode == http.StatusUnauthorized && attempt == 0:
			continue
		case res.StatusCode != http.StatusOK:
			return fmt.Errorf("%w: %s returned %d: %s", domain.ErrUpstream, endpoint, res.StatusCode, truncate(data))
		}
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("%w: decode %s: %v", domain.ErrUpstream, endpoint, err) //nolint:errorlint // see above
		}
		return nil
	}
	return fmt.Errorf("%w: unauthorized after token refresh", domain.ErrUpstream)
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// accessToken returns a cached app access token, refreshing it when it is
// about to expire or when force is set.
func (c *Client) accessToken(ctx context.Context, force bool) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && c.token != "" && time.Now().Before(c.expires) {
		return c.token, nil
	}

	form := url.Values{
		"client_id":     {c.cfg.ClientID},
		"client_secret": {c.cfg.ClientSecret},
		"grant_type":    {"client_credentials"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.TokenURL, bytes.NewBufferString(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: token: %v", domain.ErrUpstream, err) //nolint:errorlint // see query
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return "", fmt.Errorf("%w: token endpoint returned %d (check IGDB_CLIENT_ID/IGDB_CLIENT_SECRET): %s",
			domain.ErrUpstream, res.StatusCode, truncate(data))
	}
	var tr tokenResponse
	if err := json.NewDecoder(res.Body).Decode(&tr); err != nil || tr.AccessToken == "" {
		return "", errors.Join(domain.ErrUpstream, errors.New("token: invalid response"), err)
	}
	c.token = tr.AccessToken
	// Renew a minute early so in-flight requests never carry an expired token.
	c.expires = time.Now().Add(time.Duration(tr.ExpiresIn)*time.Second - time.Minute)
	return c.token, nil
}

func truncate(b []byte) string {
	const max = 300
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}
