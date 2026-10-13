package importer

import (
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/geo"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestAreaResampleAveragesWhatItCovers(t *testing.T) {
	// 4x2 palette image: left half white, right half black -> 2x1 is white, black
	pal := color.Palette{color.RGBA{255, 255, 255, 255}, color.RGBA{0, 0, 0, 255}}
	src := image.NewPaletted(image.Rect(0, 0, 4, 2), pal)
	for y := 0; y < 2; y++ {
		for x := 2; x < 4; x++ {
			src.SetColorIndex(x, y, 1)
		}
	}
	got := areaResample(src, 2, 1)
	if got.RGBAAt(0, 0).R != 255 || got.RGBAAt(1, 0).R != 0 {
		t.Fatalf("2x1 = %v %v", got.RGBAAt(0, 0), got.RGBAAt(1, 0))
	}
	// one pixel of the average of a checkerboard is mid-grey
	chk := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
	chk.SetColorIndex(0, 0, 1)
	chk.SetColorIndex(1, 1, 1)
	if v := areaResample(chk, 1, 1).RGBAAt(0, 0).R; v < 126 || v > 129 {
		t.Fatalf("checkerboard mean = %d", v)
	}
	// a fractional ratio keeps the total (3x3 -> 2x2 of an all-white picture stays white)
	white := image.NewPaletted(image.Rect(0, 0, 3, 3), pal)
	for _, c := range [][2]int{{0, 0}, {1, 1}, {2, 2}} {
		_ = c
	}
	if v := areaResample(white, 2, 2).RGBAAt(1, 1).R; v != 255 {
		t.Fatalf("white stays white, got %d", v)
	}
}

func TestParseFGDCBoundsAndDate(t *testing.T) {
	htm := []byte(`<p>Publication_Date: 20260903</p><dl><dt>West_Bounding_Coordinate: -71.670333 <dt>East_Bounding_Coordinate: -70.312945
	<dt>North_Bounding_Coordinate: 42.800850 <dt>South_Bounding_Coordinate: 41.970733`)
	nl, ok := ParseFGDCBounds(htm)
	if !ok || nl.West != -71.670333 || nl.North != 42.800850 {
		t.Fatalf("bounds = %+v (%v)", nl, ok)
	}
	if d := ParseFGDCPublication(htm); d != "2026-09-03" {
		t.Fatalf("date = %q", d)
	}
}

// With the real FAA helicopter chart present (samples are never committed), import it and check the
// georeferencing against the chart itself: Boston Logan must land inside the map area and the
// neatline corners must come back where the metadata says.
func TestImportFAAHelicopterChartIfPresent(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "samples", "heli")
	tif, err := os.ReadFile(filepath.Join(dir, "Boston HEL.tif"))
	if err != nil {
		t.Skip("sample not present")
	}
	htm, _ := os.ReadFile(filepath.Join(dir, "Boston HEL.htm"))
	nl, ok := ParseFGDCBounds(htm)
	if !ok {
		t.Fatal("no bounds in the metadata")
	}
	res, err := ImportGeoTIFF(tif, Options{ID: "boston-hel", Source: "faa-us", Title: "BOSTON HEL", LongEdgePx: 3200})
	if err != nil {
		t.Fatal(err)
	}
	m := res.Meta
	// 10491 px is cut into a pyramid of 2 halvings: chart.png is the overview, 10492/4 = 2623 px wide
	if m.Image.Width != 2623 || m.Image.Height != 2173 || m.Tiles == nil || m.Tiles.MaxZoom != 2 || m.Tiles.Width != 10492 {
		t.Fatalf("image = %dx%d tiles = %+v", m.Image.Width, m.Image.Height, m.Tiles)
	}
	if len(res.Files) < 300 {
		t.Fatalf("only %d tiles", len(res.Files))
	}
	if len(res.PNG) < 100000 {
		t.Fatalf("PNG is only %d bytes", len(res.PNG))
	}
	// Boston Logan (KBOS): 42.3629 N, 71.0064 W -> inside the frame
	proj := mustProj(t, m)
	e, n := proj.Forward(42.3629, -71.0064)
	px := m.ToPixel.A*e + m.ToPixel.B*n + m.ToPixel.C
	py := m.ToPixel.D*e + m.ToPixel.E*n + m.ToPixel.F
	f := m.MapFramePx
	if f.Left < 250 || f.Left > 500 || f.Right < 1900 || f.Right > 2300 || f.Top > 150 || f.Bottom < 1900 {
		t.Fatalf("the map area %+v should be the central part of a 2623x%d picture, legends excluded", f, m.Image.Height)
	}
	if px < f.Left || px > f.Right || py < f.Top || py > f.Bottom {
		t.Fatalf("KBOS at (%.0f, %.0f) is outside the map area %+v", px, py, f)
	}
	// the corners of the map area are inside the metadata's extent of the whole file
	for _, c := range m.GeoCorners {
		if c.Lat > nl.North || c.Lat < nl.South || c.Lon < nl.West || c.Lon > nl.East {
			t.Fatalf("corner %+v outside the file extent %+v", c, nl)
		}
	}
	if m.Georef.MetersPerPixel < 40 || m.Georef.MetersPerPixel > 45 {
		t.Fatalf("metres per pixel = %v", m.Georef.MetersPerPixel)
	}
}

func mustProj(t *testing.T, m Meta) *geo.Projection {
	t.Helper()
	v := func(k string) float64 { f, _ := m.Projection[k].(float64); return f }
	if m.Projection["type"] != "lcc" {
		t.Fatalf("projection = %v", m.Projection)
	}
	return geo.NewLCC(v("a"), v("invF"), v("lat0"), v("lon0"), v("lat1"), v("lat2"), v("k0"), v("falseEasting"), v("falseNorthing"))
}
