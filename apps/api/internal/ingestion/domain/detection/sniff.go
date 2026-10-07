package detection

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"strings"
)

// Detector keys of the built-in consoles (consoles.detector_key).
const (
	DetectorSwitch = "switch"
	DetectorWii    = "wii"
	DetectorGC     = "gc"
	DetectorPSX    = "psx"
	DetectorPS2    = "ps2"
	DetectorPS3    = "ps3"
)

// SniffFile identifies a console from a file's content. It only reads a few
// kilobytes (header bytes and, for disc images, the ISO9660 root directory).
// It returns the detector key, or "" when the content is inconclusive.
func SniffFile(r io.ReaderAt, size int64) string {
	head := make([]byte, 0x200)
	n, _ := r.ReadAt(head, 0)
	head = head[:n]

	switch {
	case bytes.HasPrefix(head, []byte("PFS0")):
		return DetectorSwitch // NSP / NSZ
	case len(head) >= 0x104 && bytes.Equal(head[0x100:0x104], []byte("HEAD")):
		return DetectorSwitch // XCI / XCZ
	case bytes.HasPrefix(head, []byte("WBFS")):
		return DetectorWii
	case len(head) >= 0x1C && bytes.Equal(head[0x18:0x1C], []byte{0x5D, 0x1C, 0x9E, 0xA3}):
		return DetectorWii // Wii disc magic at 0x18
	case len(head) >= 0x20 && bytes.Equal(head[0x1C:0x20], []byte{0xC2, 0x33, 0x9F, 0x3D}):
		return DetectorGC // GameCube disc magic at 0x1C
	case bytes.HasPrefix(head, []byte("\x00PBP")):
		return DetectorPSX // PSP-style eboot of a PS1 game
	case bytes.HasPrefix(head, []byte("\x7FPKG")):
		return DetectorPS3
	}
	return sniffISO9660(r, size)
}

// sniffISO9660 recognizes PlayStation discs by their root directory:
// SYSTEM.CNF with BOOT2 (PS2) or BOOT (PS1), or PS3_DISC.SFB (PS3).
func sniffISO9660(r io.ReaderAt, size int64) string {
	img, ok := openISO(r, size)
	if !ok {
		return ""
	}
	entries, ok := img.rootEntries()
	if !ok {
		return ""
	}
	if e, found := entries["SYSTEM.CNF"]; found {
		cnf, ok := img.readFile(e, 4096)
		if !ok {
			return ""
		}
		return parseSystemCNF(cnf)
	}
	if _, found := entries["PS3_DISC.SFB"]; found {
		return DetectorPS3
	}
	return ""
}

func parseSystemCNF(cnf []byte) string {
	for line := range strings.SplitSeq(strings.ToUpper(string(cnf)), "\n") {
		key, _, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "BOOT2":
			return DetectorPS2
		case "BOOT":
			return DetectorPSX
		}
	}
	return ""
}

// SniffFolder recognizes folder-format games (PS3 "JB" folders).
func SniffFolder(fsys fs.FS, dir string) string {
	if exists(fsys, path.Join(dir, "PS3_GAME", "PARAM.SFO")) || exists(fsys, path.Join(dir, "PS3_DISC.SFB")) {
		return DetectorPS3
	}
	return ""
}

func exists(fsys fs.FS, name string) bool {
	_, err := fs.Stat(fsys, name)
	return err == nil
}
