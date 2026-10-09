// Command demo-catalog builds the fixed IGDB catalog used by the backend-less
// demo (spec RF-61). It is run by a developer with IGDB credentials, and its
// output is committed; the demo never talks to IGDB.
//
//	IGDB_CLIENT_ID=… IGDB_CLIENT_SECRET=… go run ./cmd/demo-catalog -out <file>
//	task demo-catalog
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/caarlos0/env/v11"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain/consoles"
	metadataapp "github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/application"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/metadata/infrastructure/igdb"
)

// required titles must exist in the catalog because the demo's sample files
// (spec RF-64) and the mockups use them.
var required = []struct {
	platform int64
	name     string
}{
	{130, "Mario Kart 8 Deluxe"},
	{130, "Inside"},
	{130, "The Rogue Prince of Persia"},
	{130, "Animal Crossing: New Horizons"},
	{130, "Splatoon 3"},
	{5, "The Legend of Zelda: Twilight Princess"},
	{38, "Daxter"},
}

type credentials struct {
	ClientID     string `env:"IGDB_CLIENT_ID,required"`
	ClientSecret string `env:"IGDB_CLIENT_SECRET,required"`
}

type catalog struct {
	GeneratedAt time.Time         `json:"generatedAt"`
	Source      string            `json:"source"`
	Platforms   []catalogPlatform `json:"platforms"`
	Games       []catalogGame     `json:"games"`
}

// The JSON shapes match the API's MetadataPlatform/MetadataGame schemas, so
// the demo adapters can reuse the generated TypeScript types.
type catalogPlatform struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Abbreviation *string `json:"abbreviation,omitempty"`
	LogoImageID  *string `json:"logoImageId,omitempty"`
	ReleaseYear  *int    `json:"releaseYear,omitempty"`
}

type catalogGame struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	ReleaseYear  *int     `json:"releaseYear,omitempty"`
	CoverImageID *string  `json:"coverImageId,omitempty"`
	Summary      *string  `json:"summary,omitempty"`
	Genres       []string `json:"genres"`
	PlatformIDs  []int64  `json:"platformIds"`
}

func main() {
	out := flag.String("out", "../web/src/modules/metadata/infrastructure/demo/catalog.json", "output file")
	perPlatform := flag.Int("per-platform", 18, "top-rated games per platform")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(*out, *perPlatform, log); err != nil {
		log.Error("demo catalog failed", "error", err)
		os.Exit(1)
	}
}

func run(out string, perPlatform int, log *slog.Logger) error {
	// The consoles defined in code, in their order (spec §6).
	var platforms []int64
	for _, c := range consoles.All() {
		platforms = append(platforms, c.IGDBPlatformID)
	}
	var creds credentials
	if err := env.Parse(&creds); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client := igdb.New(igdb.Config{ClientID: creds.ClientID, ClientSecret: creds.ClientSecret})
	svc := metadataapp.NewService(client, nil)

	ps, err := svc.PlatformsByID(ctx, platforms)
	if err != nil {
		return err
	}
	cat := catalog{GeneratedAt: time.Now().UTC().Truncate(time.Second), Source: "IGDB (https://www.igdb.com)"}
	for _, p := range ps {
		cat.Platforms = append(cat.Platforms, catalogPlatform(p))
	}

	seen := map[int64]bool{}
	add := func(g domain.Game) {
		if !seen[g.ID] {
			seen[g.ID] = true
			cat.Games = append(cat.Games, catalogGame(g))
		}
	}

	for _, r := range required {
		platform := r.platform
		games, err := svc.SearchGames(ctx, r.name, &platform, 1)
		if err != nil {
			return fmt.Errorf("required %q: %w", r.name, err)
		}
		if len(games) == 0 {
			return fmt.Errorf("required %q not found on platform %d", r.name, r.platform)
		}
		add(games[0])
	}
	for _, p := range platforms {
		games, err := client.TopGames(ctx, p, perPlatform)
		if err != nil {
			return fmt.Errorf("top games of %d: %w", p, err)
		}
		for _, g := range games {
			add(g)
		}
	}
	if len(cat.Games) == 0 {
		return errors.New("empty catalog")
	}

	data, err := json.MarshalIndent(cat, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(out, append(data, '\n'), 0o644); err != nil { //nolint:gosec // committed source file
		return err
	}
	log.Info("demo catalog written", "file", out, "platforms", len(cat.Platforms), "games", len(cat.Games))
	return nil
}
