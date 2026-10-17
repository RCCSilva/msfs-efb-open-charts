package navdb

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// withFixes is a database with the sample airport and the two waypoints of its departure.
func withFixes(t *testing.T) *DB {
	t.Helper()
	d := openTemp(t)
	require.NoError(t, d.SaveAirport(sample()))
	require.NoError(t, d.SaveFixes([]navdata.Fix{
		{Ident: "BOBIC", Region: "SB", Kind: 'W', Lat: -23.1, Lon: -45.9, Routes: []navdata.Route{
			{Airway: "UZ6", Type: 1, NextIdent: "VUPSA", NextRegion: "SB", NextKind: 'W', NextLat: -22.9, NextLon: -45.5},
		}},
		{Ident: "VUPSA", Region: "SB", Kind: 'W', Lat: -22.9, Lon: -45.5},
	}))
	return d
}

func TestUncheckedIdentsAreTheWaypointsNotTheRunways(t *testing.T) {
	// Arrange
	d := openTemp(t)
	require.NoError(t, d.SaveAirport(sample()))

	// Act
	missing, err := d.UncheckedIdents("SBSJ")

	// Assert
	require.NoError(t, err)
	// BOBIC and VUPSA are waypoints; RW34 is a runway and is placed from the airport, not asked for
	assert.ElementsMatch(t, []string{"BOBIC", "VUPSA"}, missing)
}

func TestACheckedIdentIsNoLongerPending(t *testing.T) {
	// Arrange
	d := withFixes(t)

	// Act
	for _, id := range []string{"BOBIC", "VUPSA"} {
		require.NoError(t, d.MarkIdentChecked(id, 1))
	}

	// Assert
	missing, err := d.UncheckedIdents("SBSJ")
	require.NoError(t, err)
	assert.Empty(t, missing)
	ok, _ := d.IdentChecked("bobic")
	assert.True(t, ok, "a checked identifier must be remembered, in any case")
}

func TestFixesByIdentGivesTheRegion(t *testing.T) {
	// Arrange
	d := withFixes(t)

	// Act
	got, err := d.FixesByIdent("BOBIC")

	// Assert
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "SB", got[0].Region)
}

func TestEveryLegOfTheProcedureGetsAPosition(t *testing.T) {
	// Arrange
	d := withFixes(t)

	// Act
	a, err := d.Airport("SBSJ")

	// Assert
	require.NoError(t, err)
	for _, tr := range a.Procedures[0].Transitions {
		for _, l := range tr.Legs {
			assert.NotNil(t, l.Lat, "leg %s without a position", l.FixICAO)
			assert.NotNil(t, l.Lon, "leg %s without a position", l.FixICAO)
		}
	}
}

func TestAWaypointLegIsPlacedAtTheStoredFix(t *testing.T) {
	// Arrange
	d := withFixes(t)

	// Act
	a, err := d.Airport("SBSJ")

	// Assert
	require.NoError(t, err)
	bobic := a.Procedures[0].Transitions[0].Legs[1]
	require.NotNil(t, bobic.Lat)
	assert.Equal(t, -23.1, *bobic.Lat)
	assert.Equal(t, -45.9, *bobic.Lon)
}

func TestARunwayLegIsPlacedHalfARunwayBehindTheMiddle(t *testing.T) {
	// Arrange
	d := withFixes(t)

	// Act
	a, err := d.Airport("SBSJ")

	// Assert
	require.NoError(t, err)
	rw := a.Procedures[0].Transitions[0].Legs[0]
	mid := a.Runways[0]
	dLat := (*rw.Lat - mid.Lat) * 111320
	dLon := (*rw.Lon - mid.Lon) * 111320 * math.Cos(mid.Lat*math.Pi/180)
	assert.InDelta(t, mid.LengthM/2, math.Hypot(dLat, dLon), 5, "RW34 is the primary end")
	// heading 315: the primary threshold is to the south-east of the middle
	assert.Less(t, dLat, 0.0)
	assert.Greater(t, dLon, 0.0)
}

func TestAFixCarriesItsAirways(t *testing.T) {
	// Arrange
	d := withFixes(t)

	// Act
	f, err := d.Fix("bobic", "sb")

	// Assert
	require.NoError(t, err)
	require.Len(t, f.Routes, 1)
	assert.Equal(t, "UZ6", f.Routes[0].Airway)
	assert.Equal(t, "VUPSA", f.Routes[0].NextIdent)
}
