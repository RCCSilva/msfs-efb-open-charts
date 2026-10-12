// Package geotiff reads the georeferencing of a GeoTIFF and decodes its picture.
//
// A GeoTIFF is an ordinary TIFF with extra tags that say where the picture is on the Earth:
// ModelPixelScale and ModelTiepoint place the pixels, and the GeoKeyDirectory (with the
// GeoDoubleParams and GeoAsciiParams it points to) names the projection. This package reads
// those, builds a geo.Projection from them and decodes the picture itself (decode.go: LZW, Deflate,
// PackBits, palettes). It does what the FAA's VFR and IFR rasters need, and it refuses what it
// does not understand instead of guessing.
package geotiff

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/geo"
)

// Tag numbers.
const (
	tagImageWidth       = 256
	tagImageLength      = 257
	tagModelPixelScale  = 33550
	tagModelTiepoint    = 33922
	tagModelTransform   = 34264
	tagGeoKeyDirectory  = 34735
	tagGeoDoubleParams  = 34736
	tagGeoAsciiParams   = 34737
	maxTagBytes         = 1 << 20
	maxDirectoryEntries = 512
)

// Georef is the georeferencing of a GeoTIFF: how pixel (column, row) maps to projected metres (or
// degrees for a geographic raster), and the projection.
type Georef struct {
	Width, Height int
	// E = X0 + col*DX ; N = Y0 + row*DY (DY is negative for a north-up picture).
	X0, Y0, DX, DY float64
	Projection     *geo.Projection
	// Datum names the horizontal datum when the file says it ("NAD83"), for the record.
	Datum string
}

// Errors the caller can report.
var (
	ErrNotTIFF         = errors.New("not a TIFF file")
	ErrNotGeoreference = errors.New("this TIFF has no georeferencing (no GeoTIFF tags)")
)

type entry struct {
	typ    uint16
	count  uint32
	offset uint32 // the value itself when it fits in 4 bytes
	raw    [4]byte
}

type reader struct {
	r  io.ReaderAt
	bo binary.ByteOrder
}

// IsTIFF reports whether data starts like a classic TIFF.
func IsTIFF(data []byte) bool {
	return len(data) >= 4 && (bytes.Equal(data[:4], []byte{'I', 'I', 42, 0}) || bytes.Equal(data[:4], []byte{'M', 'M', 0, 42}))
}

// ReadGeoref reads the georeferencing without decoding the picture.
func ReadGeoref(data []byte) (*Georef, error) {
	if !IsTIFF(data) {
		if len(data) >= 4 && (bytes.Equal(data[:4], []byte{'I', 'I', 43, 0}) || bytes.Equal(data[:4], []byte{'M', 'M', 0, 43})) {
			return nil, errors.New("BigTIFF is not supported")
		}
		return nil, ErrNotTIFF
	}
	rd := &reader{r: bytes.NewReader(data)}
	if data[0] == 'I' {
		rd.bo = binary.LittleEndian
	} else {
		rd.bo = binary.BigEndian
	}
	ifd := rd.bo.Uint32(data[4:8])
	tags, err := rd.readIFD(int64(ifd))
	if err != nil {
		return nil, err
	}

	g := &Georef{}
	if w, ok := rd.uint(tags[tagImageWidth]); ok {
		g.Width = int(w)
	}
	if h, ok := rd.uint(tags[tagImageLength]); ok {
		g.Height = int(h)
	}
	if g.Width <= 0 || g.Height <= 0 {
		return nil, errors.New("TIFF has no size")
	}

	scale, hasScale := rd.doubles(tags[tagModelPixelScale])
	tie, hasTie := rd.doubles(tags[tagModelTiepoint])
	trans, hasTrans := rd.doubles(tags[tagModelTransform])
	switch {
	case hasScale && hasTie && len(scale) >= 2 && len(tie) >= 6:
		// the tiepoint (I, J) -> (X, Y); pixels are PixelIsArea by default
		g.DX, g.DY = scale[0], -scale[1]
		g.X0 = tie[3] - tie[0]*g.DX
		g.Y0 = tie[4] - tie[1]*g.DY
	case hasTrans && len(trans) >= 16:
		if trans[1] != 0 || trans[4] != 0 {
			return nil, errors.New("rotated GeoTIFFs are not supported")
		}
		g.DX, g.DY, g.X0, g.Y0 = trans[0], trans[5], trans[3], trans[7]
	default:
		return nil, ErrNotGeoreference
	}

	proj, datum, err := rd.projection(tags)
	if err != nil {
		return nil, err
	}
	g.Projection, g.Datum = proj, datum
	// PixelIsPoint rasters put the tiepoint at the pixel centre; shift to the corner.
	if keys, ok := rd.geoKeys(tags); ok && keys[1025] == 2 {
		g.X0 -= g.DX / 2
		g.Y0 -= g.DY / 2
	}
	return g, nil
}

// PixelToProjected returns the projected coordinates of the top-left corner of a pixel.
func (g *Georef) PixelToProjected(col, row float64) (e, n float64) {
	return g.X0 + col*g.DX, g.Y0 + row*g.DY
}

// ProjectedToPixel is the inverse of PixelToProjected.
func (g *Georef) ProjectedToPixel(e, n float64) (col, row float64) {
	return (e - g.X0) / g.DX, (n - g.Y0) / g.DY
}

// MetersPerPixel is the ground size of a pixel, in metres (for a geographic raster, at its centre).
func (g *Georef) MetersPerPixel() float64 {
	if g.Projection.Kind == geo.KindGeographic {
		midLat := g.Y0 + g.DY*float64(g.Height)/2
		return math.Abs(g.DX) * 111320 * math.Cos(midLat*math.Pi/180)
	}
	return (math.Abs(g.DX) + math.Abs(g.DY)) / 2
}

// ---------------------------------------------------------------- TIFF structure

func (rd *reader) readIFD(off int64) (map[uint16]entry, error) {
	var cnt [2]byte
	if _, err := rd.r.ReadAt(cnt[:], off); err != nil {
		return nil, fmt.Errorf("read IFD: %w", err)
	}
	n := int(rd.bo.Uint16(cnt[:]))
	if n == 0 || n > maxDirectoryEntries {
		return nil, errors.New("TIFF directory looks wrong")
	}
	buf := make([]byte, n*12)
	if _, err := rd.r.ReadAt(buf, off+2); err != nil {
		return nil, fmt.Errorf("read IFD entries: %w", err)
	}
	tags := make(map[uint16]entry, n)
	for i := 0; i < n; i++ {
		b := buf[i*12 : i*12+12]
		e := entry{typ: rd.bo.Uint16(b[2:4]), count: rd.bo.Uint32(b[4:8]), offset: rd.bo.Uint32(b[8:12])}
		copy(e.raw[:], b[8:12])
		tags[rd.bo.Uint16(b[0:2])] = e
	}
	return tags, nil
}

func typeSize(t uint16) int {
	switch t {
	case 1, 2, 6, 7: // BYTE, ASCII, SBYTE, UNDEFINED
		return 1
	case 3, 8: // SHORT, SSHORT
		return 2
	case 4, 9, 11: // LONG, SLONG, FLOAT
		return 4
	case 5, 10, 12: // RATIONAL, SRATIONAL, DOUBLE
		return 8
	}
	return 0
}

// data returns the bytes of a tag's value.
func (rd *reader) data(e entry) ([]byte, bool) {
	sz := typeSize(e.typ) * int(e.count)
	if sz <= 0 || sz > maxTagBytes {
		return nil, false
	}
	if sz <= 4 {
		return append([]byte(nil), e.raw[:sz]...), true
	}
	buf := make([]byte, sz)
	if _, err := rd.r.ReadAt(buf, int64(e.offset)); err != nil {
		return nil, false
	}
	return buf, true
}

func (rd *reader) uint(e entry) (uint32, bool) {
	if e.count == 0 {
		return 0, false
	}
	b, ok := rd.data(entry{typ: e.typ, count: 1, offset: e.offset, raw: e.raw})
	if !ok {
		return 0, false
	}
	switch e.typ {
	case 3:
		return uint32(rd.bo.Uint16(b)), true
	case 4:
		return rd.bo.Uint32(b), true
	}
	return 0, false
}

func (rd *reader) doubles(e entry) ([]float64, bool) {
	if e.typ != 12 || e.count == 0 {
		return nil, false
	}
	b, ok := rd.data(e)
	if !ok {
		return nil, false
	}
	out := make([]float64, e.count)
	for i := range out {
		out[i] = math.Float64frombits(rd.bo.Uint64(b[i*8:]))
	}
	return out, true
}

func (rd *reader) shorts(e entry) ([]uint16, bool) {
	if e.typ != 3 || e.count == 0 {
		return nil, false
	}
	b, ok := rd.data(e)
	if !ok {
		return nil, false
	}
	out := make([]uint16, e.count)
	for i := range out {
		out[i] = rd.bo.Uint16(b[i*2:])
	}
	return out, true
}

// ---------------------------------------------------------------- GeoKeys

// geoKeys returns the keys with a numeric value (SHORT keys stored inline).
func (rd *reader) geoKeys(tags map[uint16]entry) (map[uint16]uint16, bool) {
	dir, ok := rd.shorts(tags[tagGeoKeyDirectory])
	if !ok || len(dir) < 4 {
		return nil, false
	}
	n := int(dir[3])
	keys := map[uint16]uint16{}
	for i := 0; i < n && 4+i*4+3 < len(dir); i++ {
		k := dir[4+i*4:]
		if k[1] == 0 { // value stored in the entry itself
			keys[k[0]] = k[3]
		}
	}
	return keys, true
}

// geoDouble reads a key whose value is in GeoDoubleParams.
func (rd *reader) geoDouble(tags map[uint16]entry, key uint16) (float64, bool) {
	dir, ok := rd.shorts(tags[tagGeoKeyDirectory])
	if !ok || len(dir) < 4 {
		return 0, false
	}
	params, ok := rd.doubles(tags[tagGeoDoubleParams])
	if !ok {
		return 0, false
	}
	n := int(dir[3])
	for i := 0; i < n && 4+i*4+3 < len(dir); i++ {
		k := dir[4+i*4:]
		if k[0] == key && k[1] == tagGeoDoubleParams && int(k[3]) < len(params) {
			return params[k[3]], true
		}
	}
	return 0, false
}

// Common GeoKey ids.
const (
	keyModelType    = 1024
	keyGeographic   = 2048
	keyEllipsoid    = 2056
	keySemiMajor    = 2057
	keyInvFlat      = 2059
	keyProjectedCS  = 3072
	keyCoordTrans   = 3075
	keyStdParallel1 = 3078
	keyStdParallel2 = 3079
	keyNatOriginLon = 3080
	keyNatOriginLat = 3081
	keyFalseEasting = 3082
	keyFalseNorth   = 3083
	keyFalseOrigLon = 3084 // Lambert 2SP: the false origin
	keyFalseOrigLat = 3085
	keyFalseOrigE   = 3086
	keyFalseOrigN   = 3087
	keyScaleNatOrig = 3092
)

type ellipsoid struct{ a, invF float64 }

// ellipsoids by EPSG code (GeogEllipsoidGeoKey).
var ellipsoids = map[uint16]ellipsoid{
	7030: {6378137, 298.257223563},   // WGS 84
	7019: {6378137, 298.257222101},   // GRS 1980 (NAD83)
	7008: {6378206.4, 294.978698214}, // Clarke 1866 (NAD27)
	7022: {6378388, 297},             // International 1924
}

// datumNames by EPSG geographic CS, for the record.
var datumNames = map[uint16]string{4326: "WGS84", 4269: "NAD83", 4267: "NAD27", 4674: "SIRGAS 2000", 4618: "SAD69"}

func (rd *reader) projection(tags map[uint16]entry) (*geo.Projection, string, error) {
	keys, ok := rd.geoKeys(tags)
	if !ok {
		return nil, "", ErrNotGeoreference
	}
	datum := datumNames[keys[keyGeographic]]
	el, known := ellipsoids[keys[keyEllipsoid]]
	if !known {
		el = ellipsoid{6378137, 298.257222101} // GRS80/WGS84 class: sub-metre for these charts
		if a, ok := rd.geoDouble(tags, keySemiMajor); ok {
			el.a = a
			if f, ok := rd.geoDouble(tags, keyInvFlat); ok {
				el.invF = f
			}
		}
	}
	if keys[keyModelType] == 2 { // geographic raster: degrees
		return geo.NewGeographic(), datum, nil
	}
	// UTM zones by EPSG code (WGS84 326xx/327xx, and NAD83 269xx)
	if cs := keys[keyProjectedCS]; cs != 32767 && cs != 0 {
		switch {
		case cs >= 32601 && cs <= 32660:
			return geo.NewTM(float64(cs-32600)*6-183, 0.9996, 500000, 0), "WGS84", nil
		case cs >= 32701 && cs <= 32760:
			return geo.NewTM(float64(cs-32700)*6-183, 0.9996, 500000, 10000000), "WGS84", nil
		case cs >= 26901 && cs <= 26923:
			return geo.NewTM(float64(cs-26900)*6-183, 0.9996, 500000, 0), "NAD83", nil
		}
		return nil, "", fmt.Errorf("projected CS EPSG:%d is not supported (supported: UTM, Lambert Conformal Conic by parameters)", cs)
	}
	get := func(k uint16) float64 { v, _ := rd.geoDouble(tags, k); return v }
	switch keys[keyCoordTrans] {
	case 8, 9: // Lambert Conformal Conic 2SP / 1SP
		p1 := get(keyStdParallel1)
		p2, has2 := rd.geoDouble(tags, keyStdParallel2)
		if !has2 {
			p2 = p1
		}
		// the 2SP form names its origin with the "false origin" keys; the 1SP form with the natural origin
		pick := func(primary, fallback uint16) float64 {
			if v, ok := rd.geoDouble(tags, primary); ok {
				return v
			}
			return get(fallback)
		}
		lat0 := pick(keyFalseOrigLat, keyNatOriginLat)
		lon0 := pick(keyFalseOrigLon, keyNatOriginLon)
		fe := pick(keyFalseOrigE, keyFalseEasting)
		fn := pick(keyFalseOrigN, keyFalseNorth)
		if keys[keyCoordTrans] == 9 { // one standard parallel: the origin latitude
			p1, p2 = lat0, lat0
		}
		k0 := 1.0
		if v, ok := rd.geoDouble(tags, keyScaleNatOrig); ok && keys[keyCoordTrans] == 9 {
			k0 = v
		}
		return geo.NewLCC(el.a, el.invF, lat0, lon0, p1, p2, k0, fe, fn), datum, nil
	case 1: // Transverse Mercator
		k0, ok := rd.geoDouble(tags, keyScaleNatOrig)
		if !ok {
			k0 = 1
		}
		return geo.NewTM(get(keyNatOriginLon), k0, get(keyFalseEasting), get(keyFalseNorth)), datum, nil
	}
	return nil, "", fmt.Errorf("GeoTIFF coordinate transformation %d is not supported (supported: Lambert Conformal Conic, Transverse Mercator/UTM, geographic)", keys[keyCoordTrans])
}
