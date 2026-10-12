package store

import (
	"fmt"
	"testing"
)

func addChart(t *testing.T, lib *Library, id, icao, typ string) {
	t.Helper()
	meta := fmt.Sprintf(`{"id":%q,"name":"%s · %s","source":"manual","icao":%q,"type":%q,"title":"t","addedAt":"2026-10-12T00:00:00Z",
	  "image":{"file":"chart.png","width":10,"height":10,"dpi":72},"georef":{"errorMeters":1,"metersPerPixel":2},
	  "originalFile":"%s.pdf","projection":{"kind":"tm"},"toPixel":{"a":1,"b":0,"c":0,"d":0,"e":1,"f":0}}`, id, icao, typ, icao, typ, id)
	if _, err := lib.Add(id, []byte("png"), []byte(meta), []byte("%PDF-1.4")); err != nil {
		t.Fatal(err)
	}
}

func TestAirportsAreSummarisedAndFilteredByPrefix(t *testing.T) {
	lib, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	addChart(t, lib, "a1", "SBSP", "IAC")
	addChart(t, lib, "a2", "SBSP", "IAC")
	addChart(t, lib, "a3", "SBSP", "ADC")
	addChart(t, lib, "b1", "SBGR", "SID")
	addChart(t, lib, "v1", "", "REA")

	all := lib.Airports("")
	if len(all) != 3 || all[0].ICAO != "SBGR" || all[1].ICAO != "SBSP" || all[2].ICAO != "" {
		t.Fatalf("airports = %+v", all)
	}
	if all[1].Count != 3 || all[1].Types["IAC"] != 2 || all[1].Types["ADC"] != 1 {
		t.Fatalf("SBSP = %+v", all[1])
	}
	if got := lib.Airports("sbs"); len(got) != 1 || got[0].ICAO != "SBSP" {
		t.Fatalf("prefix sbs = %+v", got)
	}
	if got := lib.Types(); len(got) != 4 {
		t.Fatalf("types = %v", got)
	}
}

func TestDeleteManyRemovesFromGroupsOnce(t *testing.T) {
	lib, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"c1", "c2", "c3"} {
		addChart(t, lib, id, "SBSP", "IAC")
	}
	g := lib.Groups().Current
	for _, id := range []string{"c1", "c2", "c3"} {
		if err := lib.Pin(g, id); err != nil {
			t.Fatal(err)
		}
	}
	n, err := lib.DeleteMany([]string{"c1", "c3", "c3", "does-not-exist"})
	if err != nil || n != 2 {
		t.Fatalf("deleted %d, %v; want 2", n, err)
	}
	if lib.Count() != 1 {
		t.Fatalf("count = %d, want 1", lib.Count())
	}
	pins := lib.Groups().Groups[0].Pins
	if len(pins) != 1 || pins[0] != "c2" {
		t.Fatalf("pins = %v, want [c2]", pins)
	}
	if _, err := lib.DeleteMany([]string{"../x"}); err == nil {
		t.Fatal("an invalid id must be refused")
	}
}
