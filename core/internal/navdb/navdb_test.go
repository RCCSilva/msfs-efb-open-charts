package navdb

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// openTemp opens a new database in a temporary folder and closes it with the test.
func openTemp(t *testing.T) *DB {
	t.Helper()
	d, err := Open(filepath.Join(t.TempDir(), "nav.db"))
	require.NoError(t, err)
	t.Cleanup(func() { d.Close() })
	return d
}

func TestSaveAndLoadKeepTheAirportAsItWas(t *testing.T) {
	// Arrange
	d := openTemp(t)
	in := sample()
	require.NoError(t, d.SaveAirport(in))

	// Act
	out, err := d.Airport("sbsj")

	// Assert
	require.NoError(t, err)
	// the runway leg gets its position from the runway itself; the rest of the data is unchanged
	first := &out.Procedures[0].Transitions[0].Legs[0]
	require.NotNil(t, first.Lat, "RW34 was not placed from the runway")
	require.NotNil(t, first.Lon)
	first.Lat, first.Lon = nil, nil
	assert.Equal(t, in, out)
}

func TestAMissingAirportIsNotFound(t *testing.T) {
	// Arrange
	d := openTemp(t)

	// Act
	_, err := d.Airport("SBXX")

	// Assert
	assert.True(t, errors.Is(err, ErrNotFound), "%v", err)
}

func TestSavingAgainReplacesTheRows(t *testing.T) {
	// Arrange
	d := openTemp(t)
	a := sample()
	require.NoError(t, d.SaveAirport(a))
	a.Procedures, a.Runways = nil, nil

	// Act
	require.NoError(t, d.SaveAirport(a))

	// Assert
	out, err := d.Airport("SBSJ")
	require.NoError(t, err)
	assert.Empty(t, out.Procedures, "old rows survived")
	assert.Empty(t, out.Runways, "old rows survived")
	list, err := d.Airports("SB")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "SBSJ", list[0].ICAO)
}

func TestReopeningKeepsTheData(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "nav.db")
	d, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, d.SaveAirport(sample()))
	d.Close()

	// Act
	d, err = Open(path)

	// Assert
	require.NoError(t, err)
	defer d.Close()
	_, err = d.Airport("SBSJ")
	assert.NoError(t, err, "data lost on reopen")
}

// With every airport of the simulator stored, the list is read a page at a time.
func airportsForPaging(t *testing.T) *DB {
	t.Helper()
	d := openTemp(t)
	for _, a := range []struct{ icao, name string }{
		{"SBAA", "Alpha Field"}, {"SBBB", "Bravo Intl"}, {"SBCC", "Charlie"}, {"SBDD", "Delta"}, {"KJFK", "John F Kennedy Alpha"},
	} {
		require.NoError(t, d.SaveAirport(&navdata.Airport{ICAO: a.icao, Name: a.name}))
	}
	return d
}

func TestAirportsPageGivesTheFirstPageAndTheTotal(t *testing.T) {
	// Arrange
	d := airportsForPaging(t)

	// Act
	page, total, err := d.AirportsPage("", 2, 0)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	require.Len(t, page, 2)
	assert.Equal(t, "KJFK", page[0].ICAO)
}

func TestAirportsPageGivesTheLastPageShort(t *testing.T) {
	// Arrange
	d := airportsForPaging(t)

	// Act
	page, _, err := d.AirportsPage("", 2, 4)

	// Assert
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, "SBDD", page[0].ICAO)
}

func TestAirportsPageFiltersByTheStartOfTheCodeOrByTheName(t *testing.T) {
	cases := map[string]struct {
		q    string
		want int
	}{
		"the start of the code": {"sb", 4},
		"the name":              {"alpha", 2},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			d := airportsForPaging(t)

			// Act
			page, total, err := d.AirportsPage(c.q, 10, 0)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, c.want, total)
			assert.Len(t, page, c.want)
		})
	}
}
