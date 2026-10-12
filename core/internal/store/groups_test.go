package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOldFlightsFileBecomesGroups(t *testing.T) {
	dir := t.TempDir()
	old := `{"current":"flight-9","flights":[{"id":"flight-9","name":"SBSP-SBGL","pins":["a","b"],"created":"2026-10-01T00:00:00Z"}]}`
	if err := os.WriteFile(filepath.Join(dir, "flights.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	lib, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	st := lib.Groups()
	if len(st.Groups) != 1 || st.Groups[0].Name != "SBSP-SBGL" || len(st.Groups[0].Pins) != 2 || st.Current != "flight-9" {
		t.Fatalf("flights.json not migrated: %+v", st)
	}
	if _, err := lib.CreateGroup("Alternate"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "groups.json")); err != nil {
		t.Fatalf("groups.json not written: %v", err)
	}
	// from now on groups.json wins
	lib2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(lib2.Groups().Groups) != 2 {
		t.Fatalf("want 2 groups, got %+v", lib2.Groups())
	}
}

func TestDefaultGroupExistsAndLastCannotBeDeleted(t *testing.T) {
	lib, _ := newTestChart(t)
	st := lib.Groups()
	if len(st.Groups) != 1 || st.Current != st.Groups[0].ID {
		t.Fatalf("want one current group, got %+v", st)
	}
	if err := lib.DeleteGroup(st.Groups[0].ID); err == nil {
		t.Fatal("the only group must not be deletable")
	}
}

func TestPinAndDeleteChartLeavesGroups(t *testing.T) {
	lib, id := newTestChart(t)
	g, err := lib.CreateGroup("Origin")
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.Pin(g.ID, id); err != nil {
		t.Fatal(err)
	}
	if err := lib.Delete(id); err != nil {
		t.Fatal(err)
	}
	for _, gr := range lib.Groups().Groups {
		for _, p := range gr.Pins {
			if p == id {
				t.Fatalf("deleted chart still pinned in %s", gr.Name)
			}
		}
	}
}
