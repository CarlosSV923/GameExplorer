package detection

import (
	"bytes"
	"encoding/binary"
	"io"
	"strings"
)

// A minimal ISO9660 reader: just enough to list the root directory and read a
// small file. Handles cooked images (2048-byte sectors, .iso) and raw CD
// images (2352-byte sectors, .bin) in Mode 1 or Mode 2.

const (
	logicalSector = 2048
	rawSector     = 2352
	pvdSector     = 16
	maxDirBytes   = 64 << 10
)

var rawSync = []byte{0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00}

type isoImage struct {
	r          io.ReaderAt
	sectorSize int64 // physical sector size
	dataOffset int64 // offset of the 2048 data bytes inside a physical sector
	root       isoEntry
}

type isoEntry struct {
	lba  uint32
	size uint32
}

func openISO(r io.ReaderAt, size int64) (*isoImage, bool) {
	layouts := []isoImage{{sectorSize: logicalSector, dataOffset: 0}}
	first := make([]byte, 16)
	if n, _ := r.ReadAt(first, 0); n == 16 && bytes.Equal(first[:12], rawSync) {
		switch first[15] {
		case 1:
			layouts = append(layouts, isoImage{sectorSize: rawSector, dataOffset: 16})
		case 2:
			layouts = append(layouts, isoImage{sectorSize: rawSector, dataOffset: 24})
		}
	}
	for _, l := range layouts {
		img := l
		img.r = r
		pvd, ok := img.readSector(pvdSector, size)
		// Primary volume descriptor: type 1, "CD001".
		if !ok || pvd[0] != 1 || string(pvd[1:6]) != "CD001" {
			continue
		}
		rec := pvd[156:190] // root directory record
		img.root = isoEntry{lba: binary.LittleEndian.Uint32(rec[2:6]), size: binary.LittleEndian.Uint32(rec[10:14])}
		return &img, true
	}
	return nil, false
}

func (img *isoImage) readSector(n uint32, size int64) ([]byte, bool) {
	off := int64(n)*img.sectorSize + img.dataOffset
	if size > 0 && off+logicalSector > size {
		return nil, false
	}
	buf := make([]byte, logicalSector)
	if read, err := img.r.ReadAt(buf, off); read != logicalSector && err != nil {
		return nil, false
	}
	return buf, true
}

// read reads length bytes starting at a logical sector, across sectors.
func (img *isoImage) read(lba uint32, length int) ([]byte, bool) {
	out := make([]byte, 0, length)
	for sector := lba; len(out) < length; sector++ {
		buf, ok := img.readSector(sector, 0)
		if !ok {
			return nil, false
		}
		out = append(out, buf[:min(logicalSector, length-len(out))]...)
	}
	return out, true
}

// rootEntries maps upper-case file names (without ";1") to their extents.
func (img *isoImage) rootEntries() (map[string]isoEntry, bool) {
	dir, ok := img.read(img.root.lba, int(min(img.root.size, maxDirBytes)))
	if !ok {
		return nil, false
	}
	entries := map[string]isoEntry{}
	for pos := 0; pos < len(dir); {
		recLen := int(dir[pos])
		if recLen == 0 { // records never cross a sector: jump to the next one
			pos = (pos/logicalSector + 1) * logicalSector
			continue
		}
		if pos+recLen > len(dir) || recLen < 34 {
			break
		}
		rec := dir[pos : pos+recLen]
		nameLen := int(rec[32])
		if 33+nameLen <= len(rec) {
			name := strings.ToUpper(string(rec[33 : 33+nameLen]))
			name, _, _ = strings.Cut(name, ";")
			entries[name] = isoEntry{lba: binary.LittleEndian.Uint32(rec[2:6]), size: binary.LittleEndian.Uint32(rec[10:14])}
		}
		pos += recLen
	}
	return entries, true
}

func (img *isoImage) readFile(e isoEntry, limit int) ([]byte, bool) {
	return img.read(e.lba, int(min(e.size, uint32(limit)))) //nolint:gosec // limit is a small positive constant
}
