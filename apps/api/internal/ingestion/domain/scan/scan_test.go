package scan_test

import (
	"slices"
	"testing"
	"testing/fstest"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/detection"
	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/scan"
)

var profiles = []detection.ConsoleProfile{
	{Slug: "switch", Extensions: []string{".nsp", ".xci"}, DetectorKey: "switch"},
	{Slug: "gc", Extensions: []string{".iso", ".rvz"}, DetectorKey: "gc"},
	{Slug: "psx", Extensions: []string{".cue", ".bin"}, DetectorKey: "psx"},
	{Slug: "ps2", Extensions: []string{".iso", ".bin", ".cue"}, DetectorKey: "ps2"},
	{Slug: "ps3", Extensions: []string{".iso", ".pkg"}, DetectorKey: "ps3"},
}

func byPath(items []domain.StagedItem, p string) domain.StagedItem {
	for _, it := range items {
		if it.Path == p {
			return it
		}
	}
	return domain.StagedItem{}
}

func TestScanRealSwitchBundleWithWrapperAndJunk(t *testing.T) {
	t.Parallel()

	// Shape of a real release (phase 0.5): wrapper folder, base, update, DLC, junk.
	fsys := fstest.MapFS{
		"Rogue Prince BASE GAME/The Rogue Prince of Persia [01008D9022462000] [v0].nsp":                  {Data: []byte("PFS0 base")},
		"Rogue Prince BASE GAME/The Rogue Prince of Persia [01008D9022462800] [v131072]Update 1.0.4.nsp": {Data: []byte("PFS0 upd")},
		"Rogue Prince BASE GAME/DLC [01008D9022463001][v0].nsp":                                          {Data: []byte("PFS0 dlc")},
		"Rogue Prince BASE GAME/leeme.txt":                                                               {Data: []byte("hola")},
		"Rogue Prince BASE GAME/release.nfo":                                                             {Data: []byte("nfo")},
		"__MACOSX/._junk":                                                                                {Data: []byte("x")},
	}

	items, err := scan.Scan(fsys, "job1", profiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 5 {
		t.Fatalf("items = %d (%+v), want 3 games + 2 ignored", len(items), items)
	}

	base := byPath(items, "Rogue Prince BASE GAME/The Rogue Prince of Persia [01008D9022462000] [v0].nsp")
	if base.SuggestedKind != detection.KindBase || base.Confidence != domain.ConfidenceHeader ||
		!slices.Equal(base.Consoles, []string{"switch"}) || base.JobID != "job1" {
		t.Errorf("base = %+v", base)
	}
	upd := byPath(items, "Rogue Prince BASE GAME/The Rogue Prince of Persia [01008D9022462800] [v131072]Update 1.0.4.nsp")
	if upd.SuggestedKind != detection.KindUpdate || upd.DisplayVersion != "1.0.4" || upd.VersionCode != "131072" {
		t.Errorf("update = %+v", upd)
	}
	if dlc := byPath(items, "Rogue Prince BASE GAME/DLC [01008D9022463001][v0].nsp"); dlc.SuggestedKind != detection.KindDLC {
		t.Errorf("dlc = %+v", dlc)
	}
	if txt := byPath(items, "Rogue Prince BASE GAME/leeme.txt"); !txt.Ignored {
		t.Errorf("leeme.txt must be ignored: %+v", txt)
	}
}

func TestScanGroupsCueWithBins(t *testing.T) {
	t.Parallel()

	cue := "FILE \"Final Fantasy VII (Disc 1) (Track 1).bin\" BINARY\n  TRACK 01 MODE2/2352\n" +
		"FILE \"final fantasy vii (disc 1) (track 2).BIN\" BINARY\n  TRACK 02 AUDIO\n"
	fsys := fstest.MapFS{
		"FF7/Final Fantasy VII (Disc 1).cue":           {Data: []byte(cue)},
		"FF7/Final Fantasy VII (Disc 1) (Track 1).bin": {Data: make([]byte, 100)},
		"FF7/Final Fantasy VII (Disc 1) (Track 2).bin": {Data: make([]byte, 50)},
		"FF7/Final Fantasy VII (Disc 2).cue":           {Data: []byte("FILE \"Final Fantasy VII (Disc 2).bin\" BINARY\n")},
		"FF7/Final Fantasy VII (Disc 2).bin":           {Data: make([]byte, 70)},
	}

	items, err := scan.Scan(fsys, "j", profiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2 discs", items)
	}
	d1 := byPath(items, "FF7/Final Fantasy VII (Disc 1).cue")
	if d1.Shape != domain.ShapeDisc || len(d1.Parts) != 3 || d1.Size != int64(len(cue))+150 ||
		d1.SuggestedKind != detection.KindDisc || d1.DiscNumber != 1 {
		t.Errorf("disc 1 = %+v (case-insensitive track names must match)", d1)
	}
	if d2 := byPath(items, "FF7/Final Fantasy VII (Disc 2).cue"); d2.DiscNumber != 2 || len(d2.Parts) != 2 {
		t.Errorf("disc 2 = %+v", d2)
	}
}

func TestScanKeepsFolderGamesWhole(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"Demons Souls [BLUS30443]/PS3_GAME/PARAM.SFO":      {Data: []byte("PSF")},
		"Demons Souls [BLUS30443]/PS3_GAME/USRDIR/EBOOT.B": {Data: make([]byte, 1000)},
		"Demons Souls [BLUS30443]/PS3_DISC.SFB":            {Data: []byte("SFB")},
	}
	items, err := scan.Scan(fsys, "j", profiles)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %+v, want the folder as one item", items)
	}
	it := items[0]
	if it.Shape != domain.ShapeFolder || it.Path != "Demons Souls [BLUS30443]" || it.Size != 1006 ||
		!slices.Equal(it.Consoles, []string{"ps3"}) || it.Confidence != domain.ConfidenceHeader {
		t.Errorf("folder item = %+v", it)
	}
}

func TestScanUnknownAndAmbiguousFiles(t *testing.T) {
	t.Parallel()

	gc := make([]byte, 0x40)
	copy(gc[0x1C:], []byte{0xC2, 0x33, 0x9F, 0x3D})
	fsys := fstest.MapFS{
		"Metroid Prime.iso": {Data: gc},
		"mystery.iso":       {Data: make([]byte, 0x40)},
		"homebrew.elf":      {Data: []byte("\x7FELF")},
	}
	items, err := scan.Scan(fsys, "j", profiles)
	if err != nil {
		t.Fatal(err)
	}
	if mp := byPath(items, "Metroid Prime.iso"); !slices.Equal(mp.Consoles, []string{"gc"}) || mp.Confidence != domain.ConfidenceHeader {
		t.Errorf("gc iso = %+v", mp)
	}
	if m := byPath(items, "mystery.iso"); len(m.Consoles) != 3 || m.Confidence != domain.ConfidenceExtension {
		t.Errorf("ambiguous iso = %+v", m)
	}
	if e := byPath(items, "homebrew.elf"); len(e.Consoles) != 0 || e.Confidence != domain.ConfidenceNone || e.Ignored {
		t.Errorf("unknown file = %+v (listed so the user can choose)", e)
	}
}
