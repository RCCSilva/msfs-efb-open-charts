package server

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// The airports step of the bulk load: the list comes from the simulator, only the missing ones that
// match the prefix are asked, and the ones the simulator does not know are skipped.
func TestLoadAirports(t *testing.T) {
	lib, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := navdb.Open(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(lib, nil, func() Position { return Position{} }, "127.0.0.1:43210")
	var asked []string
	s.SetNav(db, func(icao string) (*navdata.Airport, error) {
		asked = append(asked, icao)
		if icao == "SBXX" {
			return nil, navdata.ErrUnknown
		}
		return &navdata.Airport{ICAO: icao, Name: icao, Lat: -23, Lon: -46}, nil
	}, nil, func([]string) ([]navdata.Fix, error) { return nil, nil })
	s.SetAirportLister(func() ([]AirportRef, error) {
		return []AirportRef{{ICAO: "SBSJ"}, {ICAO: "SBGR"}, {ICAO: "KJFK"}, {ICAO: "SBXX"}, {ICAO: "SBSJ"}}, nil
	})
	// SBGR is already stored: it must not be asked again
	if err := db.SaveAirport(&navdata.Airport{ICAO: "SBGR", Name: "Guarulhos"}); err != nil {
		t.Fatal(err)
	}
	var phase, msg string
	done := make(chan struct{})
	go func() {
		ok := s.loadAirports(db, "SB", func(p, m string) { phase, msg = p, m }, func() bool { return false })
		if !ok {
			t.Errorf("stopped early: %s %s", phase, msg)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
	if len(asked) != 2 || asked[0] != "SBSJ" || asked[1] != "SBXX" {
		t.Fatalf("asked: %v", asked)
	}
	if _, err := db.Airport("SBSJ"); err != nil {
		t.Fatalf("SBSJ not stored: %v", err)
	}
	if _, err := db.Airport("KJFK"); err == nil {
		t.Fatal("KJFK does not match the prefix and must not be stored")
	}
	if s.nav.job.snapshot().Total != 2 {
		t.Fatalf("job: %+v", s.nav.job.snapshot())
	}
}
