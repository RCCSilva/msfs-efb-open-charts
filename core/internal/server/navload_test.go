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

// With a batch fetcher the airports are asked many at a time, in the order of the list, and one the
// simulator does not know does not stop the others.
func TestLoadAirportsInBatches(t *testing.T) {
	lib, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := navdb.Open(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := New(lib, nil, func() Position { return Position{} }, "127.0.0.1:43211")
	s.SetNav(db, func(string) (*navdata.Airport, error) { t.Error("the single fetch is not used"); return nil, nil }, nil,
		func([]string) ([]navdata.Fix, error) { return nil, nil })
	var sizes []int
	s.SetAirportBatcher(func(icaos []string) ([]AirportAnswer, error) {
		sizes = append(sizes, len(icaos))
		out := make([]AirportAnswer, len(icaos))
		for i, c := range icaos {
			if c == "SB0050" || c == "SBXX" {
				out[i].Err = navdata.ErrUnknown
				continue
			}
			out[i].Airport = &navdata.Airport{ICAO: c, Name: c}
		}
		return out, nil
	})
	var list []AirportRef
	for i := 0; i < 450; i++ {
		list = append(list, AirportRef{ICAO: "SB" + string(rune('A'+i/26/26%26)) + string(rune('A'+i/26%26)) + string(rune('A'+i%26))})
	}
	list = append(list, AirportRef{ICAO: "SBXX"})
	s.SetAirportLister(func() ([]AirportRef, error) { return list, nil })
	ok := s.loadAirports(db, "SB", func(p, m string) { t.Errorf("finished early: %s %s", p, m) }, func() bool { return false })
	if !ok {
		t.Fatal("stopped early")
	}
	if len(sizes) != 3 || sizes[0] != 200 || sizes[1] != 200 || sizes[2] != 51 {
		t.Fatalf("batches: %v", sizes)
	}
	if j := s.nav.job.snapshot(); j.Total != 451 || j.Done != 451 {
		t.Fatalf("job: %+v", j)
	}
	if _, err := db.Airport("SBXX"); err == nil {
		t.Fatal("an unknown airport is not stored")
	}
	if _, err := db.Airport(list[10].ICAO); err != nil {
		t.Fatalf("stored: %v", err)
	}
}
