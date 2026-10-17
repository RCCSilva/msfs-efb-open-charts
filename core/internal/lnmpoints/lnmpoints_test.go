package lnmpoints

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

const sample = `Waypoint,Portão Paulínia,,-22.79139,-47.16056,0,-21.31010,,REA SP,SB,250,2024-12-31T14:57:03.163-03:00,C:/x/reas.csv
Waypoint,Portão UNICAMP,,-22.79250,-47.05361,2001,-21.28289,,REA SP,SB,250,2024-12-31T14:53:58.891-03:00,C:/x/reas.csv
Marker,Torre,TWR1,-23.5,-46.6,2500,0,hill,Uma torre,SB,250,2024-12-31T14:53:58.891-03:00,
not,a,point
`

func TestParseKeepsThePointsAndCountsWhatItSkipped(t *testing.T) {
	// Act
	pts, skipped, err := Parse(strings.NewReader(sample))

	// Assert
	require.NoError(t, err)
	assert.Len(t, pts, 3)
	assert.Equal(t, 1, skipped)
}

func TestParseReadsNameCoordinatesAndNote(t *testing.T) {
	// Act
	pts, _, err := Parse(strings.NewReader(sample))

	// Assert
	require.NoError(t, err)
	require.Len(t, pts, 3)
	assert.Equal(t, "Portão Paulínia", pts[0].Name)
	assert.Equal(t, -22.79139, pts[0].Lat)
	assert.Equal(t, -47.16056, pts[0].Lon)
	assert.Equal(t, "REA SP", pts[0].Note)
}

func TestParseReadsTheElevationIdentTypeAndTags(t *testing.T) {
	// Act
	pts, _, err := Parse(strings.NewReader(sample))

	// Assert
	require.NoError(t, err)
	require.Len(t, pts, 3)
	assert.EqualValues(t, 2001, pts[1].AltFt)
	assert.Equal(t, "TWR1", pts[2].Ident)
	assert.Equal(t, "Marker", pts[2].Type)
	assert.Equal(t, "hill", pts[2].Tags)
}

func TestParseWithHeaderAndBOM(t *testing.T) {
	// Arrange
	in := "\xEF\xBB\xBFType,Name,Ident,Latitude,Longitude,Elevation\nWaypoint,A B,AB,10.5,-20.25,100\n"

	// Act
	pts, skipped, err := Parse(strings.NewReader(in))

	// Assert
	require.NoError(t, err)
	assert.Zero(t, skipped)
	require.Len(t, pts, 1)
	assert.Equal(t, 10.5, pts[0].Lat)
	assert.Equal(t, -20.25, pts[0].Lon)
}

func importedDB(t *testing.T) (*navdb.DB, []navdb.UserPoint) {
	t.Helper()
	pts, _, err := Parse(strings.NewReader(sample))
	require.NoError(t, err)
	db, err := navdb.Open(filepath.Join(t.TempDir(), "n.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db, pts
}

func TestImportAddsEveryPoint(t *testing.T) {
	// Arrange
	db, pts := importedDB(t)

	// Act
	res, err := db.ImportUserPoints(pts)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 3, res.Added)
	assert.Zero(t, res.Skipped)
}

func TestImportingTwiceAddsNothingTheSecondTime(t *testing.T) {
	// Arrange
	db, pts := importedDB(t)
	_, err := db.ImportUserPoints(pts)
	require.NoError(t, err)

	// Act
	again, err := db.ImportUserPoints(pts)

	// Assert
	require.NoError(t, err)
	assert.Zero(t, again.Added)
	assert.Equal(t, 3, again.Skipped)
}

func TestImportMakesIdentifiersOfAtMostTwelveCharacters(t *testing.T) {
	// Arrange
	db, pts := importedDB(t)
	_, err := db.ImportUserPoints(pts)
	require.NoError(t, err)

	// Act
	list, err := db.UserPoints("")

	// Assert
	require.NoError(t, err)
	idents := map[string]string{}
	for _, p := range list {
		idents[p.Ident] = p.Name
	}
	assert.NotContains(t, idents, "PORTAOPAULINI", "longer than 12")
	assert.Equal(t, "Portão Paulínia", idents["PORTAOPAULIN"])
	assert.Equal(t, "Torre", idents["TWR1"])
}
