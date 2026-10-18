package importer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/geo"
)

// A page of 200 x 100 points whose corners are told apart: bottom-left (0,0), bottom-right (200,0),
// top-left (0,100), top-right (200,100).
var testBox = [4]float64{0, 0, 200, 100}

func TestPageViewKeepsAPageThatIsNotTurned(t *testing.T) {
	// Arrange
	pv, err := newPageView(testBox, 0)
	require.NoError(t, err)

	// Act
	u, v := pv.point(0, 100) // the top left corner

	// Assert
	assert.Equal(t, 200.0, pv.W)
	assert.Equal(t, 100.0, pv.H)
	assert.Equal(t, 0.0, u)
	assert.Equal(t, 0.0, v)
}

func TestPageViewTurnedBy90ShowsTheTopEdgeOnTheRight(t *testing.T) {
	// Arrange
	pv, err := newPageView(testBox, 90)
	require.NoError(t, err)

	// Act
	blU, blV := pv.point(0, 0)     // bottom left: becomes the top left
	tlU, tlV := pv.point(0, 100)   // top left: becomes the top right
	brU, brV := pv.point(200, 0)   // bottom right: becomes the bottom left
	trU, trV := pv.point(200, 100) // top right: becomes the bottom right

	// Assert
	assert.Equal(t, 100.0, pv.W, "the sides swap")
	assert.Equal(t, 200.0, pv.H, "the sides swap")
	assert.Equal(t, [2]float64{0, 0}, [2]float64{blU, blV})
	assert.Equal(t, [2]float64{100, 0}, [2]float64{tlU, tlV})
	assert.Equal(t, [2]float64{0, 200}, [2]float64{brU, brV})
	assert.Equal(t, [2]float64{100, 200}, [2]float64{trU, trV})
}

func TestPageViewTurnedBy180(t *testing.T) {
	// Arrange
	pv, err := newPageView(testBox, 180)
	require.NoError(t, err)

	// Act
	u, v := pv.point(0, 0) // bottom left: becomes the top right

	// Assert
	assert.Equal(t, 200.0, pv.W)
	assert.Equal(t, [2]float64{200, 0}, [2]float64{u, v})
}

func TestPageViewTurnedBy270ShowsTheTopEdgeOnTheLeft(t *testing.T) {
	// Arrange
	pv, err := newPageView(testBox, 270)
	require.NoError(t, err)

	// Act
	tlU, tlV := pv.point(0, 100) // top left: becomes the bottom left
	blU, blV := pv.point(0, 0)   // bottom left: becomes the bottom right

	// Assert
	assert.Equal(t, 100.0, pv.W)
	assert.Equal(t, 200.0, pv.H)
	assert.Equal(t, [2]float64{0, 200}, [2]float64{tlU, tlV})
	assert.Equal(t, [2]float64{100, 200}, [2]float64{blU, blV})
}

func TestPageViewRefusesARotationThatIsNotAMultipleOf90(t *testing.T) {
	// Act
	_, err := newPageView(testBox, 45)

	// Assert
	assert.Error(t, err)
}

// composing must agree with doing the steps one by one, for every rotation.
func TestPageViewComposeAgreesWithTheStepsOneByOne(t *testing.T) {
	for _, rot := range []int{0, 90, 180, 270} {
		t.Run("", func(t *testing.T) {
			// Arrange
			pv, err := newPageView(testBox, rot)
			require.NoError(t, err)
			aff := geo.Affine{A: 0.001, B: 0.0002, C: 0.1, D: -0.0003, E: 0.002, F: 0.2}
			vp := [4]float64{10, 20, 150, 60}
			sx, sy := 3.0, 2.5
			e, n := 120.0, 80.0

			// Act
			t2 := pv.compose(aff, vp, sx, sy)
			gotX := t2.A*e + t2.B*n + t2.C
			gotY := t2.D*e + t2.E*n + t2.F

			// Assert
			U := aff.A*e + aff.B*n + aff.C
			V := aff.D*e + aff.E*n + aff.F
			u, v := pv.point(vp[0]+vp[2]*U, vp[1]+vp[3]*V)
			assert.InDelta(t, u*sx, gotX, 1e-9)
			assert.InDelta(t, v*sy, gotY, 1e-9)
		})
	}
}

func TestCoarsePointsAreThoseWithAtMostThreeDecimals(t *testing.T) {
	assert.True(t, coarsePoints([]float64{-23.173, -43.26, -22.771}))
	assert.False(t, coarsePoints([]float64{-23.1734, -43.26}))
	assert.False(t, coarsePoints(nil))
}
