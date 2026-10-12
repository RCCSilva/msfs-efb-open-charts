package geotiff

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// buildTIFF writes a small little-endian GeoTIFF: 4x2, 8-bit grey, uncompressed, LCC (two parallels).
func buildTIFF(t *testing.T, withGeo bool) []byte {
	t.Helper()
	le := binary.LittleEndian
	type tag struct {
		id, typ uint16
		count   uint32
		val     []byte // little-endian value bytes
	}
	u16 := func(v ...uint16) []byte {
		b := make([]byte, 2*len(v))
		for i, x := range v {
			le.PutUint16(b[i*2:], x)
		}
		return b
	}
	u32 := func(v uint32) []byte { b := make([]byte, 4); le.PutUint32(b, v); return b }
	f64 := func(v ...float64) []byte {
		b := make([]byte, 8*len(v))
		for i, x := range v {
			le.PutUint64(b[i*8:], math.Float64bits(x))
		}
		return b
	}
	pixels := []byte{0, 10, 20, 30, 40, 50, 60, 70}
	tags := []tag{
		{256, 3, 1, u16(4)}, {257, 3, 1, u16(2)}, {258, 3, 1, u16(8)}, {259, 3, 1, u16(1)}, {262, 3, 1, u16(1)},
		{273, 4, 1, nil}, // strip offset: patched below
		{277, 3, 1, u16(1)}, {278, 3, 1, u16(2)}, {279, 4, 1, u32(8)},
	}
	if withGeo {
		keys := u16(1, 1, 0, 6, // header: version 1.1.0, 6 keys
			1024, 0, 1, 1, // ModelType = projected
			2048, 0, 1, 4269, // GeographicType = NAD83
			2056, 0, 1, 7019, // Ellipsoid = GRS80
			3072, 0, 1, 32767, // user-defined projected CS
			3075, 0, 1, 8, // Lambert Conformal Conic 2SP
			3078, 34736, 1, 0) // StdParallel1 in GeoDoubleParams[0]
		keys = append(keys[:6], keys[6:]...)
		_ = keys
		tags = append(tags,
			tag{33550, 12, 3, f64(100, 100, 0)},
			tag{33922, 12, 6, f64(0, 0, 0, -1000, 2000, 0)},
		)
		// the full key list: 8 keys
		full := u16(1, 1, 0, 9,
			1024, 0, 1, 1,
			2048, 0, 1, 4269,
			2056, 0, 1, 7019,
			3072, 0, 1, 32767,
			3075, 0, 1, 8,
			3078, 34736, 1, 0,
			3079, 34736, 1, 1,
			3080, 34736, 1, 2,
			3081, 34736, 1, 3)
		tags = append(tags,
			tag{34735, 3, uint32(len(full) / 2), full},
			tag{34736, 12, 4, f64(42.75, 42.08333, -71, 42.36667)})
	}
	// layout: header(8) + pixels(8) + IFD + values
	ifdOff := 16
	n := len(tags)
	valOff := ifdOff + 2 + n*12 + 4
	var vals bytes.Buffer
	var ifd bytes.Buffer
	_ = binary.Write(&ifd, le, uint16(n))
	for _, tg := range tags {
		var field [12]byte
		le.PutUint16(field[0:], tg.id)
		le.PutUint16(field[2:], tg.typ)
		le.PutUint32(field[4:], tg.count)
		v := tg.val
		if tg.id == 273 {
			v = u32(8)
		}
		if len(v) <= 4 {
			copy(field[8:], v)
		} else {
			le.PutUint32(field[8:], uint32(valOff+vals.Len()))
			vals.Write(v)
		}
		ifd.Write(field[:])
	}
	ifd.Write([]byte{0, 0, 0, 0})
	var out bytes.Buffer
	out.Write([]byte{'I', 'I', 42, 0})
	_ = binary.Write(&out, le, uint32(ifdOff))
	out.Write(pixels)
	out.Write(ifd.Bytes())
	out.Write(vals.Bytes())
	return out.Bytes()
}

func TestReadsAGeoTIFFAndItsLambertProjection(t *testing.T) {
	data := buildTIFF(t, true)
	g, err := ReadGeoref(data)
	if err != nil {
		t.Fatal(err)
	}
	if g.Width != 4 || g.Height != 2 || g.DX != 100 || g.DY != -100 || g.X0 != -1000 || g.Y0 != 2000 {
		t.Fatalf("georef = %+v", g)
	}
	if g.Projection.Kind != "lcc" || g.Projection.Lat1 != 42.75 || g.Projection.Lon0 != -71 || g.Projection.Lat0 != 42.36667 {
		t.Fatalf("projection = %+v", g.Projection)
	}
	if g.Datum != "NAD83" {
		t.Errorf("datum = %q", g.Datum)
	}
	if e, n := g.PixelToProjected(2, 1); e != -800 || n != 1900 {
		t.Errorf("pixel (2,1) = %v, %v", e, n)
	}
	if c, r := g.ProjectedToPixel(-800, 1900); c != 2 || r != 1 {
		t.Errorf("projected back = %v, %v", c, r)
	}
	img, err := Decode(data)
	if err != nil || img.Bounds().Dx() != 4 || img.Bounds().Dy() != 2 {
		t.Fatalf("decode: %v %v", err, img)
	}
}

func TestATIFFWithoutGeoTagsIsRefusedPlainly(t *testing.T) {
	if _, err := ReadGeoref(buildTIFF(t, false)); err != ErrNotGeoreference {
		t.Fatalf("err = %v", err)
	}
	if _, err := ReadGeoref([]byte("%PDF-1.4 ...")); err != ErrNotTIFF {
		t.Fatalf("a PDF: %v", err)
	}
}

// With a real FAA chart around (samples are never committed), the numbers must agree with its
// world file and metadata: 10.5845 m x 10.5839 m pixels. The world file gives the CENTRE of the
// first pixel (-54831.98, 48224.95); the GeoTIFF tag gives its corner, half a pixel further.
func TestFAAHelicopterChartIfPresent(t *testing.T) {
	p := filepath.Join("..", "..", "..", "samples", "heli", "Boston HEL.tif")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Skip("sample not present")
	}
	g, err := ReadGeoref(data)
	if err != nil {
		t.Fatal(err)
	}
	if g.Width != 10491 || g.Height != 8691 {
		t.Fatalf("size = %dx%d", g.Width, g.Height)
	}
	if math.Abs(g.DX-10.5845538695) > 1e-6 || math.Abs(g.DY+10.5839382341) > 1e-6 {
		t.Errorf("pixel size = %v, %v", g.DX, g.DY)
	}
	if math.Abs(g.X0-(-54831.9854141097-g.DX/2)) > 0.01 || math.Abs(g.Y0-(48224.9517815849-g.DY/2)) > 0.01 {
		t.Errorf("origin = %v, %v", g.X0, g.Y0)
	}
	if g.Projection.Kind != "lcc" || math.Abs(g.Projection.Lat0-42.36667) > 1e-4 || math.Abs(g.Projection.Lon0+71) > 1e-9 {
		t.Errorf("projection = %+v", g.Projection)
	}
}
