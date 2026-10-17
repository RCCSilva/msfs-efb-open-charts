package store

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addChart(t *testing.T, lib *Library, id, icao, typ string) {
	t.Helper()
	meta := fmt.Sprintf(`{"id":%q,"name":"%s · %s","source":"manual","icao":%q,"type":%q,"title":"t","addedAt":"2026-10-12T00:00:00Z",
	  "image":{"file":"chart.png","width":10,"height":10,"dpi":72},"georef":{"errorMeters":1,"metersPerPixel":2},
	  "originalFile":"%s.pdf","projection":{"kind":"tm"},"toPixel":{"a":1,"b":0,"c":0,"d":0,"e":1,"f":0}}`, id, icao, typ, icao, typ, id)
	_, err := lib.Add(id, []byte("png"), []byte(meta), []byte("%PDF-1.4"))
	require.NoError(t, err)
}

// libraryOfAirports has three charts of SBSP, one of SBGR and a visual chart that has no ICAO.
func libraryOfAirports(t *testing.T) *Library {
	t.Helper()
	lib, err := Open(t.TempDir())
	require.NoError(t, err)
	addChart(t, lib, "a1", "SBSP", "IAC")
	addChart(t, lib, "a2", "SBSP", "IAC")
	addChart(t, lib, "a3", "SBSP", "ADC")
	addChart(t, lib, "b1", "SBGR", "SID")
	addChart(t, lib, "v1", "", "REA")
	return lib
}

func TestAirportsListsTheAerodromesWithoutTheRouteCharts(t *testing.T) {
	// Arrange
	lib := libraryOfAirports(t)

	// Act
	all := lib.Airports("")

	// Assert
	// the visual chart has no ICAO: it is a route chart, not part of any aerodrome
	require.Len(t, all, 2)
	assert.Equal(t, "SBGR", all[0].ICAO)
	assert.Equal(t, "SBSP", all[1].ICAO)
}

func TestAirportsCountTheChartsByType(t *testing.T) {
	// Arrange
	lib := libraryOfAirports(t)

	// Act
	all := lib.Airports("")

	// Assert
	require.Len(t, all, 2)
	assert.Equal(t, 3, all[1].Count)
	assert.Equal(t, 2, all[1].Types["IAC"])
	assert.Equal(t, 1, all[1].Types["ADC"])
}

func TestAirportsFilterByPrefix(t *testing.T) {
	// Arrange
	lib := libraryOfAirports(t)

	// Act
	got := lib.Airports("sbs")

	// Assert
	require.Len(t, got, 1)
	assert.Equal(t, "SBSP", got[0].ICAO)
}

func TestTypesListsEveryTypeOfTheLibrary(t *testing.T) {
	// Arrange
	lib := libraryOfAirports(t)

	// Act
	got := lib.Types()

	// Assert
	assert.ElementsMatch(t, []string{"IAC", "ADC", "SID", "REA"}, got)
}

// threePinned has the charts c1, c2 and c3 of SBSP, all pinned to the current group.
func threePinned(t *testing.T) *Library {
	t.Helper()
	lib, err := Open(t.TempDir())
	require.NoError(t, err)
	g := lib.Groups().Current
	for _, id := range []string{"c1", "c2", "c3"} {
		addChart(t, lib, id, "SBSP", "IAC")
		require.NoError(t, lib.Pin(g, id))
	}
	return lib
}

func TestDeleteManyCountsEachChartOnce(t *testing.T) {
	// Arrange
	lib := threePinned(t)

	// Act
	n, err := lib.DeleteMany([]string{"c1", "c3", "c3", "does-not-exist"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, 1, lib.Count())
}

func TestDeleteManyRemovesTheChartsFromTheGroups(t *testing.T) {
	// Arrange
	lib := threePinned(t)

	// Act
	_, err := lib.DeleteMany([]string{"c1", "c3"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"c2"}, lib.Groups().Groups[0].Pins)
}

func TestDeleteManyRefusesAnInvalidID(t *testing.T) {
	// Arrange
	lib := threePinned(t)

	// Act
	_, err := lib.DeleteMany([]string{"../x"})

	// Assert
	assert.Error(t, err)
}

func TestADateGivesTheCycleOfTheChart(t *testing.T) {
	// Arrange
	lib, err := Open(t.TempDir())
	require.NoError(t, err)
	addChart(t, lib, "c1", "SBSP", "IAC")
	v := "2026-08-06"

	// Act
	u, err := lib.Update("c1", Patch{Validity: &v})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "2608", u.Cycle)
	assert.Equal(t, "2026-08-06", u.Validity)
}

func TestACycleStandsForItsEffectiveDate(t *testing.T) {
	// Arrange
	lib, err := Open(t.TempDir())
	require.NoError(t, err)
	addChart(t, lib, "c1", "SBSP", "IAC")
	cy := "2610"

	// Act
	u, err := lib.Update("c1", Patch{Validity: &cy})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "2026-10-01", u.Validity)
	assert.Equal(t, "2610", u.Cycle)
}

func TestADateThatStartsNoCycleHasNoCycle(t *testing.T) {
	// Arrange
	lib, err := Open(t.TempDir())
	require.NoError(t, err)
	addChart(t, lib, "c1", "SBSP", "IAC")
	other := "2026-05-20"

	// Act
	u, err := lib.Update("c1", Patch{Validity: &other})

	// Assert
	require.NoError(t, err)
	assert.Empty(t, u.Cycle, "not the first day of a cycle: the date stays, there is no cycle")
	assert.Equal(t, "2026-05-20", u.Validity)
}

func TestAChartIsFoundByItsCycle(t *testing.T) {
	// Arrange
	lib, err := Open(t.TempDir())
	require.NoError(t, err)
	addChart(t, lib, "c1", "SBSP", "IAC")
	require.Empty(t, lib.Search(Query{Text: "2610"}), "no chart has cycle 2610 yet")
	cy := "2610"

	// Act
	_, err = lib.Update("c1", Patch{Validity: &cy})

	// Assert
	require.NoError(t, err)
	assert.Len(t, lib.Search(Query{Text: "2610"}), 1)
}
