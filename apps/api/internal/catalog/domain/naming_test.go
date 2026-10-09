package domain_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/catalog/domain"
)

type namingVectors struct {
	Cases []struct {
		Name  string `json:"name"`
		Input struct {
			Title     string `json:"title"`
			Kind      string `json:"kind"`
			Label     string `json:"label"`
			Extension string `json:"extension"`
		} `json:"input"`
		Expected struct {
			SanitizedTitle string `json:"sanitizedTitle"`
			FileName       string `json:"fileName"`
			Error          string `json:"error"`
		} `json:"expected"`
	} `json:"cases"`
	ExtensionCases []struct {
		Name     string   `json:"name"`
		Known    []string `json:"known"`
		Expected string   `json:"expected"`
	} `json:"extensionCases"`
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
	if len(v.Cases) == 0 || len(v.ExtensionCases) == 0 {
		t.Fatal("no vectors loaded")
	}
	return v
}

func TestNamingGoldenVectors(t *testing.T) {
	t.Parallel()
	for _, c := range loadNamingVectors(t).Cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			label := c.Input.Label
			kind := domain.ItemKind(c.Input.Kind)
			if c.Expected.Error == "" || c.Expected.Error == "label" || c.Expected.Error == "version" || c.Expected.Error == "disc" {
				clean, err := domain.CleanLabel(kind, label)
				var ne *domain.NameError
				if c.Expected.Error != "" && c.Expected.Error != "title" {
					if !errors.As(err, &ne) || ne.Field != c.Expected.Error {
						t.Fatalf("CleanLabel err = %v, want field %q", err, c.Expected.Error)
					}
					return
				}
				if err != nil {
					t.Fatalf("CleanLabel: %v", err)
				}
				label = clean
			}
			name, err := domain.ItemName{Title: c.Input.Title, Kind: kind, Label: label}.FileName(c.Input.Extension)
			if c.Expected.Error != "" {
				var ne *domain.NameError
				if !errors.As(err, &ne) || ne.Field != c.Expected.Error {
					t.Fatalf("err = %v, want field %q", err, c.Expected.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if name != c.Expected.FileName {
				t.Errorf("file = %q, want %q", name, c.Expected.FileName)
			}
			if folder, _ := domain.GameFolder(c.Input.Title); folder != c.Expected.SanitizedTitle {
				t.Errorf("folder = %q, want %q", folder, c.Expected.SanitizedTitle)
			}
		})
	}
}

func TestExtensionGoldenVectors(t *testing.T) {
	t.Parallel()
	for _, c := range loadNamingVectors(t).ExtensionCases {
		if got := domain.ExtensionOf(c.Name, c.Known); got != c.Expected {
			t.Errorf("ExtensionOf(%q, %v) = %q, want %q", c.Name, c.Known, got, c.Expected)
		}
	}
}

func TestNameTooLong(t *testing.T) {
	t.Parallel()
	long := string(make([]byte, 0, 300))
	for range 250 {
		long += "a"
	}
	_, err := domain.ItemName{Title: long, Kind: domain.KindBase}.FileName(".nsp")
	if !errors.Is(err, domain.ErrNameTooLong) {
		t.Fatalf("err = %v", err)
	}
}
