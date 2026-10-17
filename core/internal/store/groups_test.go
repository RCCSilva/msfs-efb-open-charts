package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// oldFlightsDir has the file the program wrote before groups existed.
func oldFlightsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := `{"current":"flight-9","flights":[{"id":"flight-9","name":"SBSP-SBGL","pins":["a","b"],"created":"2026-10-01T00:00:00Z"}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "flights.json"), []byte(old), 0o644))
	return dir
}

func TestOldFlightsFileBecomesGroups(t *testing.T) {
	// Arrange
	dir := oldFlightsDir(t)

	// Act
	lib, err := Open(dir)

	// Assert
	require.NoError(t, err)
	st := lib.Groups()
	require.Len(t, st.Groups, 1)
	assert.Equal(t, "SBSP-SBGL", st.Groups[0].Name)
	assert.Len(t, st.Groups[0].Pins, 2)
	assert.Equal(t, "flight-9", st.Current)
}

func TestGroupsFileWinsOverTheOldFlightsFile(t *testing.T) {
	// Arrange
	dir := oldFlightsDir(t)
	lib, err := Open(dir)
	require.NoError(t, err)
	_, err = lib.CreateGroup("Alternate")
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(dir, "groups.json"))

	// Act
	lib2, err := Open(dir)

	// Assert
	require.NoError(t, err)
	assert.Len(t, lib2.Groups().Groups, 2)
}

func TestADefaultGroupExistsAndIsCurrent(t *testing.T) {
	// Arrange
	lib, _ := newTestChart(t)

	// Act
	st := lib.Groups()

	// Assert
	require.Len(t, st.Groups, 1)
	assert.Equal(t, st.Groups[0].ID, st.Current)
}

func TestTheLastGroupCannotBeDeleted(t *testing.T) {
	// Arrange
	lib, _ := newTestChart(t)
	only := lib.Groups().Groups[0].ID

	// Act
	err := lib.DeleteGroup(only)

	// Assert
	assert.Error(t, err)
}

func TestDeletingAChartUnpinsItFromTheGroups(t *testing.T) {
	// Arrange
	lib, id := newTestChart(t)
	g, err := lib.CreateGroup("Origin")
	require.NoError(t, err)
	require.NoError(t, lib.Pin(g.ID, id))

	// Act
	err = lib.Delete(id)

	// Assert
	require.NoError(t, err)
	for _, gr := range lib.Groups().Groups {
		assert.NotContains(t, gr.Pins, id, "deleted chart still pinned in %s", gr.Name)
	}
}
