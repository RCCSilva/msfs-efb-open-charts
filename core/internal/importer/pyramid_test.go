package importer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func decodePNG(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	require.NoError(t, err)
	return img
}

func TestPyramidLevelsPickTheClosestOverview(t *testing.T) {
	cases := []struct {
		name               string
		w, h, target, want int
	}{
		{"small enough as it is", 3000, 2000, 3200, 0},
		{"2623 px overview", 10491, 8691, 3200, 2},
		{"3500 px overview is closer to 3200 than 7000", 14000, 10000, 3200, 2},
		{"3125 px", 25000, 18000, 3200, 3},
		{"within 20%: no pyramid", 3600, 2000, 3200, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Act
			got := pyramidLevels(c.w, c.h, c.target)

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}

// A 1201 x 701 palette picture: red left half, green right half, one blue pixel at the corner.
func redGreenPicture() *image.Paletted {
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
	return src
}

const halvings = 2

func TestBuildPyramidPadsToAMultipleOfTheLevels(t *testing.T) {
	// Arrange
	src := redGreenPicture()

	// Act
	overview, _, info, err := buildPyramid(src, halvings)

	// Assert
	require.NoError(t, err)
	// padded to a multiple of 4: 1204 x 704; the overview is exactly a quarter of it
	assert.Equal(t, 1204, info.Width)
	assert.Equal(t, 704, info.Height)
	assert.Equal(t, halvings, info.MaxZoom)
	assert.Equal(t, TileSize, info.Size)
	assert.Equal(t, 301, overview.Bounds().Dx())
	assert.Equal(t, 176, overview.Bounds().Dy())
}

func TestBuildPyramidMakesEveryTileOfEveryLevelAndNoMore(t *testing.T) {
	// Arrange
	src := redGreenPicture()

	// Act
	_, files, _, err := buildPyramid(src, halvings)

	// Assert
	require.NoError(t, err)
	// finest level (zoom 2): 1204/512 -> 3 columns, 704/512 -> 2 rows
	for z, want := range map[int][2]int{2: {3, 2}, 1: {2, 1}, 0: {1, 1}} {
		for x := 0; x < want[0]; x++ {
			for y := 0; y < want[1]; y++ {
				assert.Contains(t, files, fmt.Sprintf("tiles/%d/%d_%d.png", z, x, y), "missing tile z=%d (%d,%d)", z, x, y)
			}
		}
		assert.NotContains(t, files, fmt.Sprintf("tiles/%d/%d_0.png", z, want[0]), "a tile beyond the picture exists at z=%d", z)
	}
}

func TestBuildPyramidFinestTilesAreTheSource(t *testing.T) {
	// Arrange
	src := redGreenPicture()

	// Act
	_, files, _, err := buildPyramid(src, halvings)

	// Assert
	require.NoError(t, err)
	t0 := decodePNG(t, files["tiles/2/0_0.png"]).(*image.Paletted)
	t1 := decodePNG(t, files["tiles/2/1_0.png"]).(*image.Paletted)
	// the red/green boundary is at x = 600 = 512 + 88: red up to 87, green from 88, in tile (1,0)
	assert.Equal(t, 512, t0.Bounds().Dx())
	assert.EqualValues(t, 1, t0.ColorIndexAt(0, 0))
	assert.EqualValues(t, 1, t0.ColorIndexAt(511, 0))
	assert.EqualValues(t, 1, t1.ColorIndexAt(87, 0))
	assert.EqualValues(t, 2, t1.ColorIndexAt(88, 0))
}

func TestBuildPyramidLastTileIsWholeAndPaddedWithWhite(t *testing.T) {
	// Arrange
	src := redGreenPicture()

	// Act
	_, files, _, err := buildPyramid(src, halvings)

	// Assert
	require.NoError(t, err)
	tl := decodePNG(t, files["tiles/2/2_1.png"]).(*image.Paletted)
	assert.Equal(t, TileSize, tl.Bounds().Dx())
	assert.Equal(t, TileSize, tl.Bounds().Dy())
	assert.EqualValues(t, 3, tl.ColorIndexAt(1200-1024, 700-512), "the corner pixel")
	assert.EqualValues(t, 0, tl.ColorIndexAt(1203-1024, 703-512), "padding")
	assert.EqualValues(t, 0, tl.ColorIndexAt(500, 500), "padding")
}

func TestBuildPyramidCoarserLevelIsTheAverage(t *testing.T) {
	// Arrange
	src := redGreenPicture()

	// Act
	_, files, _, err := buildPyramid(src, halvings)

	// Assert
	require.NoError(t, err)
	l0 := decodePNG(t, files["tiles/0/0_0.png"])
	r1, g1, _, _ := l0.At(10, 10).RGBA()
	r2, g2, _, _ := l0.At(290, 10).RGBA()
	assert.Greater(t, r1>>8, uint32(200), "left part is red-ish")
	assert.Less(t, g1>>8, uint32(60), "left part is red-ish")
	assert.Greater(t, g2>>8, uint32(100), "right part is green-ish")
	assert.Less(t, r2>>8, uint32(60), "right part is green-ish")
}
