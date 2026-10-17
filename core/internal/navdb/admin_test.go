package navdb

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

func TestAUserPointIsStoredUpperCaseAndTrimmed(t *testing.T) {
	// Arrange
	d := openTemp(t)

	// Act
	p, err := d.CreateUserPoint(UserPoint{Ident: " mypt1 ", Name: "My point", Lat: -23.5, Lon: -46.6})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "MYPT1", p.Ident)
	assert.NotZero(t, p.ID)
}

func TestAUserPointIsRefusedWithABadIdentOrLatitude(t *testing.T) {
	cases := map[string]UserPoint{
		"a one-letter identifier": {Ident: "X"},
		"latitude 91":             {Ident: "ABC", Lat: 91},
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			d := openTemp(t)

			// Act
			_, err := d.CreateUserPoint(p)

			// Assert
			assert.Error(t, err)
		})
	}
}

func TestAUserPointShowsAsAFix(t *testing.T) {
	// Arrange
	d := openTemp(t)
	p, err := d.CreateUserPoint(UserPoint{Ident: "MYPT1", Name: "My point", Lat: -23.5, Lon: -46.6})
	require.NoError(t, err)
	p.Name = "Renamed"
	require.NoError(t, d.UpdateUserPoint(p))

	// Act
	fx, err := d.UserFixes("mypt1")

	// Assert
	require.NoError(t, err)
	require.Len(t, fx, 1)
	assert.Equal(t, UserRegion, fx[0].Region)
	assert.Equal(t, "Renamed", fx[0].Name)
	assert.True(t, fx[0].Kind == navdata.FixWaypoint, "kind %v", fx[0].Kind)
}

func TestDeletingAUserPointTwiceIsNotFound(t *testing.T) {
	// Arrange
	d := openTemp(t)
	p, err := d.CreateUserPoint(UserPoint{Ident: "MYPT1", Lat: 1, Lon: 1})
	require.NoError(t, err)
	require.NoError(t, d.DeleteUserPoint(p.ID))

	// Act
	err = d.DeleteUserPoint(p.ID)

	// Assert
	assert.Equal(t, ErrNotFound, err)
}

func TestTheSeedIsCountedAndTheCheckedIdentsLeaveThePending(t *testing.T) {
	// Arrange
	d := openTemp(t)
	_, err := d.CreateUserPoint(UserPoint{Ident: "MYPT1", Lat: 1, Lon: 1})
	require.NoError(t, err)
	require.NoError(t, d.SaveSeed([]SeedIdent{{Ident: "bobic", Kind: 1, Lat: 1, Lon: 2}, {Ident: "VUPSA"}}))

	// Act
	require.NoError(t, d.MarkIdentChecked("BOBIC", 1))

	// Assert
	pend, err := d.SeedPending(10)
	require.NoError(t, err)
	assert.Equal(t, []string{"VUPSA"}, pend)
	c, err := d.Counts()
	require.NoError(t, err)
	assert.Equal(t, 1, c.UserPoints)
	assert.Equal(t, 2, c.SeedIdents)
	assert.Equal(t, 1, c.SeedPending)
}

// The user data and the seed survive a reopen (and would survive a schema change: they are not dropped).
func TestUserPointsSurviveAReopen(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "nav.db")
	d, err := Open(path)
	require.NoError(t, err)
	_, err = d.CreateUserPoint(UserPoint{Ident: "MYPT1", Lat: 1, Lon: 1})
	require.NoError(t, err)
	d.Close()

	// Act
	d, err = Open(path)

	// Assert
	require.NoError(t, err)
	defer d.Close()
	list, err := d.UserPoints("")
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestASavedRouteIsRefusedWithoutANameOrText(t *testing.T) {
	cases := map[string]SavedRoute{
		"without a name": {Name: "", Text: "SBSJ SBGR"},
		"an empty route": {Name: "x", Text: "   "},
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			d := openTemp(t)

			// Act
			_, err := d.CreateSavedRoute(r)

			// Assert
			assert.Error(t, err)
		})
	}
}

func TestASavedRouteKeepsATrimmedNameAndOneSpaceBetweenNames(t *testing.T) {
	// Arrange
	d := openTemp(t)

	// Act
	r, err := d.CreateSavedRoute(SavedRoute{Name: " Casa ", Text: "SBSJ   BOBI3A\nDAGUB SBGR"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Casa", r.Name)
	assert.Equal(t, "SBSJ BOBI3A DAGUB SBGR", r.Text)
}

func TestASavedRouteCanBeRenamedAndDeleted(t *testing.T) {
	// Arrange
	d := openTemp(t)
	r, err := d.CreateSavedRoute(SavedRoute{Name: "Casa", Text: "SBSJ SBGR"})
	require.NoError(t, err)
	r.Name = "Casa 2"

	// Act
	_, err = d.UpdateSavedRoute(r)

	// Assert
	require.NoError(t, err)
	list, err := d.SavedRoutes()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Casa 2", list[0].Name)
	require.NoError(t, d.DeleteSavedRoute(r.ID))
	assert.Equal(t, ErrNotFound, d.DeleteSavedRoute(r.ID), "the second delete")
}

// areaDB has two airports, a stretch of an airway and a far fix.
func areaDB(t *testing.T) *DB {
	t.Helper()
	d := openTemp(t)
	require.NoError(t, d.SaveAirport(&navdata.Airport{ICAO: "SBSJ", Name: "Sao Jose", Lat: -23.2, Lon: -45.9}))
	require.NoError(t, d.SaveAirport(&navdata.Airport{ICAO: "KJFK", Name: "JFK", Lat: 40.6, Lon: -73.8}))
	require.NoError(t, d.SaveFixes([]navdata.Fix{
		{Ident: "AAAAA", Region: "SB", Kind: navdata.FixWaypoint, Lat: -23.0, Lon: -46.0, Routes: []navdata.Route{
			{Airway: "UZ6", NextIdent: "BBBBB", NextRegion: "SB", NextKind: navdata.FixWaypoint, NextLat: -22.5, NextLon: -46.5}}},
		{Ident: "BBBBB", Region: "SB", Kind: navdata.FixWaypoint, Lat: -22.5, Lon: -46.5},
		{Ident: "FAR", Region: "K6", Kind: navdata.FixWaypoint, Lat: 40.0, Lon: -74.0},
	}))
	return d
}

func TestInAreaGivesWhatIsInsideTheBox(t *testing.T) {
	// Arrange
	d := areaDB(t)

	// Act
	a, err := d.InArea(-24, -47, -22, -45, 100, 100, true, true)

	// Assert
	require.NoError(t, err)
	require.Len(t, a.Airports, 1)
	assert.Equal(t, "SBSJ", a.Airports[0].ICAO)
	assert.Len(t, a.Fixes, 2)
	require.Len(t, a.Segments, 1)
	assert.Equal(t, "UZ6", a.Segments[0].Airway)
}

func TestInAreaReportsWhenTheCapCutTheFixes(t *testing.T) {
	// Arrange
	d := areaDB(t)

	// Act
	capped, err := d.InArea(-24, -47, -22, -45, 1, 100, true, false)

	// Assert
	require.NoError(t, err)
	assert.Len(t, capped.Fixes, 1)
	assert.True(t, capped.FixesCut)
}

func TestInAreaGivesFixesAndSegmentsOnlyWhenAskedFor(t *testing.T) {
	// Arrange
	d := areaDB(t)

	// Act
	none, err := d.InArea(-24, -47, -22, -45, 100, 100, false, false)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, none.Fixes)
	assert.Empty(t, none.Segments)
}

func searchDB(t *testing.T) *DB {
	t.Helper()
	d := openTemp(t)
	require.NoError(t, d.SaveAirport(&navdata.Airport{ICAO: "SBSJ", Name: "Sao Jose dos Campos", Lat: -23.2, Lon: -45.9}))
	require.NoError(t, d.SaveFixes([]navdata.Fix{
		{Ident: "RBV", Region: "K6", Kind: navdata.FixVOR, Name: "ROBBINSVILLE", Lat: 40, Lon: -74},
		{Ident: "RBVAA", Region: "SB", Kind: navdata.FixWaypoint, Lat: -22, Lon: -46},
		{Ident: "ZZZZZ", Region: "SB", Kind: navdata.FixVOR, Name: "ROBIN VOR", Lat: -21, Lon: -45},
	}))
	_, err := d.CreateUserPoint(UserPoint{Ident: "RBVHOME", Name: "Casa", Lat: -23, Lon: -46})
	require.NoError(t, err)
	return d
}

func TestSearchFindsFixesAndUserPointsByIdent(t *testing.T) {
	// Arrange
	d := searchDB(t)

	// Act
	hits, err := d.Search("rbv", 10)

	// Assert
	require.NoError(t, err)
	require.Len(t, hits, 3)
	assert.Equal(t, "RBV", hits[0].Ident, "the exact name first")
}

func TestSearchFindsAirportsByName(t *testing.T) {
	// Arrange
	d := searchDB(t)

	// Act
	hits, err := d.Search("jose", 10)

	// Assert
	require.NoError(t, err)
	require.Len(t, hits, 1)
	assert.Equal(t, "SBSJ", hits[0].Ident)
	assert.Equal(t, "airport", hits[0].Type)
}

func TestSearchNeedsTwoLetters(t *testing.T) {
	// Arrange
	d := searchDB(t)

	// Act
	hits, err := d.Search("r", 10)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, hits)
}

// B is stored (and says that the airway comes from A and goes to C); A and C are not stored.
func airwayWithOneFixStored(t *testing.T) *DB {
	t.Helper()
	d := openTemp(t)
	require.NoError(t, d.SaveFixes([]navdata.Fix{{Ident: "BBBBB", Region: "SB", Kind: navdata.FixWaypoint, Lat: -22.5, Lon: -46.5, Routes: []navdata.Route{{
		Airway: "UZ6", PrevIdent: "AAAAA", PrevRegion: "SB", PrevKind: navdata.FixWaypoint, PrevLat: -23, PrevLon: -46,
		NextIdent: "CCCCC", NextRegion: "SB", NextKind: navdata.FixWaypoint, NextLat: -22, NextLon: -47}}}}))
	return d
}

func TestAnAirwayShowsBothStretchesWhenTheEndsAreMissing(t *testing.T) {
	// Arrange
	d := airwayWithOneFixStored(t)

	// Act
	a, err := d.InArea(-24, -48, -21, -45, 100, 100, false, true)

	// Assert
	require.NoError(t, err)
	assert.Len(t, a.Segments, 2)
}

func TestMissingNeighborsAreTheEndsNotStored(t *testing.T) {
	// Arrange
	d := airwayWithOneFixStored(t)

	// Act
	miss, err := d.MissingNeighbors(10)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"AAAAA", "CCCCC"}, miss)
}

func TestAMissingNeighborAlreadyAskedIsNotAskedAgain(t *testing.T) {
	// Arrange
	d := airwayWithOneFixStored(t)
	require.NoError(t, d.MarkIdentChecked("AAAAA", 0))

	// Act
	miss, err := d.MissingNeighbors(10)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"CCCCC"}, miss)
}
