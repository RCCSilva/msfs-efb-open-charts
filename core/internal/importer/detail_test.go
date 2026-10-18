package importer

import (
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetailPlanPadsTheFinestLevelToAMultipleOfTheHalvings(t *testing.T) {
	// Act: a page of 1000 x 700 points at 25 pixels per point is 25,000 x 17,500 pixels
	m, w0, h0 := detailPlan(1000, 700, 25, 3200)

	// Assert
	assert.Equal(t, 3, m, "three halvings bring 25,000 to about 3,200")
	assert.Zero(t, w0%(1<<m))
	assert.Zero(t, h0%(1<<m))
	assert.GreaterOrEqual(t, w0, 25000)
	assert.Less(t, w0, 25000+(1<<m))
}

func TestDetailPlanOfASmallPageNeedsNoHalvings(t *testing.T) {
	// Act
	m, w0, h0 := detailPlan(100, 100, 10, 3200)

	// Assert
	assert.Zero(t, m)
	assert.Equal(t, 1000, w0)
	assert.Equal(t, 1000, h0)
}

func TestDetailScaleLeavesAPageThatFitsAlone(t *testing.T) {
	assert.Equal(t, 10.0, detailScale(100, 100, 10))
}

func TestDetailScaleShrinksAPageThatWouldHaveTooManyPixels(t *testing.T) {
	// Act: 4000 x 3000 points at 20 pixels per point is 80,000 x 60,000 = 4.8 billion pixels
	s := detailScale(4000, 3000, 20)

	// Assert
	px := 4000 * s * 3000 * s
	assert.InDelta(t, float64(MaxDetailPixels), px, float64(MaxDetailPixels)*0.001)
	assert.Less(t, s, 20.0)
}

func TestWhiteTileCompletesARegionThatCameOutSmallerAndMakesItOpaque(t *testing.T) {
	// Arrange: a region of 10 x 10 that is fully transparent black
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))

	// Act
	tile := whiteTile(src)

	// Assert
	b := tile.Bounds()
	assert.Equal(t, TileSize, b.Dx())
	assert.Equal(t, TileSize, b.Dy())
	assert.Equal(t, uint8(255), tile.Pix[3], "the copied pixel is opaque")
	last := len(tile.Pix) - 4
	assert.Equal(t, []uint8{255, 255, 255, 255}, tile.Pix[last:last+4], "beyond the region it is white")
}

func TestCopyIntoCutsATileAtThePicturesEdge(t *testing.T) {
	// Arrange
	dst := image.NewRGBA(image.Rect(0, 0, 600, 600))
	tile := whiteTile(image.NewRGBA(image.Rect(0, 0, TileSize, TileSize)))

	// Act
	copyInto(dst, tile, TileSize, TileSize) // only 88 x 88 of the tile fits

	// Assert
	assert.Equal(t, uint8(255), dst.Pix[(TileSize*dst.Stride)+TileSize*4+3], "inside: the tile is opaque")
	assert.Zero(t, dst.Pix[(599*dst.Stride)+3], "outside the copied corner stays untouched")
}
