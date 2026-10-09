package domain_test

import (
	"testing"
	"time"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
)

func TestParseVolume(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		set   string
		index int
	}{
		{"Animal Crossing.part1.rar", "animal crossing|rar-part", 1},
		{"Animal Crossing.PART02.rar", "animal crossing|rar-part", 2},
		{"game.part001.rar", "game|rar-part", 1},
		{"Zelda.7z.001", "zelda.7z|numbered", 1},
		{"Zelda.7z.003", "zelda.7z|numbered", 3},
		{"pack.zip.002", "pack.zip|numbered", 2},
		{"disc.iso.001", "disc.iso|numbered", 1},
	}
	for _, tt := range tests {
		v, ok := domain.ParseVolume(tt.name)
		if !ok || v.Set != tt.set || v.Index != tt.index {
			t.Errorf("%q = %+v %v, want set %q index %d", tt.name, v, ok, tt.set, tt.index)
		}
	}
	for _, single := range []string{"game.rar", "game.7z", "game.part.rar", "game.part0.rar", "game.7z.000", "game.r00", "INSIDE [v196608][1.0.3].nsp"} {
		if v, ok := domain.ParseVolume(single); ok {
			t.Errorf("%q parsed as volume %+v", single, v)
		}
	}
	if !domain.IsLegacyRarVolume("game.r00") || !domain.IsLegacyRarVolume("GAME.R17") || domain.IsLegacyRarVolume("game.rar") {
		t.Error("legacy .rNN detection")
	}
}

func TestVolumeLifecycle(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	group := domain.Spec{Console: "psp", Title: "G", GroupID: "g", GroupSize: 2}
	first, _ := domain.NewUploadJob("a", "g.7z.001", 10, group, now)
	second, _ := domain.NewUploadJob("b", "g.7z.002", 10, group, now)
	for _, j := range []*domain.UploadJob{first, second} {
		_ = j.MarkUploaded("/up/"+string(j.ID), now)
	}
	if err := first.WaitForParts("/vol/g.7z.001", now); err != nil {
		t.Fatal(err)
	}
	if err := second.WaitForParts("/vol/g.7z.002", now); err != nil {
		t.Fatal(err)
	}
	if err := second.MergeInto(first.ID, now); err != nil || second.MergedInto != "a" || !second.Status.Terminal() {
		t.Fatalf("merge: %v %+v", err, second)
	}
	if err := first.PartsComplete(now); err != nil || first.Status != domain.StatusUploaded || first.StoragePath != "/vol/g.7z.001" {
		t.Fatalf("complete: %v %+v", err, first)
	}
}

func TestFirstVolume(t *testing.T) {
	t.Parallel()

	tests := []struct {
		names []string
		first int
		ok    bool
	}{
		{[]string{"g.7z.003", "g.7z.001", "g.7z.002"}, 1, true},
		{[]string{"Game.part2.rar", "Game.part1.rar"}, 1, true},
		{[]string{"g.7z.001", "g.7z.003"}, 0, false},     // a part is missing
		{[]string{"g.7z.001", "other.7z.002"}, 0, false}, // two archives
		{[]string{"a.iso", "b.iso"}, 0, false},           // not parts at all
		{[]string{"g.7z.001", "g.7z.001"}, 0, false},     // the same part twice
		{[]string{"g.part1.rar", "g.7z.002"}, 0, false},  // mixed styles
	}
	for _, tt := range tests {
		first, ok := domain.FirstVolume(tt.names)
		if ok != tt.ok || (ok && first != tt.first) {
			t.Errorf("FirstVolume(%v) = %d %v, want %d %v", tt.names, first, ok, tt.first, tt.ok)
		}
	}
}
