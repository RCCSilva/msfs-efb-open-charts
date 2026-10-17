package geo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// the FAA helicopter chart of Boston: two standard parallels, centre 42.36667N 71W (NAD83)
func bostonLCC() *Projection {
	return NewLCC(6378137, 298.257222101, 42.36667, -71, 42.75, 42.08333, 1, 0, 0)
}

func TestLambertInverseReturnsWhatForwardWent(t *testing.T) {
	for _, c := range [][2]float64{{42.3601, -71.0589}, {42.8, -71.67}, {41.97, -70.31}, {42.36667, -71}} {
		t.Run("", func(t *testing.T) {
			// Arrange
			p := bostonLCC()
			x, y := p.Forward(c[0], c[1])

			// Act
			lat, lon, ok := p.Inverse(x, y)

			// Assert
			assert.True(t, ok)
			assert.InDelta(t, c[0], lat, 1e-8)
			assert.InDelta(t, c[1], lon, 1e-8)
		})
	}
}

func TestLambertOriginIsAtZero(t *testing.T) {
	// Arrange
	p := bostonLCC()

	// Act
	x, y := p.Forward(42.36667, -71)

	// Assert
	assert.InDelta(t, 0, x, 1e-6)
	assert.InDelta(t, 0, y, 1e-6)
}

func TestTransverseMercatorInverseReturnsWhatForwardWent(t *testing.T) {
	for _, c := range [][2]float64{{-23.627, -46.656}, {-22.9, -43.2}, {-30.0, -51.2}} {
		t.Run("", func(t *testing.T) {
			// Arrange
			p := NewTM(-45, 0.9996, 500000, 10000000) // UTM 23S
			x, y := p.Forward(c[0], c[1])

			// Act
			lat, lon, ok := p.Inverse(x, y)

			// Assert
			assert.True(t, ok)
			assert.InDelta(t, c[0], lat, 1e-7)
			assert.InDelta(t, c[1], lon, 1e-7)
		})
	}
}
