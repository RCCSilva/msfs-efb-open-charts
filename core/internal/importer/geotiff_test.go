package importer

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/geo"
)

var blackAndWhite = color.Palette{color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255}}

func TestAreaResampleAveragesWhatItCovers(t *testing.T) {
	// Arrange: 4x2, left half white, right half black
	src := image.NewPaletted(image.Rect(0, 0, 4, 2), blackAndWhite)
	for y := 0; y < 2; y++ {
		for x := 2; x < 4; x++ {
			src.SetColorIndex(x, y, 1)
		}
	}

	// Act
	got := areaResample(src, 2, 1)

	// Assert
	assert.EqualValues(t, 255, got.RGBAAt(0, 0).R)
	assert.EqualValues(t, 0, got.RGBAAt(1, 0).R)
}

func TestAreaResampleOfACheckerboardIsMidGrey(t *testing.T) {
	// Arrange
	chk := image.NewPaletted(image.Rect(0, 0, 2, 2), blackAndWhite)
	chk.SetColorIndex(0, 0, 1)
	chk.SetColorIndex(1, 1, 1)

	// Act
	got := areaResample(chk, 1, 1).RGBAAt(0, 0).R

	// Assert
	assert.InDelta(t, 127.5, float64(got), 1.5)
}

func TestAreaResampleWithAFractionalRatioKeepsWhiteWhite(t *testing.T) {
	// Arrange
	white := image.NewPaletted(image.Rect(0, 0, 3, 3), blackAndWhite)

	// Act
	got := areaResample(white, 2, 2).RGBAAt(1, 1).R

	// Assert
	assert.EqualValues(t, 255, got)
}

const fgdc = `<p>Publication_Date: 20260903</p><dl><dt>West_Bounding_Coordinate: -71.670333 <dt>East_Bounding_Coordinate: -70.312945
	<dt>North_Bounding_Coordinate: 42.800850 <dt>South_Bounding_Coordinate: 41.970733`

func TestParseFGDCBounds(t *testing.T) {
	// Act
	nl, ok := ParseFGDCBounds([]byte(fgdc))

	// Assert
	require.True(t, ok)
	assert.Equal(t, -71.670333, nl.West)
	assert.Equal(t, 42.800850, nl.North)
}

func TestParseFGDCPublicationDate(t *testing.T) {
	// Act
	d := ParseFGDCPublication([]byte(fgdc))

	// Assert
	assert.Equal(t, "2026-09-03", d)
}

// With the real FAA helicopter chart present (samples are never committed), import it and check the
// georeferencing against the chart itself: Boston Logan must land inside the map area and the
// neatline corners must come back where the metadata says.
func importBostonHeli(t *testing.T) (*Result, Neatline) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "samples", "heli")
	tif, err := os.ReadFile(filepath.Join(dir, "Boston HEL.tif"))
	if err != nil {
		t.Skip("sample not present")
	}
	htm, _ := os.ReadFile(filepath.Join(dir, "Boston HEL.htm"))
	nl, ok := ParseFGDCBounds(htm)
	require.True(t, ok, "no bounds in the metadata")
	res, err := ImportGeoTIFF(tif, Options{ID: "boston-hel", Source: "faa-us", Title: "BOSTON HEL", LongEdgePx: 3200})
	require.NoError(t, err)
	return res, nl
}

func TestImportFAAHelicopterChartIsCutIntoAPyramid(t *testing.T) {
	// Arrange / Act
	res, _ := importBostonHeli(t)

	// Assert
	m := res.Meta
	// 10491 px is cut into a pyramid of 2 halvings: chart.png is the overview, 10492/4 = 2623 px wide
	assert.Equal(t, 2623, m.Image.Width)
	assert.Equal(t, 2173, m.Image.Height)
	require.NotNil(t, m.Tiles)
	assert.Equal(t, 2, m.Tiles.MaxZoom)
	assert.Equal(t, 10492, m.Tiles.Width)
	assert.GreaterOrEqual(t, len(res.Files), 300, "tiles")
	assert.GreaterOrEqual(t, len(res.PNG), 100000, "PNG bytes")
}

func TestImportFAAHelicopterChartPutsBostonLoganInsideTheMapArea(t *testing.T) {
	// Arrange
	res, _ := importBostonHeli(t)
	m := res.Meta
	proj := mustProj(t, m)

	// Act: Boston Logan (KBOS) is at 42.3629 N, 71.0064 W
	e, n := proj.Forward(42.3629, -71.0064)
	px := m.ToPixel.A*e + m.ToPixel.B*n + m.ToPixel.C
	py := m.ToPixel.D*e + m.ToPixel.E*n + m.ToPixel.F

	// Assert
	f := m.MapFramePx
	assert.True(t, f.Left >= 250 && f.Left <= 500 && f.Right >= 1900 && f.Right <= 2300 && f.Top <= 150 && f.Bottom >= 1900,
		"the map area %+v should be the central part of the picture, legends excluded", f)
	assert.True(t, px >= f.Left && px <= f.Right && py >= f.Top && py <= f.Bottom,
		"KBOS at (%.0f, %.0f) is outside the map area %+v", px, py, f)
}

func TestImportFAAHelicopterChartKeepsTheCornersInsideTheFileExtent(t *testing.T) {
	// Arrange / Act
	res, nl := importBostonHeli(t)

	// Assert
	for _, c := range res.Meta.GeoCorners {
		assert.True(t, c.Lat <= nl.North && c.Lat >= nl.South && c.Lon >= nl.West && c.Lon <= nl.East,
			"corner %+v outside the file extent %+v", c, nl)
	}
	assert.InDelta(t, 42.5, res.Meta.Georef.MetersPerPixel, 2.5)
}

func mustProj(t *testing.T, m Meta) *geo.Projection {
	t.Helper()
	v := func(k string) float64 { f, _ := m.Projection[k].(float64); return f }
	require.Equal(t, "lcc", m.Projection["type"], "projection %v", m.Projection)
	return geo.NewLCC(v("a"), v("invF"), v("lat0"), v("lon0"), v("lat1"), v("lat2"), v("k0"), v("falseEasting"), v("falseNorthing"))
}
