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
