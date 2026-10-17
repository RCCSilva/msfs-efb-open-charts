package route

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// fake is an in-memory navigation provider with a small, known world (see provider()).
type fake struct {
	airports map[string]*navdata.Airport
	fixes    map[string][]navdata.Fix
}

func (f fake) Airport(icao string) (*navdata.Airport, error) { return f.airports[icao], nil }
func (f fake) Fixes(ident string) ([]navdata.Fix, error)     { return f.fixes[ident], nil }
func (f fake) FixAt(ref navdata.FixRef) (*navdata.Fix, error) {
	for _, x := range f.fixes[ref.Ident] {
		if x.Region == ref.Region {
			c := x
			return &c, nil
		}
	}
	return nil, nil
}

func wp(id, region string, lat, lon float64, routes ...navdata.Route) navdata.Fix {
	return navdata.Fix{Ident: id, Region: region, Kind: 'W', Lat: lat, Lon: lon, Routes: routes}
}

func ptr(f float64) *float64 { return &f }

func provider() fake {
	// an airway UZ6: AAAAA - BBBBB - CCCCC - DDDDD, in region SB
	a := wp("AAAAA", "SB", -23, -46, navdata.Route{Airway: "UZ6", NextIdent: "BBBBB", NextRegion: "SB", NextKind: 'W', NextLat: -22, NextLon: -45})
	b := wp("BBBBB", "SB", -22, -45,
		navdata.Route{Airway: "UZ6", PrevIdent: "AAAAA", PrevRegion: "SB", PrevKind: 'W', PrevLat: -23, PrevLon: -46, NextIdent: "CCCCC", NextRegion: "SB", NextKind: 'W', NextLat: -21, NextLon: -44})
	c := wp("CCCCC", "SB", -21, -44,
		navdata.Route{Airway: "UZ6", PrevIdent: "BBBBB", PrevRegion: "SB", PrevKind: 'W', PrevLat: -22, PrevLon: -45, NextIdent: "DDDDD", NextRegion: "SB", NextKind: 'W', NextLat: -20, NextLon: -43})
	d := wp("DDDDD", "SB", -20, -43,
		navdata.Route{Airway: "UZ6", PrevIdent: "CCCCC", PrevRegion: "SB", PrevKind: 'W', PrevLat: -21, PrevLon: -44})
	return fake{
		airports: map[string]*navdata.Airport{
			"SBSJ": {ICAO: "SBSJ", Region: "SB", Lat: -23.2, Lon: -45.9, Procedures: []navdata.Procedure{{
				Kind: navdata.KindSID, Name: "BOBI3A", Transitions: []navdata.Transition{
					{Kind: navdata.TransCommon, Legs: []navdata.Leg{{FixICAO: "BOBIC", FixRegion: "SB", Lat: ptr(-23.19), Lon: ptr(-45.90)}}},
					{Kind: navdata.TransEnroute, Name: "AAAAA", Legs: []navdata.Leg{{FixICAO: "AAAAA", FixRegion: "SB", Lat: ptr(-23), Lon: ptr(-46)}}},
				}}}},
			"SBGR": {ICAO: "SBGR", Region: "SB", Lat: -23.43, Lon: -46.47},
		},
		fixes: map[string][]navdata.Fix{
			"AAAAA": {a}, "BBBBB": {b}, "CCCCC": {c}, "DDDDD": {d},
			// the same name in two places
			"RBV": {
				{Ident: "RBV", Region: "FA", Kind: 'V', Lat: -29.6, Lon: 30.8},
				{Ident: "RBV", Region: "K6", Kind: 'V', Lat: 40.2, Lon: -74.5},
			},
		},
	}
}

func idents(points []Point) []string {
	var out []string
	for _, p := range points {
		out = append(out, p.Ident)
	}
	return out
}

func TestTokens(t *testing.T) {
	// Arrange
	text := "sbsj dct BOBI3A/N0400F250  DAGUB, N0450F350 uz6 SBGR"

	// Act
	got := Tokens(text)

	// Assert
	assert.Equal(t, []string{"SBSJ", "BOBI3A", "DAGUB", "UZ6", "SBGR"}, got)
}

func TestAirwayAndAirports(t *testing.T) {
	// Arrange
	p := provider()

	// Act
	res, err := Resolve("SBSJ AAAAA UZ6 DDDDD SBGR", p)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"SBSJ", "AAAAA", "BBBBB", "CCCCC", "DDDDD", "SBGR"}, idents(res.Points))
	assert.Empty(t, res.Problems)
	assert.Equal(t, "UZ6", res.Points[2].Via)
	assert.Empty(t, res.Points[1].Via)
}

func TestProcedureWithTransition(t *testing.T) {
	// Arrange
	p := provider()

	// Act
	res, err := Resolve("SBSJ BOBI3A AAAAA SBGR", p)

	// Assert: the common leg and the enroute transition named AAAAA; the fix written after the
	// procedure is the place the procedure ended at, not a second point
	require.NoError(t, err)
	assert.Equal(t, []string{"SBSJ", "BOBIC", "AAAAA", "SBGR"}, idents(res.Points), "problems: %+v", res.Problems)
}

func TestHomonymsUseTheClosest(t *testing.T) {
	// Arrange
	p := provider()

	// Act
	res, err := Resolve("SBSJ RBV SBGR", p)

	// Assert
	require.NoError(t, err)
	var rbv *Point
	for i := range res.Points {
		if res.Points[i].Ident == "RBV" {
			rbv = &res.Points[i]
		}
	}
	require.NotNil(t, rbv)
	assert.Equal(t, "FA", rbv.Region, "the closest RBV to São Paulo is the one in FA")
	require.Len(t, res.Problems, 1, "the doubt must be reported")
	assert.Len(t, res.Problems[0].Candidates, 2)
}

func TestNotFoundIsReported(t *testing.T) {
	// Arrange
	p := provider()

	// Act
	res, err := Resolve("SBSJ ZZZZZ SBGR", p)

	// Assert
	require.NoError(t, err)
	require.Len(t, res.Problems, 1)
	assert.Equal(t, "ZZZZZ", res.Problems[0].Token)
	assert.Len(t, res.Points, 2, "the rest of the route is kept")
}

func TestCoordinates(t *testing.T) {
	// Arrange
	p := provider()

	// Act
	res, err := Resolve("SBSJ 2330S04600W SBGR", p)

	// Assert
	require.NoError(t, err)
	got := res.Points[1]
	assert.Equal(t, "coordinate", got.Kind)
	assert.InDelta(t, -23.5, got.Lat, 0.01)
	assert.InDelta(t, -46.0, got.Lon, 0.01)
}

func TestAirwayThatDoesNotConnect(t *testing.T) {
	// Arrange
	p := provider()

	// Act
	res, err := Resolve("SBSJ AAAAA UY1 DDDDD SBGR", p)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, res.Problems, "an unknown airway must be a problem")
}

// The other RBV is the one the user wants: the choice is by region and survives edits of the text.
func TestChoiceOverridesTheClosest(t *testing.T) {
	// Arrange
	p := provider()
	first, err := Resolve("SBSJ RBV SBGR", p)
	require.NoError(t, err)
	var other string
	for _, pt := range first.Points {
		if pt.Ident != "RBV" {
			continue
		}
		require.Len(t, pt.Candidates, 2, "the point lists both places")
		assert.False(t, pt.Chosen)
		for _, c := range pt.Candidates {
			if c.Region != "FA" {
				other = c.Region
			}
		}
	}
	require.NotEmpty(t, other)

	// Act
	res, err := ResolveWith("SBSJ RBV SBGR", p, []Choice{{Token: "RBV", Nth: 0, Region: other, Kind: "vor"}})

	// Assert
	require.NoError(t, err)
	found := false
	for _, pt := range res.Points {
		if pt.Ident == "RBV" {
			found = true
			assert.Equal(t, other, pt.Region, "the choice was applied")
			assert.True(t, pt.Chosen)
		}
	}
	assert.True(t, found, "RBV is in the route")
	assert.Empty(t, res.Problems, "a chosen place is not a doubt")
}

func TestChoiceForAnotherOccurrenceChangesNothing(t *testing.T) {
	// Arrange
	p := provider()
	choices := []Choice{{Token: "RBV", Nth: 1, Region: "K6"}} // the second RBV; the route has one

	// Act
	res, err := ResolveWith("SBSJ RBV SBGR", p, choices)

	// Assert
	require.NoError(t, err)
	for _, pt := range res.Points {
		if pt.Ident == "RBV" {
			assert.Equal(t, "FA", pt.Region, "a choice for the second RBV must not change the first")
		}
	}
}

func TestGreatCircleNm(t *testing.T) {
	// Arrange: one degree of latitude is 60 nautical miles

	// Act
	d := GreatCircleNm(0, 0, 1, 0)
	legs, total := Legs([]float64{0, 1, 2}, []float64{0, 0, 0})

	// Assert
	assert.InDelta(t, 60, d, 0.2)
	require.Len(t, legs, 3)
	assert.Equal(t, 0.0, legs[0])
	assert.InDelta(t, 120, total, 0.3)
}

func TestResolveGivesTheLengthOfTheRoute(t *testing.T) {
	// Arrange
	p := provider()

	// Act
	res, err := Resolve("SBSJ AAAAA UZ6 DDDDD SBGR", p)

	// Assert
	require.NoError(t, err)
	assert.Greater(t, res.TotalNm, 0.0)
	assert.Equal(t, 0.0, res.Points[0].LegNm)
	assert.Greater(t, res.Points[1].LegNm, 0.0)
}
