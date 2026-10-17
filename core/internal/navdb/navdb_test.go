package navdb

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

func sample() *navdata.Airport {
	return &navdata.Airport{
		ICAO: "SBSJ", Region: "SB", Name: "Sao Jose Dos Campos", Lat: -23.228889, Lon: -45.871111, AltitudeM: 632,
		TransAltM: 2438.4, FetchedAt: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC), Source: "msfs-simconnect",
		Runways:     []navdata.Runway{{Lat: -23.2282, Lon: -45.8627, AltitudeM: 632, HeadingTrue: 315, LengthM: 2671, WidthM: 39.6, Surface: 4, PrimaryName: "34", SecondaryNam: "16"}},
		Frequencies: []navdata.Frequency{{Type: 6, Hz: 118500000, Name: "SAO JOSE"}},
		Procedures: []navdata.Procedure{{
			Kind: navdata.KindSID, Name: "BOBI3A",
			Transitions: []navdata.Transition{
				{Kind: navdata.TransRunway, Runway: "34", Legs: []navdata.Leg{
					{Type: 15, FixType: "R", FixICAO: "RW34", FixRegion: "SB", SpeedLimit: -1},
					{Type: 18, FixType: "W", FixICAO: "BOBIC", FixRegion: "SB", FlyOver: true, SpeedLimit: -1},
				}},
				{Kind: navdata.TransEnroute, Name: "DAGUB", Legs: []navdata.Leg{
					{Type: 7, FixType: "W", FixICAO: "VUPSA", FixRegion: "SB", AltDesc: 3, Altitude1M: 1524, SpeedLimit: 210},
				}},
			},
		}},
	}
}

func TestSaveAndLoad(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "nav.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	in := sample()
	if err := d.SaveAirport(in); err != nil {
		t.Fatal(err)
	}
	out, err := d.Airport("sbsj")
	if err != nil {
		t.Fatal(err)
	}
	// the runway leg gets its position from the runway itself; the rest of the data is unchanged
	first := &out.Procedures[0].Transitions[0].Legs[0]
	if first.Lat == nil || first.Lon == nil {
		t.Fatal("RW34 was not placed from the runway")
	}
	first.Lat, first.Lon = nil, nil
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip differs:\n in  %+v\n out %+v", in, out)
	}
	if _, err := d.Airport("SBXX"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing airport: %v", err)
	}
}

func TestSaveReplaces(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "nav.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	a := sample()
	if err := d.SaveAirport(a); err != nil {
		t.Fatal(err)
	}
	a.Procedures = nil
	a.Runways = nil
	if err := d.SaveAirport(a); err != nil {
		t.Fatal(err)
	}
	out, _ := d.Airport("SBSJ")
	if len(out.Procedures) != 0 || len(out.Runways) != 0 {
		t.Fatalf("old rows survived: %+v", out)
	}
	list, _ := d.Airports("SB")
	if len(list) != 1 || list[0].ICAO != "SBSJ" {
		t.Fatalf("list: %+v", list)
	}
}

func TestReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nav.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SaveAirport(sample()); err != nil {
		t.Fatal(err)
	}
	d.Close()
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Airport("SBSJ"); err != nil {
		t.Fatalf("data lost on reopen: %v", err)
	}
}

// With every airport of the simulator stored, the list is read a page at a time.
func TestAirportsPage(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, a := range []struct{ icao, name string }{
		{"SBAA", "Alpha Field"}, {"SBBB", "Bravo Intl"}, {"SBCC", "Charlie"}, {"SBDD", "Delta"}, {"KJFK", "John F Kennedy Alpha"},
	} {
		if err := d.SaveAirport(&navdata.Airport{ICAO: a.icao, Name: a.name}); err != nil {
			t.Fatal(err)
		}
	}
	page, total, err := d.AirportsPage("", 2, 0)
	if err != nil || total != 5 || len(page) != 2 || page[0].ICAO != "KJFK" {
		t.Fatalf("%+v %d %v", page, total, err)
	}
	page, _, _ = d.AirportsPage("", 2, 4)
	if len(page) != 1 || page[0].ICAO != "SBDD" {
		t.Fatalf("the last page: %+v", page)
	}
	if page, total, _ = d.AirportsPage("sb", 10, 0); total != 4 || len(page) != 4 {
		t.Fatalf("by the start of the code: %d %+v", total, page)
	}
	if page, total, _ = d.AirportsPage("alpha", 10, 0); total != 2 {
		t.Fatalf("by the name: %d %+v", total, page)
	}
}
