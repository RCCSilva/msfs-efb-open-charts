package navdb

import (
	"path/filepath"
	"testing"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

func TestUserPointsAndSeed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nav.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.CreateUserPoint(UserPoint{Ident: " mypt1 ", Name: "My point", Lat: -23.5, Lon: -46.6})
	if err != nil || p.Ident != "MYPT1" || p.ID == 0 {
		t.Fatalf("create: %+v %v", p, err)
	}
	if _, err := d.CreateUserPoint(UserPoint{Ident: "X", Lat: 0, Lon: 0}); err == nil {
		t.Fatal("a one-letter identifier must be refused")
	}
	if _, err := d.CreateUserPoint(UserPoint{Ident: "ABC", Lat: 91, Lon: 0}); err == nil {
		t.Fatal("latitude 91 must be refused")
	}
	p.Name = "Renamed"
	if err := d.UpdateUserPoint(p); err != nil {
		t.Fatal(err)
	}
	fx, err := d.UserFixes("mypt1")
	if err != nil || len(fx) != 1 || fx[0].Region != UserRegion || fx[0].Name != "Renamed" || fx[0].Kind != navdata.FixWaypoint {
		t.Fatalf("user fixes: %+v %v", fx, err)
	}

	if err := d.SaveSeed([]SeedIdent{{Ident: "bobic", Kind: 1, Lat: 1, Lon: 2}, {Ident: "VUPSA"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.MarkIdentChecked("BOBIC", 1); err != nil {
		t.Fatal(err)
	}
	pend, err := d.SeedPending(10)
	if err != nil || len(pend) != 1 || pend[0] != "VUPSA" {
		t.Fatalf("pending: %v %v", pend, err)
	}
	c, err := d.Counts()
	if err != nil || c.UserPoints != 1 || c.SeedIdents != 2 || c.SeedPending != 1 {
		t.Fatalf("counts: %+v %v", c, err)
	}

	// user data and the seed survive a reopen (and would survive a schema change: they are not dropped)
	d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if list, _ := d.UserPoints(""); len(list) != 1 {
		t.Fatalf("points after reopen: %v", list)
	}
	if err := d.DeleteUserPoint(p.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteUserPoint(p.ID); err != ErrNotFound {
		t.Fatalf("second delete: %v", err)
	}
}

func TestSavedRoutes(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "r.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.CreateSavedRoute(SavedRoute{Name: "", Text: "SBSJ SBGR"}); err == nil {
		t.Fatal("a route without a name must be refused")
	}
	if _, err := d.CreateSavedRoute(SavedRoute{Name: "x", Text: "   "}); err == nil {
		t.Fatal("an empty route must be refused")
	}
	r, err := d.CreateSavedRoute(SavedRoute{Name: " Casa ", Text: "SBSJ   BOBI3A\nDAGUB SBGR"})
	if err != nil || r.Name != "Casa" || r.Text != "SBSJ BOBI3A DAGUB SBGR" {
		t.Fatalf("create: %+v %v", r, err)
	}
	r.Name = "Casa 2"
	if _, err := d.UpdateSavedRoute(r); err != nil {
		t.Fatal(err)
	}
	list, _ := d.SavedRoutes()
	if len(list) != 1 || list[0].Name != "Casa 2" {
		t.Fatalf("list: %+v", list)
	}
	if err := d.DeleteSavedRoute(r.ID); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteSavedRoute(r.ID); err != ErrNotFound {
		t.Fatalf("second delete: %v", err)
	}
}

func TestInArea(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.SaveAirport(&navdata.Airport{ICAO: "SBSJ", Name: "Sao Jose", Lat: -23.2, Lon: -45.9}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveAirport(&navdata.Airport{ICAO: "KJFK", Name: "JFK", Lat: 40.6, Lon: -73.8}); err != nil {
		t.Fatal(err)
	}
	fixes := []navdata.Fix{
		{Ident: "AAAAA", Region: "SB", Kind: navdata.FixWaypoint, Lat: -23.0, Lon: -46.0, Routes: []navdata.Route{
			{Airway: "UZ6", NextIdent: "BBBBB", NextRegion: "SB", NextKind: navdata.FixWaypoint, NextLat: -22.5, NextLon: -46.5}}},
		{Ident: "BBBBB", Region: "SB", Kind: navdata.FixWaypoint, Lat: -22.5, Lon: -46.5},
		{Ident: "FAR", Region: "K6", Kind: navdata.FixWaypoint, Lat: 40.0, Lon: -74.0},
	}
	if err := d.SaveFixes(fixes); err != nil {
		t.Fatal(err)
	}
	a, err := d.InArea(-24, -47, -22, -45, 100, 100, true, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Airports) != 1 || a.Airports[0].ICAO != "SBSJ" || len(a.Fixes) != 2 || len(a.Segments) != 1 || a.Segments[0].Airway != "UZ6" {
		t.Fatalf("area: %+v", a)
	}
	capped, _ := d.InArea(-24, -47, -22, -45, 1, 100, true, false)
	if len(capped.Fixes) != 1 || !capped.FixesCut {
		t.Fatalf("cap not reported: %+v", capped)
	}
	none, _ := d.InArea(-24, -47, -22, -45, 100, 100, false, false)
	if len(none.Fixes) != 0 || len(none.Segments) != 0 {
		t.Fatalf("fixes and segments are asked for: %+v", none)
	}
}

func TestSearch(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.SaveAirport(&navdata.Airport{ICAO: "SBSJ", Name: "Sao Jose dos Campos", Lat: -23.2, Lon: -45.9}); err != nil {
		t.Fatal(err)
	}
	if err := d.SaveFixes([]navdata.Fix{
		{Ident: "RBV", Region: "K6", Kind: navdata.FixVOR, Name: "ROBBINSVILLE", Lat: 40, Lon: -74},
		{Ident: "RBVAA", Region: "SB", Kind: navdata.FixWaypoint, Lat: -22, Lon: -46},
		{Ident: "ZZZZZ", Region: "SB", Kind: navdata.FixVOR, Name: "ROBIN VOR", Lat: -21, Lon: -45},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := d.CreateUserPoint(UserPoint{Ident: "RBVHOME", Name: "Casa", Lat: -23, Lon: -46}); err != nil {
		t.Fatal(err)
	}
	hits, err := d.Search("rbv", 10)
	if err != nil || len(hits) != 3 || hits[0].Ident != "RBV" {
		t.Fatalf("hits: %+v %v", hits, err)
	}
	byName, _ := d.Search("jose", 10)
	if len(byName) != 1 || byName[0].Ident != "SBSJ" || byName[0].Type != "airport" {
		t.Fatalf("by name: %+v", byName)
	}
	if short, _ := d.Search("r", 10); len(short) != 0 {
		t.Fatalf("one letter must not search: %+v", short)
	}
}

func TestAirwaysShowWhenOneEndIsMissing(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	// B is stored (and says that the airway comes from A and goes to C); A and C are not stored
	if err := d.SaveFixes([]navdata.Fix{{Ident: "BBBBB", Region: "SB", Kind: navdata.FixWaypoint, Lat: -22.5, Lon: -46.5, Routes: []navdata.Route{{
		Airway: "UZ6", PrevIdent: "AAAAA", PrevRegion: "SB", PrevKind: navdata.FixWaypoint, PrevLat: -23, PrevLon: -46,
		NextIdent: "CCCCC", NextRegion: "SB", NextKind: navdata.FixWaypoint, NextLat: -22, NextLon: -47}}}}); err != nil {
		t.Fatal(err)
	}
	a, err := d.InArea(-24, -48, -21, -45, 100, 100, false, true)
	if err != nil || len(a.Segments) != 2 {
		t.Fatalf("both stretches must show: %+v %v", a.Segments, err)
	}
	miss, err := d.MissingNeighbors(10)
	if err != nil || len(miss) != 2 || miss[0] != "AAAAA" || miss[1] != "CCCCC" {
		t.Fatalf("missing neighbours: %v %v", miss, err)
	}
	if err := d.MarkIdentChecked("AAAAA", 0); err != nil {
		t.Fatal(err)
	}
	miss, _ = d.MissingNeighbors(10)
	if len(miss) != 1 || miss[0] != "CCCCC" {
		t.Fatalf("a name already asked is not asked again: %v", miss)
	}
}
