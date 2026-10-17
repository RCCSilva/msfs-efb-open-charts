package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestChart(t *testing.T) (*Library, string) {
	t.Helper()
	lib, err := Open(t.TempDir())
	require.NoError(t, err)
	meta := `{"id":"t1","name":"SBSP · IAC · ILS X RWY 17R","source":"manual","icao":"SBSP","type":"IAC",
	  "title":"ILS X RWY 17R","validity":"2026-08-06","originalFile":"a.pdf","addedAt":"2026-10-12T00:00:00Z",
	  "image":{"file":"chart.png","width":100,"height":200,"dpi":72},
	  "georef":{"errorMeters":1.5,"metersPerPixel":2.5},
	  "projection":{"kind":"tm"},"toPixel":{"a":1,"b":0,"c":0,"d":0,"e":1,"f":0}}`
	_, err = lib.Add("t1", []byte("png"), []byte(meta), []byte("%PDF-1.4"))
	require.NoError(t, err)
	return lib, "t1"
}

func ptr(s string) *string { return &s }

func TestUpdateCleansTheFieldsAndRegeneratesTheName(t *testing.T) {
	// Arrange
	lib, id := newTestChart(t)

	// Act
	c, err := lib.Update(id, Patch{ICAO: ptr("sbgr"), Title: ptr("  ILS  Z   RWY 09L ")})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "SBGR", c.ICAO)
	assert.Equal(t, "ILS Z RWY 09L", c.Title)
	assert.Equal(t, "IAC", c.Type)
	assert.Equal(t, "SBGR · IAC · ILS Z RWY 09L", c.Name)
}

func TestUpdateKeepsWhatItDidNotTouch(t *testing.T) {
	// Arrange
	lib, id := newTestChart(t)

	// Act
	_, err := lib.Update(id, Patch{ICAO: ptr("sbgr")})

	// Assert
	require.NoError(t, err)
	raw, err := os.ReadFile(filepath.Join(lib.ChartDir(id), "chart.json"))
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	assert.Equal(t, "SBGR", m["icao"], "persisted")
	assert.Equal(t, "manual", m["source"])
	assert.NotNil(t, m["projection"], "the georeference is still there")
	assert.NotNil(t, m["toPixel"])
}

func TestUpdateMakesTheChartFoundByTheNewICAOOnly(t *testing.T) {
	// Arrange
	lib, id := newTestChart(t)

	// Act
	_, err := lib.Update(id, Patch{ICAO: ptr("sbgr")})

	// Assert
	require.NoError(t, err)
	assert.Len(t, lib.Search(Query{Text: "sbgr"}), 1)
	assert.Empty(t, lib.Search(Query{Text: "sbsp"}))
}

func TestUpdateRefusesInvalidValues(t *testing.T) {
	cases := []struct {
		name  string
		p     Patch
		field string
	}{
		{"icao too long", Patch{ICAO: ptr("SBSPX")}, "icao"},
		{"icao symbols", Patch{ICAO: ptr("SB-P")}, "icao"},
		{"type symbols", Patch{Type: ptr("I A C")}, "type"},
		{"validity format", Patch{Validity: ptr("06/08/2026")}, "validity"},
		{"validity impossible date", Patch{Validity: ptr("2026-13-40")}, "validity"},
		{"empty name", Patch{Name: ptr("   ")}, "name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			lib, id := newTestChart(t)

			// Act
			_, err := lib.Update(id, c.p)

			// Assert
			var inv InvalidError
			require.True(t, errors.As(err, &inv), "%v", err)
			assert.Equal(t, c.field, inv.Field)
			got, _ := lib.Get(id)
			assert.Equal(t, "SBSP", got.ICAO, "the rejected patch modified the chart")
			assert.Equal(t, "2026-08-06", got.Validity, "the rejected patch modified the chart")
		})
	}
}

func TestUpdateClearsAndFallsBackToTheOriginalFileName(t *testing.T) {
	// Arrange
	lib, id := newTestChart(t)

	// Act
	c, err := lib.Update(id, Patch{ICAO: ptr(""), Type: ptr(""), Title: ptr(""), Validity: ptr("")})

	// Assert
	require.NoError(t, err)
	assert.Empty(t, c.Validity)
	assert.Empty(t, c.ICAO)
	assert.Equal(t, "A", c.Name, "nothing left to build a name from: the original file name, tidied")
}

func TestUpdateOfAnUnknownChartIsNotFound(t *testing.T) {
	// Arrange
	lib, _ := newTestChart(t)

	// Act
	_, err := lib.Update("missing", Patch{ICAO: ptr("SBSP")})

	// Assert
	assert.True(t, errors.Is(err, os.ErrNotExist), "%v", err)
}
