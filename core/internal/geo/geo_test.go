package geo

import (
	"math"
	"testing"
)

func TestLambertInverseReturnsWhatForwardWent(t *testing.T) {
	// the FAA helicopter chart of Boston: two standard parallels, centre 42.36667N 71W (NAD83)
	p := NewLCC(6378137, 298.257222101, 42.36667, -71, 42.75, 42.08333, 1, 0, 0)
	for _, c := range [][2]float64{{42.3601, -71.0589}, {42.8, -71.67}, {41.97, -70.31}, {42.36667, -71}} {
		x, y := p.Forward(c[0], c[1])
		lat, lon, ok := p.Inverse(x, y)
		if !ok || math.Abs(lat-c[0]) > 1e-8 || math.Abs(lon-c[1]) > 1e-8 {
			t.Errorf("(%v) -> (%.3f, %.3f) -> (%.8f, %.8f)", c, x, y, lat, lon)
		}
	}
	// the origin of the projection is at (0, 0)
	if x, y := p.Forward(42.36667, -71); math.Abs(x) > 1e-6 || math.Abs(y) > 1e-6 {
		t.Errorf("origin = %.6f, %.6f", x, y)
	}
}

func TestTransverseMercatorInverseReturnsWhatForwardWent(t *testing.T) {
	p := NewTM(-45, 0.9996, 500000, 10000000) // UTM 23S
	for _, c := range [][2]float64{{-23.627, -46.656}, {-22.9, -43.2}, {-30.0, -51.2}} {
		x, y := p.Forward(c[0], c[1])
		lat, lon, ok := p.Inverse(x, y)
		if !ok || math.Abs(lat-c[0]) > 1e-7 || math.Abs(lon-c[1]) > 1e-7 {
			t.Errorf("(%v) -> (%.3f, %.3f) -> (%.8f, %.8f)", c, x, y, lat, lon)
		}
	}
}
