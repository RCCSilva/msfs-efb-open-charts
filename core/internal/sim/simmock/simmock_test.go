package simmock_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/simmock"
)

// the mocks are what the packages that take the interfaces are given
var (
	_ sim.NavSource        = (*simmock.Nav)(nil)
	_ sim.SeedSource       = (*simmock.Seed)(nil)
	_ sim.FlightPlanLoader = (*simmock.Plans)(nil)
	_ sim.PositionSource   = (*simmock.Position)(nil)
)

func TestNavReturnsWhatTheTestSaid(t *testing.T) {
	// Arrange
	nav := new(simmock.Nav)
	nav.On("Airport", "SBSP").Return(&navdata.Airport{ICAO: "SBSP"}, nil)

	// Act
	a, err := nav.Airport("SBSP")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "SBSP", a.ICAO)
	nav.AssertExpectations(t)
}

func TestNavAcceptsAPlainNilWithAnError(t *testing.T) {
	// Arrange
	boom := errors.New("boom")
	nav := new(simmock.Nav)
	nav.On("Airport", "SBSP").Return(nil, boom) // no cast of the nil
	nav.On("Airports", []string{"SBSP"}).Return(nil, boom)

	// Act
	a, err := nav.Airport("SBSP")
	list, err2 := nav.Airports([]string{"SBSP"})

	// Assert
	assert.Nil(t, a)
	assert.ErrorIs(t, err, boom)
	assert.Nil(t, list)
	assert.ErrorIs(t, err2, boom)
}
