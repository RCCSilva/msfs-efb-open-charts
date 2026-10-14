package lnmpoints

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

const sample = `Waypoint,Portão Paulínia,,-22.79139,-47.16056,0,-21.31010,,REA SP,SB,250,2024-12-31T14:57:03.163-03:00,C:/x/reas.csv
Waypoint,Portão UNICAMP,,-22.79250,-47.05361,2001,-21.28289,,REA SP,SB,250,2024-12-31T14:53:58.891-03:00,C:/x/reas.csv
Marker,Torre,TWR1,-23.5,-46.6,2500,0,hill,Uma torre,SB,250,2024-12-31T14:53:58.891-03:00,
not,a,point
`

func TestParseWithoutHeader(t *testing.T) {
	pts, skipped, err := Parse(strings.NewReader(sample))
	if err != nil {
		t.Fatal(err)
	}
	if len(pts) != 3 || skipped != 1 {
		t.Fatalf("%d points, %d skipped", len(pts), skipped)
	}
	if pts[0].Name != "Portão Paulínia" || pts[0].Lat != -22.79139 || pts[0].Lon != -47.16056 || pts[0].Note != "REA SP" {
		t.Fatalf("first point: %+v", pts[0])
	}
	if pts[1].AltFt != 2001 {
		t.Fatalf("elevation: %+v", pts[1])
	}
	if pts[2].Ident != "TWR1" || pts[2].Type != "Marker" || pts[2].Tags != "hill" {
		t.Fatalf("third point: %+v", pts[2])
	}
}

func TestParseWithHeaderAndBOM(t *testing.T) {
	in := "\xEF\xBB\xBFType,Name,Ident,Latitude,Longitude,Elevation\nWaypoint,A B,AB,10.5,-20.25,100\n"
	pts, skipped, err := Parse(strings.NewReader(in))
	if err != nil || skipped != 0 || len(pts) != 1 || pts[0].Lat != 10.5 || pts[0].Lon != -20.25 {
		t.Fatalf("%+v %d %v", pts, skipped, err)
	}
}

func TestImportMakesIdentsAndIsIdempotent(t *testing.T) {
	pts, _, _ := Parse(strings.NewReader(sample))
	db, err := navdb.Open(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	res, err := db.ImportUserPoints(pts)
	if err != nil || res.Added != 3 || res.Skipped != 0 {
		t.Fatalf("%+v %v", res, err)
	}
	again, err := db.ImportUserPoints(pts)
	if err != nil || again.Added != 0 || again.Skipped != 3 {
		t.Fatalf("second import: %+v %v", again, err)
	}
	list, _ := db.UserPoints("")
	idents := map[string]string{}
	for _, p := range list {
		idents[p.Ident] = p.Name
	}
	if idents["PORTAOPAULINI"] != "" {
		t.Fatalf("identifier longer than 12: %v", idents)
	}
	if idents["PORTAOPAULIN"] != "Portão Paulínia" || idents["TWR1"] != "Torre" {
		t.Fatalf("identifiers: %v", idents)
	}
}
