package importer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func decodePNG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func TestPyramidLevelsPickTheClosestOverview(t *testing.T) {
	for _, c := range []struct{ w, h, target, want int }{
		{3000, 2000, 3200, 0},   // small enough as it is
		{10491, 8691, 3200, 2},  // 2623 px overview
		{14000, 10000, 3200, 2}, // 3500 px overview is closer to 3200 than 7000
		{25000, 18000, 3200, 3}, // 3125 px
		{3600, 2000, 3200, 0},   // within 20%: no pyramid
	} {
		if got := pyramidLevels(c.w, c.h, c.target); got != c.want {
			t.Errorf("pyramidLevels(%d, %d, %d) = %d, want %d", c.w, c.h, c.target, got, c.want)
		}
	}
}

func TestBuildPyramidCutsLevelsAndPadsWithWhite(t *testing.T) {
	// a 1201 x 701 palette picture: red left half, green right half, one blue pixel at the corner
	pal := color.Palette{color.RGBA{255, 255, 255, 255}, color.RGBA{255, 0, 0, 255}, color.RGBA{0, 160, 0, 255}, color.RGBA{0, 0, 255, 255}}
	src := image.NewPaletted(image.Rect(0, 0, 1201, 701), pal)
	for y := 0; y < 701; y++ {
		for x := 0; x < 1201; x++ {
			if x < 600 {
				src.SetColorIndex(x, y, 1)
			} else {
				src.SetColorIndex(x, y, 2)
			}
		}
	}
	src.SetColorIndex(1200, 700, 3)

	const m = 2
	overview, files, info, err := buildPyramid(src, m)
	if err != nil {
		t.Fatal(err)
	}
	// padded to a multiple of 4: 1204 x 704; the overview is exactly a quarter of it
	if info.Width != 1204 || info.Height != 704 || info.MaxZoom != m || info.Size != TileSize {
		t.Fatalf("info = %+v", info)
	}
	if overview.Bounds().Dx() != 301 || overview.Bounds().Dy() != 176 {
		t.Fatalf("overview = %v", overview.Bounds())
	}
	// finest level (zoom 2): 1204/512 -> 3 columns, 704/512 -> 2 rows
	if info.MinZoom != -1 { // 301 x 176 still fits in one tile at zoom 0, and the next level (151 x 88) is the last
		t.Logf("minZoom = %d", info.MinZoom)
	}
	for z, want := range map[int][2]int{2: {3, 2}, 1: {2, 1}, 0: {1, 1}} {
		for x := 0; x < want[0]; x++ {
			for y := 0; y < want[1]; y++ {
				if _, ok := files[fmt.Sprintf("tiles/%d/%d_%d.png", z, x, y)]; !ok {
					t.Errorf("missing tile z=%d (%d,%d)", z, x, y)
				}
			}
		}
		if _, ok := files[fmt.Sprintf("tiles/%d/%d_0.png", z, want[0])]; ok {
			t.Errorf("a tile beyond the picture exists at z=%d", z)
		}
	}
	// the finest tile (0,0) is the source, exactly (palette indexes, same palette)
	t0 := decodePNG(t, files["tiles/2/0_0.png"]).(*image.Paletted)
	t1 := decodePNG(t, files["tiles/2/1_0.png"]).(*image.Paletted)
	// the red/green boundary is at x = 600 = 512 + 88: red up to 87, green from 88, in tile (1,0)
	if t0.Bounds().Dx() != 512 || t0.ColorIndexAt(0, 0) != 1 || t0.ColorIndexAt(511, 0) != 1 ||
		t1.ColorIndexAt(87, 0) != 1 || t1.ColorIndexAt(88, 0) != 2 {
		t.Fatalf("finest tiles are not the source: %d %d | %d %d", t0.ColorIndexAt(0, 0), t0.ColorIndexAt(511, 0), t1.ColorIndexAt(87, 0), t1.ColorIndexAt(88, 0))
	}
	// the last tile holds the corner pixel and white padding: always a whole tile
	tl := decodePNG(t, files["tiles/2/2_1.png"]).(*image.Paletted)
	if tl.Bounds().Dx() != TileSize || tl.Bounds().Dy() != TileSize {
		t.Fatalf("last tile = %v, want a whole %d x %d tile", tl.Bounds(), TileSize, TileSize)
	}
	if tl.ColorIndexAt(1200-1024, 700-512) != 3 || tl.ColorIndexAt(1203-1024, 703-512) != 0 || tl.ColorIndexAt(500, 500) != 0 {
		t.Fatalf("corner/padding = %d %d %d", tl.ColorIndexAt(1200-1024, 700-512), tl.ColorIndexAt(1203-1024, 703-512), tl.ColorIndexAt(500, 500))
	}
	// a coarser level is the average: its left part is red-ish, its right part green-ish
	l0 := decodePNG(t, files["tiles/0/0_0.png"])
	r1, g1, _, _ := l0.At(10, 10).RGBA()
	r2, g2, _, _ := l0.At(290, 10).RGBA()
	if r1>>8 < 200 || g1>>8 > 60 || g2>>8 < 100 || r2>>8 > 60 {
		t.Fatalf("overview colours: left (%d,%d) right (%d,%d)", r1>>8, g1>>8, r2>>8, g2>>8)
	}
}
