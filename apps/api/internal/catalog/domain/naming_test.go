package domain_test

import (
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

type namingVectors struct {
	Cases []struct {
		Name  string `json:"name"`
		Input struct {
			Title      string `json:"title"`
			Kind       string `json:"kind"`
			Label      string `json:"label"`
			DiscNumber int    `json:"discNumber"`
			Extension  string `json:"extension"`
		} `json:"input"`
		Expected struct {
			SanitizedTitle string `json:"sanitizedTitle"`
			FileName       string `json:"fileName"`
			Error          string `json:"error"`
		} `json:"expected"`
	} `json:"cases"`
	TrackCases []struct {
		Name  string `json:"name"`
		Input struct {
			Stem       string   `json:"stem"`
			Extensions []string `json:"extensions"`
		} `json:"input"`
		Expected []string `json:"expected"`
	} `json:"trackCases"`
}

func loadNamingVectors(t *testing.T) namingVectors {
	t.Helper()
	raw, err := os.ReadFile("../../../../../contracts/naming-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var v namingVectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	if len(v.Cases) == 0 || len(v.TrackCases) == 0 {
		t.Fatal("no vectors loaded")
	}
	return v
}

func TestNamingGoldenVectors(t *testing.T) {
	t.Parallel()
	for _, c := range loadNamingVectors(t).Cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			n := domain.ItemName{
				Title: c.Input.Title, Kind: domain.ItemKind(c.Input.Kind),
				Label: c.Input.Label, DiscNumber: c.Input.DiscNumber,
			}
			got, err := n.FileName(c.Input.Extension)
			if c.Expected.Error != "" {
				var ne *domain.NameError
				if !errors.As(err, &ne) || ne.Field != c.Expected.Error {
					t.Fatalf("err = %v, want a %s error", err, c.Expected.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.Expected.FileName {
				t.Errorf("fileName = %q, want %q", got, c.Expected.FileName)
			}
			if s := domain.SanitizeTitle(c.Input.Title); s != c.Expected.SanitizedTitle {
				t.Errorf("sanitizedTitle = %q, want %q", s, c.Expected.SanitizedTitle)
			}
		})
	}
}

func TestTrackGoldenVectors(t *testing.T) {
	t.Parallel()
	for _, c := range loadNamingVectors(t).TrackCases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.TrackNames(c.Input.Stem, c.Input.Extensions)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got, c.Expected) {
				t.Errorf("tracks = %q, want %q", got, c.Expected)
			}
		})
	}
}

func TestFileNameLimits(t *testing.T) {
	t.Parallel()
	long := domain.ItemName{Title: strings.Repeat("a", 260), Kind: domain.KindBase}
	if _, err := long.FileName(".iso"); !errors.Is(err, domain.ErrNameTooLong) {
		t.Errorf("long title err = %v", err)
	}
	for _, ext := range []string{"iso", ".", ".i so", `.a"b`} {
		if _, err := (domain.ItemName{Title: "Game", Kind: domain.KindBase}).FileName(ext); err == nil {
			t.Errorf("extension %q accepted", ext)
		}
	}
	if _, err := (domain.ItemName{Title: "Game", Kind: domain.KindDisc}).FileName(".cue"); err == nil {
		t.Error("disc without number accepted")
	}
	if _, err := (domain.ItemName{Title: "Game", Kind: "demo"}).FileName(".iso"); err == nil {
		t.Error("unknown kind accepted")
	}
}

func TestGameFolder(t *testing.T) {
	t.Parallel()
	year := 2002
	tests := []struct {
		name  string
		year  *int
		taken []string
		want  string
	}{
		{"free title", &year, nil, "Resident Evil"},
		{"title taken adds the year", &year, []string{"Resident Evil"}, "Resident Evil (2002)"},
		{"year taken too adds the IGDB id", &year, []string{"Resident Evil", "Resident Evil (2002)"}, "Resident Evil (2002) [1234]"},
		{"unknown year goes to the id", nil, []string{"Resident Evil"}, "Resident Evil [1234]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := domain.GameFolder("Resident Evil", tt.year, 1234, func(f string) (bool, error) {
				return slices.Contains(tt.taken, f), nil
			})
			if err != nil || got != tt.want {
				t.Fatalf("GameFolder = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
	if _, err := domain.GameFolder("Resident Evil", nil, 1, func(string) (bool, error) { return true, nil }); err == nil {
		t.Error("every name taken must fail")
	}
}

func TestFindDuplicate(t *testing.T) {
	t.Parallel()
	entry := domain.TrashEntryID(1)
	existing := []domain.GameItem{
		{ID: 1, Kind: domain.KindBase, Files: []string{"Game.nsp"}},
		{ID: 2, Kind: domain.KindUpdate, Label: "v1.0.3", Files: []string{"Game [Update v1.0.3].nsp"}},
		{ID: 3, Kind: domain.KindDLC, Label: "Pack", Files: []string{"Game [DLC] Pack.nsp"}},
		{ID: 4, Kind: domain.KindDisc, DiscNumber: 1, Files: []string{"Game (Disc 1).cue", "Game (Disc 1).bin"}},
		{ID: 5, Kind: domain.KindUpdate, Label: "v2.0.0", Files: []string{"Game [Update v2.0.0].nsp"}, TrashEntry: &entry},
	}
	tests := []struct {
		name string
		c    domain.Candidate
		want domain.ItemID
	}{
		{"another base, other format", domain.Candidate{Kind: domain.KindBase, Files: []string{"Game.xci"}}, 1},
		{"same update version without v", domain.Candidate{Kind: domain.KindUpdate, Label: "1.0.3", Files: []string{"Game [Update 1.0.3].nsp"}}, 2},
		{"newer update is not a duplicate", domain.Candidate{Kind: domain.KindUpdate, Label: "v1.0.4", Files: []string{"Game [Update v1.0.4].nsp"}}, 0},
		{"dlc with the same file name", domain.Candidate{Kind: domain.KindDLC, Label: "pack", Files: []string{"Game [DLC] pack.nsp"}}, 3},
		{"another dlc", domain.Candidate{Kind: domain.KindDLC, Label: "Other", Files: []string{"Game [DLC] Other.nsp"}}, 0},
		{"same disc", domain.Candidate{Kind: domain.KindDisc, DiscNumber: 1, Files: []string{"Game (Disc 1).chd"}}, 4},
		{"next disc", domain.Candidate{Kind: domain.KindDisc, DiscNumber: 2, Files: []string{"Game (Disc 2).cue"}}, 0},
		{"trashed items do not count", domain.Candidate{Kind: domain.KindUpdate, Label: "v2.0.0", Files: []string{"Game [Update v2.0.0].nsp"}}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := domain.FindDuplicate(existing, tt.c)
			switch {
			case tt.want == 0 && got != nil:
				t.Fatalf("duplicate = %d, want none", got.ID)
			case tt.want != 0 && (got == nil || got.ID != tt.want):
				t.Fatalf("duplicate = %v, want %d", got, tt.want)
			}
		})
	}
}

func TestRewriteCue(t *testing.T) {
	t.Parallel()
	sheet := "REM test\r\nFILE \"Old Name (Track 1).bin\" BINARY\r\n  TRACK 01 MODE2/2352\r\n" +
		"file track2.bin binary\r\n  TRACK 02 AUDIO\r\nFILE \"missing.bin\" BINARY\r\n"
	got := domain.RewriteCue(sheet, func(ref string) (string, bool) {
		switch ref {
		case "Old Name (Track 1).bin":
			return "Game (Track 1).bin", true
		case "track2.bin":
			return "Game (Track 2).bin", true
		}
		return "", false
	})
	want := "REM test\r\nFILE \"Game (Track 1).bin\" BINARY\r\n  TRACK 01 MODE2/2352\r\n" +
		"file \"Game (Track 2).bin\" binary\r\n  TRACK 02 AUDIO\r\nFILE \"missing.bin\" BINARY\r\n"
	if got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}
