package navdb

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

func TestFixesPlaceLegs(t *testing.T) {
	d, err := Open(filepath.Join(t.TempDir(), "nav.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err := d.SaveAirport(sample()); err != nil {
		t.Fatal(err)
	}
	missing, err := d.MissingFixes("SBSJ")
	if err != nil {
		t.Fatal(err)
	}
	// BOBIC and VUPSA are waypoints; RW34 is a runway and is placed from the airport, not asked for
	if len(missing) != 2 {
		t.Fatalf("missing: %+v", missing)
	}
	if err := d.SaveFixes([]navdata.Fix{
		{Ident: "BOBIC", Region: "SB", Kind: 'W', Lat: -23.1, Lon: -45.9, Routes: []navdata.Route{
			{Airway: "UZ6", Type: 1, NextIdent: "VUPSA", NextRegion: "SB", NextKind: 'W', NextLat: -22.9, NextLon: -45.5},
		}},
		{Ident: "VUPSA", Region: "SB", Kind: 'W', Lat: -22.9, Lon: -45.5},
	}); err != nil {
		t.Fatal(err)
	}
	if missing, _ = d.MissingFixes("SBSJ"); len(missing) != 0 {
		t.Fatalf("still missing: %+v", missing)
	}
	a, err := d.Airport("SBSJ")
	if err != nil {
		t.Fatal(err)
	}
	for _, tr := range a.Procedures[0].Transitions {
		for _, l := range tr.Legs {
			if l.Lat == nil || l.Lon == nil {
				t.Fatalf("leg %s without a position", l.FixICAO)
			}
		}
	}
	bobic := a.Procedures[0].Transitions[0].Legs[1]
	if *bobic.Lat != -23.1 || *bobic.Lon != -45.9 {
		t.Fatalf("BOBIC at %v,%v", *bobic.Lat, *bobic.Lon)
	}

	// RW34 is the primary end: half a runway length (1335 m) behind the middle, against the heading
	rw := a.Procedures[0].Transitions[0].Legs[0]
	dLat := (*rw.Lat - a.Runways[0].Lat) * 111320
	dLon := (*rw.Lon - a.Runways[0].Lon) * 111320 * math.Cos(a.Runways[0].Lat*math.Pi/180)
	dist := math.Hypot(dLat, dLon)
	if math.Abs(dist-a.Runways[0].LengthM/2) > 5 {
		t.Fatalf("RW34 is %.0f m from the middle, want %.0f", dist, a.Runways[0].LengthM/2)
	}
	// heading 315: the primary threshold is to the south-east of the middle
	if !(dLat < 0 && dLon > 0) {
		t.Fatalf("RW34 on the wrong side: dLat %.0f dLon %.0f", dLat, dLon)
	}

	f, err := d.Fix("bobic", "sb")
	if err != nil || len(f.Routes) != 1 || f.Routes[0].Airway != "UZ6" || f.Routes[0].NextIdent != "VUPSA" {
		t.Fatalf("fix with airways: %+v %v", f, err)
	}
}
