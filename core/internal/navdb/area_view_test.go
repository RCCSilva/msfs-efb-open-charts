package navdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// worldDB has an airport in Brazil (-47), one in Fiji (178, near the antimeridian) and one in
// Samoa (-172, on the other side of it), and an airway stretch across the antimeridian.
func worldDB(t *testing.T) *DB {
	t.Helper()
	d := openTemp(t)
	require.NoError(t, d.SaveAirport(&navdata.Airport{ICAO: "SBSJ", Name: "Sao Jose", Lat: -23.2, Lon: -47}))
	require.NoError(t, d.SaveAirport(&navdata.Airport{ICAO: "NFFN", Name: "Nadi", Lat: -17.7, Lon: 178}))
	require.NoError(t, d.SaveAirport(&navdata.Airport{ICAO: "NSFA", Name: "Faleolo", Lat: -13.8, Lon: -172}))
	require.NoError(t, d.SaveFixes([]navdata.Fix{
		{Ident: "WEST1", Region: "NF", Kind: navdata.FixWaypoint, Lat: -17, Lon: 179, Routes: []navdata.Route{
			{Airway: "R1", NextIdent: "EAST1", NextRegion: "NS", NextKind: navdata.FixWaypoint, NextLat: -17, NextLon: -179}}},
		{Ident: "EAST1", Region: "NS", Kind: navdata.FixWaypoint, Lat: -17, Lon: -179},
		{Ident: "BRZ01", Region: "SB", Kind: navdata.FixWaypoint, Lat: -23, Lon: -46},
	}))
	return d
}

func icaos(a Area) []string {
	var out []string
	for _, x := range a.Airports {
		out = append(out, x.ICAO)
	}
	return out
}

func TestAViewOfTheWorldAsItIsFindsWhatIsThere(t *testing.T) {
	// Arrange
	d := worldDB(t)

	// Act
	a, err := d.InAreaView(-26, -50, -20, -44, 100, 100, true, false)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"SBSJ"}, icaos(a))
	require.Len(t, a.Fixes, 1)
	assert.Equal(t, -46.0, a.Fixes[0].Lon)
}

// Moving the map east, from Australia on, ends in Brazil one world to the east: its longitudes are
// 360 beyond the real ones, and what is there must still be found, where the view has it.
func TestAViewOneWorldToTheEastFindsBrazilAt313(t *testing.T) {
	// Arrange
	d := worldDB(t)

	// Act
	a, err := d.InAreaView(-26, 310, -20, 316, 100, 100, true, false)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"SBSJ"}, icaos(a))
	assert.Equal(t, 313.0, a.Airports[0].Lon, "-47 + 360")
	require.Len(t, a.Fixes, 1)
	assert.Equal(t, 314.0, a.Fixes[0].Lon, "-46 + 360")
}

func TestAViewOneWorldToTheWestFindsBrazilAtMinus407(t *testing.T) {
	// Arrange
	d := worldDB(t)

	// Act
	a, err := d.InAreaView(-30, -420, -10, -400, 100, 100, false, false)

	// Assert
	require.NoError(t, err)
	require.Len(t, a.Airports, 1)
	assert.Equal(t, -407.0, a.Airports[0].Lon)
}

func TestAViewAcrossTheAntimeridianFindsBothSides(t *testing.T) {
	// Arrange
	d := worldDB(t)

	// Act
	a, err := d.InAreaView(-30, 170, 0, 200, 100, 100, false, false)

	// Assert
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"NFFN", "NSFA"}, icaos(a))
	for _, x := range a.Airports {
		if x.ICAO == "NSFA" {
			assert.Equal(t, 188.0, x.Lon, "-172 + 360: past 180 where the view has it")
		}
	}
}

func TestAnAirwayAcrossTheAntimeridianIsDrawnTheShortWay(t *testing.T) {
	// Arrange
	d := worldDB(t)

	// Act
	a, err := d.InAreaView(-20, 175, -14, 183, 100, 100, false, true)

	// Assert
	require.NoError(t, err)
	require.NotEmpty(t, a.Segments)
	s := a.Segments[0]
	assert.InDelta(t, 2.0, s.ToLon-s.Lon, 1e-9, "from 179 to 181, not to -179")
}

func TestAViewTooWideShowsNoAirportsFixesOrAirways(t *testing.T) {
	// Arrange
	d := worldDB(t)

	// Act
	a, err := d.InAreaView(-60, -120, 30, 20, 100, 100, true, true)

	// Assert
	require.NoError(t, err)
	assert.True(t, a.AirportsHidden)
	assert.True(t, a.FixesHidden)
	assert.True(t, a.SegmentsHidden)
	assert.Empty(t, a.Airports)
	assert.Empty(t, a.Fixes)
	assert.Empty(t, a.Segments)
}

func TestAViewOfAMiddleWidthShowsAirportsButNotFixes(t *testing.T) {
	// Arrange
	d := worldDB(t)

	// Act: 20 degrees wide: fixes need 8, airways 20, airports 35
	a, err := d.InAreaView(-30, -60, -10, -40, 100, 100, true, true)

	// Assert
	require.NoError(t, err)
	assert.False(t, a.AirportsHidden)
	assert.True(t, a.FixesHidden)
	assert.False(t, a.SegmentsHidden)
	assert.Equal(t, []string{"SBSJ"}, icaos(a))
	assert.Empty(t, a.Fixes)
}

func TestAViewOfAWorldOrMoreGivesTheWorldOnceByItsMiddleCopy(t *testing.T) {
	// Act
	got := worldStretches(-400, 400)

	// Assert
	assert.Equal(t, []stretch{{-180, 180, 0}}, got)
}

func TestWorldStretchesOfAViewInsideTheWorldIsOneStretchUnshifted(t *testing.T) {
	// Act
	got := worldStretches(-60, -40)

	// Assert
	assert.Equal(t, []stretch{{-60, -40, 0}}, got)
}
