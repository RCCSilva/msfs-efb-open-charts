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

func TestFileFixesAreAddedWithTheirAirwaysAndSayTheyCameFromTheFiles(t *testing.T) {
	// Arrange
	d := openTemp(t)
	fix := navdata.Fix{Ident: "REBAM", Region: "SB", Kind: navdata.FixWaypoint, Lat: -9.48, Lon: -40.75, Routes: []navdata.Route{
		{Airway: "UZ84", Type: 2, NextIdent: "ANBUM", NextRegion: "SB", NextKind: navdata.FixWaypoint, NextLat: -9.32, NextLon: -40.37}}}

	// Act
	added, err := d.SaveFileFixes([]navdata.Fix{fix})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, added)
	got, err := d.Fix("REBAM", "SB")
	require.NoError(t, err)
	require.Len(t, got.Routes, 1)
	assert.Equal(t, "ANBUM", got.Routes[0].NextIdent)
	var source string
	require.NoError(t, d.db.QueryRow(`SELECT source FROM fix WHERE ident = 'REBAM'`).Scan(&source))
	assert.Equal(t, SourceFiles, source)
}

func TestFileFixesDoNotTouchAFixTheSimulatorGaveAtTheSamePlaceWhateverItsRegion(t *testing.T) {
	// Arrange: the simulator's answer for an en-route waypoint has no region
	d := openTemp(t)
	require.NoError(t, d.SaveFixes([]navdata.Fix{{Ident: "GERTU", Region: "", Kind: navdata.FixWaypoint, Lat: -22.43018, Lon: -47.56245,
		Routes: []navdata.Route{{Airway: "UM775", NextIdent: "X"}}}}))

	// Act
	added, err := d.SaveFileFixes([]navdata.Fix{{Ident: "GERTU", Region: "SB", Kind: navdata.FixWaypoint, Lat: -22.43018, Lon: -47.56245}})

	// Assert
	require.NoError(t, err)
	assert.Zero(t, added)
	var n int
	require.NoError(t, d.db.QueryRow(`SELECT COUNT(*) FROM fix WHERE ident = 'GERTU'`).Scan(&n))
	assert.Equal(t, 1, n, "no duplicate")
	got, err := d.Fix("GERTU", "")
	require.NoError(t, err)
	assert.Len(t, got.Routes, 1, "the simulator's airways are kept")
}

func TestFileFixesOfTheSameNameInAnotherPlaceAreAnotherFix(t *testing.T) {
	// Arrange
	d := openTemp(t)
	_, err := d.SaveFileFixes([]navdata.Fix{{Ident: "ABCDE", Region: "SB", Kind: navdata.FixWaypoint, Lat: -10, Lon: -40}})
	require.NoError(t, err)

	// Act
	added, err := d.SaveFileFixes([]navdata.Fix{{Ident: "ABCDE", Region: "K1", Kind: navdata.FixWaypoint, Lat: 40, Lon: -100}})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, added)
}

func TestTheSimulatorReplacesAFixFromTheFilesAndItBecomesTheSimulators(t *testing.T) {
	// Arrange
	d := openTemp(t)
	_, err := d.SaveFileFixes([]navdata.Fix{{Ident: "ABCDE", Region: "SB", Kind: navdata.FixWaypoint, Lat: -10, Lon: -40}})
	require.NoError(t, err)

	// Act
	require.NoError(t, d.SaveFixes([]navdata.Fix{{Ident: "ABCDE", Region: "SB", Kind: navdata.FixWaypoint, Lat: -10, Lon: -40}}))

	// Assert
	var source string
	require.NoError(t, d.db.QueryRow(`SELECT source FROM fix WHERE ident = 'ABCDE'`).Scan(&source))
	assert.Equal(t, SourceSim, source)
}

func TestNamesCoveredByTheFilesAreMarkedCheckedWithHowManyFixesTheyHave(t *testing.T) {
	// Arrange
	d := openTemp(t)
	_, err := d.SaveFileFixes([]navdata.Fix{{Ident: "ABCDE", Region: "SB", Kind: navdata.FixWaypoint, Lat: -10, Lon: -40}})
	require.NoError(t, err)

	// Act
	require.NoError(t, d.MarkIdentsCovered([]string{"abcde", "NOPE"}))

	// Assert
	checked, err := d.IdentChecked("ABCDE")
	require.NoError(t, err)
	assert.True(t, checked)
	var found int
	require.NoError(t, d.db.QueryRow(`SELECT found FROM ident_check WHERE ident = 'ABCDE'`).Scan(&found))
	assert.Equal(t, 1, found)
}

func TestFillRoutePositionsPlacesANeighbourOnceTheSimulatorGaveIt(t *testing.T) {
	// Arrange: the files named ANBUM as the next fix of REBAM but had no record of it
	d := openTemp(t)
	_, err := d.SaveFileFixes([]navdata.Fix{{Ident: "REBAM", Region: "SB", Kind: navdata.FixWaypoint, Lat: -9.48, Lon: -40.75, Routes: []navdata.Route{
		{Airway: "UZ84", NextIdent: "ANBUM", NextRegion: "SB", NextKind: navdata.FixWaypoint}}}})
	require.NoError(t, err)
	require.NoError(t, d.SaveFixes([]navdata.Fix{{Ident: "ANBUM", Region: "", Kind: navdata.FixWaypoint, Lat: -9.32, Lon: -40.37}}))

	// Act
	n, err := d.FillRoutePositions()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	got, err := d.Fix("REBAM", "SB")
	require.NoError(t, err)
	assert.Equal(t, -9.32, got.Routes[0].NextLat)
	assert.Equal(t, -40.37, got.Routes[0].NextLon)
}

func TestFillRoutePositionsTakesTheNearestOfTwoFixesWithTheSameName(t *testing.T) {
	// Arrange
	d := openTemp(t)
	_, err := d.SaveFileFixes([]navdata.Fix{{Ident: "AAAAA", Region: "SB", Kind: navdata.FixWaypoint, Lat: -9, Lon: -40, Routes: []navdata.Route{
		{Airway: "UZ84", NextIdent: "TWINS", NextKind: navdata.FixWaypoint}}}})
	require.NoError(t, err)
	require.NoError(t, d.SaveFixes([]navdata.Fix{
		{Ident: "TWINS", Region: "SB", Kind: navdata.FixWaypoint, Lat: -9.5, Lon: -40.5},
		{Ident: "TWINS", Region: "K1", Kind: navdata.FixWaypoint, Lat: 40, Lon: -100}}))

	// Act
	_, err = d.FillRoutePositions()

	// Assert
	require.NoError(t, err)
	got, err := d.Fix("AAAAA", "SB")
	require.NoError(t, err)
	assert.Equal(t, -9.5, got.Routes[0].NextLat)
}

func TestAnAirwayDoesNotDrawASegmentToAnUnplacedNeighbour(t *testing.T) {
	// Arrange
	d := openTemp(t)
	_, err := d.SaveFileFixes([]navdata.Fix{{Ident: "REBAM", Region: "SB", Kind: navdata.FixWaypoint, Lat: -9.48, Lon: -40.75, Routes: []navdata.Route{
		{Airway: "UZ84", NextIdent: "ANBUM", NextRegion: "SB", NextKind: navdata.FixWaypoint}}}})
	require.NoError(t, err)

	// Act
	segs, err := d.AirwaySegments("UZ84")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, segs, "a line to 0, 0 would be wrong")
}

func TestAnAirwaySegmentSaidOnlyByTheFixAfterIsListedOnceWhenBothSayIt(t *testing.T) {
	// Arrange: B lists A as its previous fix; A lists B as its next one; C lists B as previous only
	d := openTemp(t)
	require.NoError(t, d.SaveFixes([]navdata.Fix{
		{Ident: "AAAAA", Region: "SB", Kind: navdata.FixWaypoint, Lat: 1, Lon: 1, Routes: []navdata.Route{
			{Airway: "UZ1", NextIdent: "BBBBB", NextKind: navdata.FixWaypoint, NextLat: 2, NextLon: 2}}},
		{Ident: "BBBBB", Region: "SB", Kind: navdata.FixWaypoint, Lat: 2, Lon: 2, Routes: []navdata.Route{
			{Airway: "UZ1", PrevIdent: "AAAAA", PrevKind: navdata.FixWaypoint, PrevLat: 1, PrevLon: 1}}},
		{Ident: "CCCCC", Region: "SB", Kind: navdata.FixWaypoint, Lat: 3, Lon: 3, Routes: []navdata.Route{
			{Airway: "UZ1", PrevIdent: "BBBBB", PrevKind: navdata.FixWaypoint, PrevLat: 2, PrevLon: 2}}},
	}))

	// Act
	segs, err := d.AirwaySegments("uz1")

	// Assert
	require.NoError(t, err)
	assert.Len(t, segs, 2, "A-B once, B-C once")
}
