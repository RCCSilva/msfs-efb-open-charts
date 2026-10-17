package sim_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/simmock"
)

func TestCapabilitiesFollowWhatTheProviderFilledIn(t *testing.T) {
	cases := []struct {
		name     string
		provider sim.Provider
		want     sim.Capabilities
	}{
		{"nothing", sim.Provider{}, sim.Capabilities{}},
		{"only a position", sim.Provider{Position: sim.Fixed(0, 0, 0)}, sim.Capabilities{LivePosition: true}},
		{"everything", sim.Provider{
			Position: sim.Fixed(0, 0, 0), Nav: new(simmock.Nav), Seed: new(simmock.Seed), Plans: new(simmock.Plans),
		}, sim.Capabilities{LivePosition: true, AskNav: true, SeedFiles: true, LoadPlans: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange: the provider of the case

			// Act
			got := c.provider.Capabilities()

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}

func TestFixedPositionNeverMoves(t *testing.T) {
	// Arrange
	src := sim.Fixed(-23.6, -46.6, 45)

	// Act
	p := src.Position()

	// Assert
	assert.True(t, p.Valid)
	assert.True(t, p.Connected)
	assert.Equal(t, "fixed", p.Source)
	assert.InDelta(t, -23.6, p.Lat, 1e-9)
	assert.InDelta(t, 45, p.HeadingTrue, 1e-9)
}

func TestPositionFuncIsAPositionSource(t *testing.T) {
	// Arrange
	var src sim.PositionSource = sim.PositionFunc(func() sim.Position { return sim.Position{Lat: 1} })

	// Act
	p := src.Position()

	// Assert
	assert.Equal(t, 1.0, p.Lat)
}
