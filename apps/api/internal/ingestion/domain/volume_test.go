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
	first, _ := domain.NewUploadJob("a", "g.7z.001", 10, nil, now)
	second, _ := domain.NewUploadJob("b", "g.7z.002", 10, nil, now)
	for _, j := range []*domain.UploadJob{first, second} {
		_ = j.MarkUploaded("/up/"+string(j.ID), now)
	}
	v1, _ := domain.ParseVolume(first.FileName)
	v2, _ := domain.ParseVolume(second.FileName)
	if err := first.WaitForParts(v1, "/vol/g.7z.001", now); err != nil {
		t.Fatal(err)
	}
	if err := second.WaitForParts(v2, "/vol/g.7z.002", now); err != nil {
		t.Fatal(err)
	}
	if err := second.MergeInto(first.ID, now); err != nil || second.MergedInto != "a" || !second.Status.Terminal() {
		t.Fatalf("merge: %v %+v", err, second)
	}
	if err := first.PartsComplete(now); err != nil || first.Status != domain.StatusUploaded || first.StoragePath != "/vol/g.7z.001" {
		t.Fatalf("complete: %v %+v", err, first)
	}
}
