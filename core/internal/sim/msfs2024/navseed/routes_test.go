package navseed

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pack38(s string) uint64 {
	var v uint64
	for _, c := range s {
		v = v*38 + uint64(strings.IndexRune(base38, c))
	}
	return v
}

func idWord(name string, class int) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, pack38(name)<<idShift|uint64(class))
	return b
}

func regionWord(region string) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, pack38(region))
	return b
}

func ref(name string, class int, region string, altM float32) []byte {
	b := append(idWord(name, class), regionWord(region)...)
	return binary.LittleEndian.AppendUint32(b, math.Float32bits(altM))
}

func entry(kind byte, airway string, prev, next []byte) []byte {
	name := make([]byte, nameLen)
	copy(name, airway)
	return append(append(append([]byte{kind}, name...), prev...), next...)
}

// fixData is the data of a fix record: position (0, 0 is the middle of the encoding), identifier and
// region, and the entries.
func fixData(name string, class int, region string, entries ...[]byte) []byte {
	b := make([]byte, 14)
	binary.LittleEndian.PutUint32(b[2:6], 0x18000000)  // lon: 0 degrees
	binary.LittleEndian.PutUint32(b[6:10], 0x10000000) // lat: 0 degrees
	b = append(b, idWord(name, class)...)
	b = append(b, regionWord(region)...)
	for _, e := range entries {
		b = append(b, e...)
	}
	return b
}

func record(data []byte) []byte {
	h := []byte{0x08, 0x01, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(h[2:6], uint32(6+len(data)))
	return append(h, data...)
}

func TestParseFixReadsTheNameRegionPositionAndAirways(t *testing.T) {
	// Arrange
	data := fixData("REBAM", 5, "SB",
		entry(2, "UZ84", ref("IROMA", 5, "SB", 7467.6), ref("ANBUM", 5, "SB", 7467.6)),
		entry(1, "Z36", ref("", 0, "", 0), ref("PUBIN", 5, "SB", 4419.6)))

	// Act
	f, ok := parseFix(data)

	// Assert
	require.True(t, ok)
	assert.Equal(t, FileRef{Ident: "REBAM", Class: 5, Region: "SB"}, f.FileRef)
	assert.InDelta(t, 0.0, f.Lat, 1e-6)
	assert.InDelta(t, 0.0, f.Lon, 1e-6)
	require.Len(t, f.Airways, 2)
	uz := f.Airways[0]
	assert.Equal(t, "UZ84", uz.Name)
	assert.Equal(t, 2, uz.Type)
	assert.Equal(t, "IROMA", uz.Prev.Ident)
	assert.Equal(t, "ANBUM", uz.Next.Ident)
	assert.InDelta(t, 7467.6, uz.MinAltM, 0.01)
	z := f.Airways[1]
	assert.False(t, z.HasPrev, "the first fix of an airway has no previous one")
	assert.True(t, z.HasNext)
	assert.InDelta(t, 4419.6, z.MinAltM, 0.01, "taken from the fix that is there")
}

func TestParseFixWithoutAirwaysIsJustAFix(t *testing.T) {
	// Act
	f, ok := parseFix(fixData("PTL", 2, "SB"))

	// Assert
	require.True(t, ok)
	assert.Equal(t, 2, f.Class)
	assert.Empty(t, f.Airways)
}

func TestParseFixTooShortIsNotAFix(t *testing.T) {
	// Act
	_, ok := parseFix(make([]byte, 10))

	// Assert
	assert.False(t, ok)
}

func TestScanBlobFindsTheRecordsBetweenOtherBytes(t *testing.T) {
	// Arrange
	one := record(fixData("AAAAA", 5, "SB", entry(2, "UZ1", ref("BBBBB", 5, "SB", 1), ref("CCCCC", 5, "SB", 1))))
	two := record(fixData("DDDDD", 5, "K1"))
	blob := append([]byte{0, 0, 0x08, 0x01, 0xff}, one...)
	blob = append(blob, 0x55, 0x66)
	blob = append(blob, two...)
	var got []string

	// Act
	scanBlob(blob, func(b []byte) {
		f, _ := parseFix(b)
		got = append(got, f.Ident+"/"+f.Region)
	})

	// Assert
	assert.Equal(t, []string{"AAAAA/SB", "DDDDD/K1"}, got)
}

func TestScanBlobIgnoresARecordThatIsCutShort(t *testing.T) {
	// Arrange
	whole := record(fixData("AAAAA", 5, "SB"))

	// Act
	called := 0
	scanBlob(whole[:len(whole)-5], func([]byte) { called++ })

	// Assert
	assert.Zero(t, called)
}

func TestScanBlobIgnoresWhatDoesNotLookLikeAFix(t *testing.T) {
	// Arrange: a class the files do not use, and a region that is not letters or digits
	badClass := record(fixData("AAAAA", 9, "SB"))
	badRegion := record(fixData("AAAAA", 5, "??"))

	// Act
	called := 0
	scanBlob(append(badClass, badRegion...), func([]byte) { called++ })

	// Assert
	assert.Zero(t, called)
}

func TestWaypointsKeepTheirRegionAndKeepByNameNeighboursTheFilesDoNotHave(t *testing.T) {
	// Arrange
	a := FileFix{FileRef: FileRef{Ident: "AAAAA", Class: 5, Region: "SB"}, Lat: 1, Lon: 2, Airways: []FileAirway{
		{Name: "UZ1", Type: 2, Next: FileRef{Ident: "BBBBB", Class: 5, Region: "SB"}, HasNext: true,
			Prev: FileRef{Ident: "GONE", Class: 5, Region: "SB"}, HasPrev: true}}}
	b := FileFix{FileRef: FileRef{Ident: "BBBBB", Class: 5, Region: "SB"}, Lat: 3, Lon: 4}
	vor := FileFix{FileRef: FileRef{Ident: "PTL", Class: 2, Region: "SB"}}

	// Act
	got := waypoints([]FileFix{a, b, vor})

	// Assert
	require.Len(t, got, 2, "the VOR is not taken")
	assert.Equal(t, "SB", got[0].Region)
	require.Len(t, got[0].Routes, 1)
	r := got[0].Routes[0]
	assert.Equal(t, "BBBBB", r.NextIdent)
	assert.Equal(t, 3.0, r.NextLat)
	assert.Equal(t, "GONE", r.PrevIdent, "the airway still goes there")
	assert.Zero(t, r.PrevLat, "but the files do not say where")
}
