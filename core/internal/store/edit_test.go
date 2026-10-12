package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func newTestChart(t *testing.T) (*Library, string) {
	t.Helper()
	lib, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	meta := `{"id":"t1","name":"SBSP · IAC · ILS X RWY 17R","source":"manual","icao":"SBSP","type":"IAC",
	  "title":"ILS X RWY 17R","validity":"2026-08-06","originalFile":"a.pdf","addedAt":"2026-10-12T00:00:00Z",
	  "image":{"file":"chart.png","width":100,"height":200,"dpi":72},
	  "georef":{"errorMeters":1.5,"metersPerPixel":2.5},
	  "projection":{"kind":"tm"},"toPixel":{"a":1,"b":0,"c":0,"d":0,"e":1,"f":0}}`
	if _, err := lib.Add("t1", []byte("png"), []byte(meta), []byte("%PDF-1.4")); err != nil {
		t.Fatal(err)
	}
	return lib, "t1"
}

func ptr(s string) *string { return &s }

func TestUpdateRegeneratesNameAndKeepsTheRest(t *testing.T) {
	lib, id := newTestChart(t)
	c, err := lib.Update(id, Patch{ICAO: ptr("sbgr"), Title: ptr("  ILS  Z   RWY 09L ")})
	if err != nil {
		t.Fatal(err)
	}
	if c.ICAO != "SBGR" || c.Title != "ILS Z RWY 09L" || c.Type != "IAC" {
		t.Fatalf("bad fields: %+v", c)
	}
	if want := "SBGR · IAC · ILS Z RWY 09L"; c.Name != want {
		t.Fatalf("name = %q, want %q", c.Name, want)
	}
	// persisted, and untouched fields (georeference, source, projection) are still there
	raw, _ := os.ReadFile(filepath.Join(lib.ChartDir(id), "chart.json"))
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m["icao"] != "SBGR" || m["source"] != "manual" || m["projection"] == nil || m["toPixel"] == nil {
		t.Fatalf("chart.json lost data: %s", raw)
	}
	// search finds it by the new ICAO and no longer by the old one
	if got := lib.Search(Query{Text: "sbgr"}); len(got) != 1 {
		t.Fatalf("search sbgr = %d results, want 1", len(got))
	}
	if got := lib.Search(Query{Text: "sbsp"}); len(got) != 0 {
		t.Fatalf("search sbsp = %d results, want 0", len(got))
	}
}

func TestUpdateValidation(t *testing.T) {
	lib, id := newTestChart(t)
	bad := []struct {
		name string
		p    Patch
		want string
	}{
		{"icao too long", Patch{ICAO: ptr("SBSPX")}, "icao"},
		{"icao symbols", Patch{ICAO: ptr("SB-P")}, "icao"},
		{"type symbols", Patch{Type: ptr("I A C")}, "type"},
		{"validity format", Patch{Validity: ptr("06/08/2026")}, "validity"},
		{"validity impossible date", Patch{Validity: ptr("2026-13-40")}, "validity"},
		{"empty name", Patch{Name: ptr("   ")}, "name"},
	}
	for _, c := range bad {
		_, err := lib.Update(id, c.p)
		var inv InvalidError
		if !errors.As(err, &inv) || inv.Field != c.want {
			t.Errorf("%s: got %v, want an InvalidError on %q", c.name, err, c.want)
		}
	}
	// nothing was changed by the rejected patches
	if c, _ := lib.Get(id); c.ICAO != "SBSP" || c.Validity != "2026-08-06" {
		t.Fatalf("rejected patches modified the chart: %+v", c)
	}
}

func TestUpdateClearsAndFallsBack(t *testing.T) {
	lib, id := newTestChart(t)
	c, err := lib.Update(id, Patch{ICAO: ptr(""), Type: ptr(""), Title: ptr(""), Validity: ptr("")})
	if err != nil {
		t.Fatal(err)
	}
	if c.Validity != "" || c.ICAO != "" {
		t.Fatalf("fields not cleared: %+v", c)
	}
	if c.Name != "A" { // nothing left to build a name from: the original file name, tidied
		t.Fatalf("fallback name = %q, want %q", c.Name, "A")
	}
	if _, err := lib.Update("missing", Patch{ICAO: ptr("SBSP")}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unknown chart: got %v, want not-exist", err)
	}
}
