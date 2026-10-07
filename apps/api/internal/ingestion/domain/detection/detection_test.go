package detection_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/CarlosSV923/GameExplorer/apps/api/internal/ingestion/domain/detection"
)

// builtins mirrors db/migrations/00002_seed_consoles.sql; the composition
// test TestSeedMatchesDetectionProfiles checks they stay equal.
var builtins = []detection.ConsoleProfile{
	{Slug: "switch", Extensions: []string{".nsp", ".xci", ".nsz", ".xcz"}, DetectorKey: "switch"},
	{Slug: "wii", Extensions: []string{".iso", ".wbfs", ".rvz"}, DetectorKey: "wii"},
	{Slug: "gc", Extensions: []string{".iso", ".gcm", ".ciso", ".rvz"}, DetectorKey: "gc"},
	{Slug: "psx", Extensions: []string{".cue", ".bin", ".chd", ".pbp"}, DetectorKey: "psx"},
	{Slug: "ps2", Extensions: []string{".iso", ".chd", ".bin", ".cue"}, DetectorKey: "ps2"},
	{Slug: "ps3", Extensions: []string{".iso", ".pkg"}, DetectorKey: "ps3"},
}

func TestGoldenVectors(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("../../../../../../contracts/detection-cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []struct {
			Name  string `json:"name"`
			Input struct {
				FileName string `json:"fileName"`
			} `json:"input"`
			Expected struct {
				Consoles       []string `json:"consoles"`
				Kind           string   `json:"kind"`
				TitleID        string   `json:"titleId"`
				VersionCode    string   `json:"versionCode"`
				DisplayVersion string   `json:"displayVersion"`
				DiscNumber     int      `json:"discNumber"`
			} `json:"expected"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("no cases loaded")
	}

	for _, c := range file.Cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			got := detection.FromName(c.Input.FileName, builtins)
			gotConsoles, wantConsoles := slices.Sorted(slices.Values(got.Consoles)), slices.Sorted(slices.Values(c.Expected.Consoles))
			if !slices.Equal(gotConsoles, wantConsoles) {
				t.Errorf("consoles = %v, want %v", got.Consoles, c.Expected.Consoles)
			}
			if string(got.Kind) != c.Expected.Kind || got.TitleID != c.Expected.TitleID ||
				got.VersionCode != c.Expected.VersionCode || got.DisplayVersion != c.Expected.DisplayVersion ||
				got.DiscNumber != c.Expected.DiscNumber {
				t.Errorf("got %+v, want %+v", got, c.Expected)
			}
		})
	}
}

// ---------- header sniffing ----------

func at(offset int, magic []byte) []byte {
	b := make([]byte, 0x400)
	copy(b[offset:], magic)
	return b
}

func TestSniffMagicBytes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"nsp", at(0, []byte("PFS0")), detection.DetectorSwitch},
		{"xci", at(0x100, []byte("HEAD")), detection.DetectorSwitch},
		{"wbfs", at(0, []byte("WBFS")), detection.DetectorWii},
		{"wii disc", at(0x18, []byte{0x5D, 0x1C, 0x9E, 0xA3}), detection.DetectorWii},
		{"gamecube disc", at(0x1C, []byte{0xC2, 0x33, 0x9F, 0x3D}), detection.DetectorGC},
		{"ps1 eboot", at(0, []byte("\x00PBP")), detection.DetectorPSX},
		{"ps3 pkg", at(0, []byte("\x7FPKG")), detection.DetectorPS3},
		{"random bytes", bytes.Repeat([]byte{0xAB}, 0x9000), ""},
		{"tiny file", []byte("hi"), ""},
	}
	for _, tt := range tests {
		if got := detection.SniffFile(bytes.NewReader(tt.data), int64(len(tt.data))); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSniffPlayStationDiscs(t *testing.T) {
	t.Parallel()

	ps2CNF := []byte("BOOT2 = cdrom0:\\SLUS_212.66;1\r\nVER = 1.00\r\nVMODE = NTSC\r\n")
	ps1CNF := []byte("BOOT = cdrom:\\SCUS_941.63;1\r\nTCB = 4\r\nEVENT = 10\r\n")

	tests := []struct {
		name  string
		image []byte
		want  string
	}{
		{"ps2 iso", buildISO(t, map[string][]byte{"SYSTEM.CNF": ps2CNF}, 0), detection.DetectorPS2},
		{"ps1 iso", buildISO(t, map[string][]byte{"SYSTEM.CNF": ps1CNF}, 0), detection.DetectorPSX},
		{"ps1 raw bin (mode 2)", buildISO(t, map[string][]byte{"README.TXT": []byte("x"), "SYSTEM.CNF": ps1CNF}, 2), detection.DetectorPSX},
		{"ps2 raw bin (mode 1)", buildISO(t, map[string][]byte{"SYSTEM.CNF": ps2CNF}, 1), detection.DetectorPS2},
		{"ps3 iso", buildISO(t, map[string][]byte{"PS3_DISC.SFB": []byte(".SFB")}, 0), detection.DetectorPS3},
		{"generic iso", buildISO(t, map[string][]byte{"SETUP.EXE": []byte("MZ")}, 0), ""},
	}
	for _, tt := range tests {
		if got := detection.SniffFile(bytes.NewReader(tt.image), int64(len(tt.image))); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSniffFolder(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"BLUS30001/PS3_GAME/PARAM.SFO": {Data: []byte("\x00PSF")},
		"BLUS30001/PS3_DISC.SFB":       {Data: []byte(".SFB")},
		"other/readme.txt":             {Data: []byte("x")},
	}
	if got := detection.SniffFolder(fsys, "BLUS30001"); got != detection.DetectorPS3 {
		t.Errorf("ps3 folder = %q", got)
	}
	if got := detection.SniffFolder(fsys, "other"); got != "" {
		t.Errorf("plain folder = %q", got)
	}
}

// buildISO writes a minimal ISO9660 image with files in the root directory.
// mode 0 = cooked 2048-byte sectors; 1 or 2 = raw 2352-byte CD sectors.
func buildISO(t *testing.T, files map[string][]byte, mode int) []byte {
	t.Helper()
	const sector = 2048
	names := slices.Sorted(func(yield func(string) bool) {
		for n := range files {
			if !yield(n) {
				return
			}
		}
	})

	logical := make([][]byte, 19+len(names))
	for i := range logical {
		logical[i] = make([]byte, sector)
	}
	// Primary volume descriptor at sector 16, terminator at 17.
	pvd := logical[16]
	pvd[0], pvd[6] = 1, 1
	copy(pvd[1:6], "CD001")
	root := pvd[156:190]
	root[0] = 34
	binary.LittleEndian.PutUint32(root[2:6], 18)
	binary.LittleEndian.PutUint32(root[10:14], sector)
	root[25] = 2 // directory
	root[32] = 1
	term := logical[17]
	term[0], term[6] = 255, 1
	copy(term[1:6], "CD001")

	// Root directory at sector 18, files from sector 19.
	dir := logical[18]
	pos := 0
	for i, name := range names {
		id := name + ";1"
		recLen := 33 + len(id)
		recLen += recLen % 2
		rec := dir[pos : pos+recLen]
		rec[0] = byte(recLen)
		binary.LittleEndian.PutUint32(rec[2:6], uint32(19+i))
		binary.LittleEndian.PutUint32(rec[10:14], uint32(len(files[name])))
		rec[32] = byte(len(id))
		copy(rec[33:], id)
		copy(logical[19+i], files[name])
		pos += recLen
	}

	var out bytes.Buffer
	for i, data := range logical {
		if mode == 0 {
			out.Write(data)
			continue
		}
		raw := make([]byte, 2352)
		copy(raw, []byte{0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00})
		raw[12], raw[13], raw[14] = 0, 2, byte(i) // address (not validated by the reader)
		raw[15] = byte(mode)
		offset := 16
		if mode == 2 {
			offset = 24 // 8-byte subheader
		}
		copy(raw[offset:], data)
		out.Write(raw)
	}
	return out.Bytes()
}
