package main

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestLibraryListsConsolesGamesAndDetail(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)

	limbo := store(t, srv, cookie, "switch", "Limbo", "limbo.nsp", "base", map[string]any{"kind": "base"})
	store(t, srv, cookie, "switch", "Limbo", "limbo-up.nsp", "update", map[string]any{"kind": "update", "label": "1.10"})
	store(t, srv, cookie, "switch", "Limbo", "limbo-up2.nsp", "update", map[string]any{"kind": "update", "label": "1.9"})
	store(t, srv, cookie, "psp", "Daxter", "daxter.cso", "cso", map[string]any{})

	consoles := decode[[]struct {
		Slug          string   `json:"slug"`
		GameCount     int      `json:"gameCount"`
		Kinds         []string `json:"kinds"`
		MultipleFiles bool     `json:"multipleFiles"`
		Extensions    []string `json:"extensions"`
	}](t, get(t, srv, cookie, "/api/consoles"))
	// Default order (spec §6): n64, gc, wii, switch, ps2, psp.
	if len(consoles) != 6 || consoles[3].Slug != "switch" || consoles[3].GameCount != 1 || !consoles[3].MultipleFiles ||
		consoles[2].Slug != "wii" || consoles[2].GameCount != 0 || consoles[5].GameCount != 1 || len(consoles[5].Kinds) != 1 ||
		len(consoles[1].Kinds) != 2 {
		t.Fatalf("consoles = %+v", consoles)
	}

	list := decode[[]gameJSON](t, get(t, srv, cookie, "/api/consoles/switch/games"))
	if len(list) != 1 || list[0].ID != limbo || list[0].ItemCount != 3 || list[0].Size != int64(len("base")+2*len("update")) {
		t.Fatalf("switch games = %+v", list)
	}
	if res := get(t, srv, cookie, "/api/consoles/ps3/games"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown console = %d, want 404", res.StatusCode)
	}

	g, _ := gameOf(t, srv, cookie, limbo)
	labels := []string{}
	for _, it := range g.Items[1:] {
		labels = append(labels, *it.Label)
	}
	if g.Path != "switch/Limbo" || g.Items[0].Kind != "base" || strings.Join(labels, ",") != "1.9,1.10" || g.Genres == nil {
		t.Fatalf("game = %+v (updates sorted by version)", g)
	}
	if _, status := gameOf(t, srv, cookie, 999); status != http.StatusNotFound {
		t.Fatalf("unknown game = %d", status)
	}
}

func TestLibrarySearchIgnoresCaseAndAccents(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)
	store(t, srv, cookie, "switch", "Pokémon Legends: Arceus", "p.nsp", "x", map[string]any{"kind": "base"})
	store(t, srv, cookie, "psp", "Daxter", "d.iso", "x", map[string]any{})

	found := decode[[]gameJSON](t, get(t, srv, cookie, "/api/games?q="+url.QueryEscape("POKEMON arceus")))
	if len(found) != 1 || found[0].Folder != "Pokémon Legends - Arceus" {
		t.Fatalf("found = %+v", found)
	}
	if res := get(t, srv, cookie, "/api/games?q="+url.QueryEscape("  ")); res.StatusCode != http.StatusBadRequest {
		t.Fatalf("blank search = %d, want 400", res.StatusCode)
	}
}

func TestDownloadsResumeAndZipAndNameNonASCII(t *testing.T) {
	t.Parallel()
	srv, cookie, _ := libraryServer(t)
	game := store(t, srv, cookie, "wii", "Ōkami", "o.iso", "0123456789", map[string]any{})
	store(t, srv, cookie, "switch", "Limbo", "l.nsp", "base", map[string]any{"kind": "base"})

	g, _ := gameOf(t, srv, cookie, game)
	item := g.Items[0]
	res := get(t, srv, cookie, "/api/items/"+id(item.ID)+"/download", "Range", "bytes=4-")
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusPartialContent || string(b) != "456789" {
		t.Fatalf("range download = %d %q", res.StatusCode, b)
	}
	cd := res.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "filename=Okami.iso;") || !strings.Contains(cd, "filename*=UTF-8''%C5%8Ckami.iso") {
		t.Fatalf("Content-Disposition = %q (needs an ASCII fallback)", cd)
	}

	zipped := readZip(t, get(t, srv, cookie, "/api/games/"+id(game)+"/download"))
	if len(zipped) != 1 || zipped["Ōkami/Ōkami.iso"] != "0123456789" {
		t.Fatalf("zip = %v", zipped)
	}
	if res := get(t, srv, cookie, "/api/items/999/download"); res.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown item = %d, want 404", res.StatusCode)
	}
}
