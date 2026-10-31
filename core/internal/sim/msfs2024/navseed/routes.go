package navseed

import (
	"encoding/binary"
	"math"
)

// The fix record of the simulator's files, past the position and the identifier, lists the airways that
// go through the fix. After the identifier (an 8-byte word that holds the name and the class) comes the
// region as an 8-byte word of two base-38 characters, and then the entries:
//
//	type (1 byte: 1 low, 2 high) | airway name (8 bytes, ASCII, NUL padded) | previous fix | next fix
//
// where each of the two fixes is the 8-byte identifier word, the 8-byte region word and a float32
// with the minimum altitude, in meters (7467.6 m is FL245).
//
// The record came in two layouts. Until the simulator update of 25 October 2026 it was record 0x0108
// and the entries followed the region. Since then it is record 0x011e: a uint32 follows the region (how
// many 88-byte records of id 0x0112 close the fix; they are not read) and the entries follow that, and
// the record ends with those and up to 3 bytes of padding. Both are read.
const (
	fixHeader = 30 // up to the end of the region
	refSize   = 20
	entrySize = 1 + nameLen + 2*refSize
	nameLen   = 8
	identAt   = 14
	regionAt  = 22
	entryMin  = 1 // entry types: 1 low, 2 high; 3 and above are other kinds of route
	entryMax  = 7
	identMask = 0x3f
	idShift   = 6
)

// layout is one of the two ways the files hold a fix record.
type layout struct {
	id    uint16 // the BGL record id
	first int    // where the first entry is
	tails bool   // the record has 88-byte records after the entries, counted by the uint32 past the region
}

var (
	legacy  = layout{id: 0x0108, first: fixHeader}
	current = layout{id: 0x011e, first: fixHeader + 4, tails: true}
)

// tailSize is the size of the records that close a fix of the current layout.
const tailSize = 88

// FileRef names a fix as the files do: by name, class and region.
type FileRef struct {
	Ident  string
	Class  int
	Region string
}

// key is the fix a reference points to.
func (r FileRef) key() fixKey { return fixKey{r.Ident, r.Region, r.Class} }

// FileAirway is one airway through a fix, with the fix before and the fix after on it.
type FileAirway struct {
	Name    string
	Type    int // 1 low, 2 high, 3...: see the entries
	Prev    FileRef
	Next    FileRef
	MinAltM float32
	HasPrev bool
	HasNext bool
}

// FileFix is a fix as the files hold it.
type FileFix struct {
	FileRef
	Lat, Lon float64
	Airways  []FileAirway
}

// parseRef reads a reference to a fix: its identifier word, region word and altitude.
func parseRef(b []byte) (FileRef, float32, bool) {
	id := binary.LittleEndian.Uint64(b[0:8])
	if id == 0 {
		return FileRef{}, 0, false
	}
	region := binary.LittleEndian.Uint64(b[8:16])
	alt := math.Float32frombits(binary.LittleEndian.Uint32(b[16:20]))
	return FileRef{Ident: unpack38(id >> idShift), Class: int(id & identMask), Region: unpack38(region)}, alt, true
}

// parseFix reads the data of a fix record (without its 6-byte header).
func parseFix(b []byte, l layout) (FileFix, bool) {
	if len(b) < l.first {
		return FileFix{}, false
	}
	le := binary.LittleEndian
	lonU, latU := le.Uint32(b[2:6]), le.Uint32(b[6:10])
	id1 := le.Uint64(b[identAt : identAt+8])
	ident := unpack38(id1 >> idShift)
	if ident == "" {
		return FileFix{}, false
	}
	f := FileFix{
		FileRef: FileRef{Ident: ident, Class: int(id1 & identMask), Region: unpack38(le.Uint64(b[regionAt : regionAt+8]))},
		Lat:     90 - float64(latU)*(180.0/0x20000000),
		Lon:     float64(lonU)*(360.0/0x30000000) - 180,
	}
	rest := b[l.first:]
	for len(rest) >= entrySize && rest[0] >= entryMin && rest[0] <= entryMax {
		a := FileAirway{Type: int(rest[0])}
		name := rest[1 : 1+nameLen]
		n := 0
		for n < len(name) && name[n] != 0 {
			n++
		}
		a.Name = string(name[:n])
		var alt, alt2 float32
		a.Prev, alt, a.HasPrev = parseRef(rest[1+nameLen:])
		a.Next, alt2, a.HasNext = parseRef(rest[1+nameLen+refSize:])
		a.MinAltM = alt
		if !a.HasPrev {
			a.MinAltM = alt2
		}
		f.Airways = append(f.Airways, a)
		rest = rest[entrySize:]
	}
	return f, true
}

// scanBlob finds the fix records inside a run of bytes that is not split into records (the airways'
// section holds them that way, cut at arbitrary places), and calls fn with the data of each and the
// layout it is in. A record is taken only when it is whole and its content makes sense: that is what
// keeps bytes that merely look like a header out.
func scanBlob(b []byte, fn func(b []byte, l layout)) {
	for i := 0; i+6+fixHeader <= len(b); {
		var l layout
		switch {
		case b[i] == byte(legacy.id) && b[i+1] == byte(legacy.id>>8):
			l = legacy
		case b[i] == byte(current.id) && b[i+1] == byte(current.id>>8):
			l = current
		default:
			i++
			continue
		}
		size := int(binary.LittleEndian.Uint32(b[i+2 : i+6]))
		if size < 6+l.first || size > 6+l.maxBody() || i+size > len(b) {
			i++
			continue
		}
		body := b[i+6 : i+size]
		if !plausibleFix(body, l) {
			i++
			continue
		}
		fn(body, l)
		i += size
	}
}

// maxBody is the biggest data of a fix record that scanBlob believes.
func (l layout) maxBody() int {
	n := l.first + 8*entrySize + 3
	if l.tails {
		n += 8 * tailSize
	}
	return n
}

// plausibleFix tells whether the data looks like a fix record: a class of fix the files use, a region
// of two letters or digits, and entries that fill what is left of the record.
func plausibleFix(body []byte, l layout) bool {
	if len(body) < l.first {
		return false
	}
	id1 := binary.LittleEndian.Uint64(body[identAt : identAt+8])
	switch id1 & identMask {
	case 2, 3, 5, 6:
	default:
		return false
	}
	if id1>>idShift == 0 || id1>>idShift >= 38*38*38*38*38*38 {
		return false
	}
	region := unpack38(binary.LittleEndian.Uint64(body[regionAt : regionAt+8]))
	if len(region) != 2 {
		return false
	}
	for _, c := range region {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	rest := body[l.first:]
	if !l.tails {
		return len(rest)%entrySize <= 3
	}
	for len(rest) >= entrySize && rest[0] >= entryMin && rest[0] <= entryMax {
		rest = rest[entrySize:]
	}
	tails := int(binary.LittleEndian.Uint32(body[fixHeader : fixHeader+4]))
	return tails <= 8 && len(rest) >= tails*tailSize && len(rest)-tails*tailSize <= 3
}
