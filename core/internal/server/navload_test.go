package server

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/simmock"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// loadServer is a server with a new navigation database and the simulator's navigation data given
// by nav.
func loadServer(t *testing.T, nav sim.NavSource) (*Server, *navdb.DB) {
	t.Helper()
	lib, err := store.Open(t.TempDir())
	require.NoError(t, err)
	db, err := navdb.Open(filepath.Join(t.TempDir(), "n.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	s := New(lib, nil, sim.Provider{Nav: nav}, "127.0.0.1:43210")
	s.SetNav(db)
	return s, db
}

func neverStopped() bool { return false }

func failOnFinish(t *testing.T) func(string, string) {
	return func(phase, msg string) { t.Errorf("finished early: %s %s", phase, msg) }
}

// The airports step of the bulk load: the list comes from the simulator, only the missing ones that
// match the prefix are asked, and the ones the simulator does not know are skipped.
func TestLoadAirports(t *testing.T) {
	// Arrange
	nav := new(simmock.Nav)
	nav.On("ListAirports").Return([]sim.AirportRef{
		{ICAO: "SBSJ"}, {ICAO: "SBGR"}, {ICAO: "KJFK"}, {ICAO: "SBXX"}, {ICAO: "SBSJ"},
	}, nil)
	// SBGR is already stored and KJFK does not match the prefix: only these two are asked
	nav.On("Airports", []string{"SBSJ", "SBXX"}).Return([]sim.AirportResult{
		{Airport: &navdata.Airport{ICAO: "SBSJ", Name: "SBSJ", Lat: -23, Lon: -46}},
		{Err: navdata.ErrUnknown},
	}, nil)
	s, db := loadServer(t, nav)
	require.NoError(t, db.SaveAirport(&navdata.Airport{ICAO: "SBGR", Name: "Guarulhos"}))

	// Act
	ok := s.loadAirports(db, "SB", failOnFinish(t), neverStopped)

	// Assert
	require.True(t, ok)
	nav.AssertExpectations(t)
	_, err := db.Airport("SBSJ")
	assert.NoError(t, err, "the airport the simulator knew is stored")
	_, err = db.Airport("KJFK")
	assert.Error(t, err, "KJFK does not match the prefix and is not stored")
	assert.Equal(t, 2, s.nav.job.snapshot().Total)
}

// echoNav answers every airport it is asked for, except SBXX, which it does not know. It remembers
// how many were asked in each batch.
type echoNav struct {
	simmock.Nav
	sizes []int
}

func (e *echoNav) Airports(codes []string) ([]sim.AirportResult, error) {
	e.sizes = append(e.sizes, len(codes))
	out := make([]sim.AirportResult, len(codes))
	for i, c := range codes {
		if c == "SBXX" {
			out[i].Err = navdata.ErrUnknown
			continue
		}
		out[i].Airport = &navdata.Airport{ICAO: c, Name: c}
	}
	return out, nil
}

// The airports are asked many at a time, in the order of the list, and one the simulator does not
// know does not stop the others.
func TestLoadAirportsInBatches(t *testing.T) {
	// Arrange
	var list []sim.AirportRef
	for i := 0; i < 450; i++ {
		list = append(list, sim.AirportRef{ICAO: "SB" + string(rune('A'+i/26/26%26)) + string(rune('A'+i/26%26)) + string(rune('A'+i%26))})
	}
	list = append(list, sim.AirportRef{ICAO: "SBXX"})
	nav := new(echoNav)
	nav.On("ListAirports").Return(list, nil)
	s, db := loadServer(t, nav)

	// Act
	ok := s.loadAirports(db, "SB", failOnFinish(t), neverStopped)

	// Assert
	require.True(t, ok)
	assert.Equal(t, []int{200, 200, 51}, nav.sizes)
	j := s.nav.job.snapshot()
	assert.Equal(t, 451, j.Total)
	assert.Equal(t, 451, j.Done)
	_, err := db.Airport("SBXX")
	assert.Error(t, err, "an unknown airport is not stored")
	_, err = db.Airport(list[10].ICAO)
	assert.NoError(t, err)
}
