package route

import (
	"reflect"
	"testing"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

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

func TestTokens(t *testing.T) {
	got := Tokens("sbsj dct BOBI3A/N0400F250  DAGUB, N0450F350 uz6 SBGR")
	want := []string{"SBSJ", "BOBI3A", "DAGUB", "UZ6", "SBGR"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestAirwayAndAirports(t *testing.T) {
	res, err := Resolve("SBSJ AAAAA UZ6 DDDDD SBGR", provider())
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range res.Points {
		ids = append(ids, p.Ident)
	}
	want := []string{"SBSJ", "AAAAA", "BBBBB", "CCCCC", "DDDDD", "SBGR"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("points %v, want %v (problems %+v)", ids, want, res.Problems)
	}
	if len(res.Problems) != 0 {
		t.Fatalf("problems: %+v", res.Problems)
	}
	if res.Points[2].Via != "UZ6" || res.Points[1].Via != "" {
		t.Fatalf("via: %+v", res.Points)
	}
}

func TestProcedureWithTransition(t *testing.T) {
	res, _ := Resolve("SBSJ BOBI3A AAAAA SBGR", provider())
	var ids []string
	for _, p := range res.Points {
		ids = append(ids, p.Ident)
	}
	// the common leg and the enroute transition named AAAAA; the fix written after the procedure
	// is the place the procedure ended at, not a second point
	want := []string{"SBSJ", "BOBIC", "AAAAA", "SBGR"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("points %v, want %v (problems %+v)", ids, want, res.Problems)
	}
}

func TestHomonymsUseTheClosest(t *testing.T) {
	res, _ := Resolve("SBSJ RBV SBGR", provider())
	var rbv *Point
	for i := range res.Points {
		if res.Points[i].Ident == "RBV" {
			rbv = &res.Points[i]
		}
	}
	if rbv == nil || rbv.Region != "FA" {
		t.Fatalf("closest RBV to São Paulo is the one in FA: %+v", res.Points)
	}
	if len(res.Problems) != 1 || len(res.Problems[0].Candidates) != 2 {
		t.Fatalf("the doubt must be reported: %+v", res.Problems)
	}
}

func TestNotFoundIsReported(t *testing.T) {
	res, _ := Resolve("SBSJ ZZZZZ SBGR", provider())
	if len(res.Problems) != 1 || res.Problems[0].Token != "ZZZZZ" {
		t.Fatalf("problems: %+v", res.Problems)
	}
	if len(res.Points) != 2 {
		t.Fatalf("the rest of the route is kept: %+v", res.Points)
	}
}

func TestCoordinates(t *testing.T) {
	res, _ := Resolve("SBSJ 2330S04600W SBGR", provider())
	p := res.Points[1]
	if p.Kind != "coordinate" || p.Lat > -23.49 || p.Lat < -23.51 || p.Lon > -45.99 || p.Lon < -46.01 {
		t.Fatalf("coordinate: %+v", p)
	}
}

func TestAirwayThatDoesNotConnect(t *testing.T) {
	res, _ := Resolve("SBSJ AAAAA UY1 DDDDD SBGR", provider())
	if len(res.Problems) == 0 {
		t.Fatalf("an unknown airway must be a problem: %+v", res)
	}
}

func TestChoiceOverridesTheClosest(t *testing.T) {
	// the other RBV is the one the user wants; the choice is by region and survives text edits
	res, _ := Resolve("SBSJ RBV SBGR", provider())
	var other string
	var cands []Candidate
	for _, p := range res.Points {
		if p.Ident == "RBV" {
			cands = p.Candidates
			if len(cands) != 2 || p.Chosen {
				t.Fatalf("point must list both places and not be chosen: %+v", p)
			}
		}
	}
	for _, c := range cands {
		if c.Region != "FA" {
			other = c.Region
		}
	}
	res, err := ResolveWith("SBSJ RBV SBGR", provider(), []Choice{{Token: "RBV", Nth: 0, Region: other, Kind: "vor"}})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, p := range res.Points {
		if p.Ident == "RBV" {
			found = true
			if p.Region != other || !p.Chosen {
				t.Fatalf("the choice was not applied: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("RBV missing")
	}
	if len(res.Problems) != 0 {
		t.Fatalf("a chosen place is not a doubt: %+v", res.Problems)
	}
	// a choice for another occurrence changes nothing
	res, _ = ResolveWith("SBSJ RBV SBGR", provider(), []Choice{{Token: "RBV", Nth: 1, Region: other}})
	for _, p := range res.Points {
		if p.Ident == "RBV" && p.Region != "FA" {
			t.Fatalf("a choice for the second RBV must not change the first: %+v", p)
		}
	}
}

func TestGreatCircleNm(t *testing.T) {
	// one degree of latitude is 60 nautical miles, within a few tenths
	d := GreatCircleNm(0, 0, 1, 0)
	if d < 59.9 || d > 60.2 {
		t.Fatalf("1 degree of latitude: %.2f NM", d)
	}
	legs, total := Legs([]float64{0, 1, 2}, []float64{0, 0, 0})
	if len(legs) != 3 || legs[0] != 0 || total < 119.8 || total > 120.4 {
		t.Fatalf("legs %v total %.1f", legs, total)
	}
}

func TestResolveGivesTheLengthOfTheRoute(t *testing.T) {
	res, _ := Resolve("SBSJ AAAAA UZ6 DDDDD SBGR", provider())
	if res.TotalNm <= 0 || res.Points[0].LegNm != 0 || res.Points[1].LegNm <= 0 {
		t.Fatalf("total %.1f, legs %+v", res.TotalNm, res.Points)
	}
}
