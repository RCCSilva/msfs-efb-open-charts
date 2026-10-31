package navseed

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnpack38(t *testing.T) {
	// Arrange: "BOBIC" is B=13 O=26 B=13 I=20 C=14 in base 38 (space 0, ? 1, digits 2-11, letters 12-37)
	v := uint64(0)
	for _, c := range "BOBIC" {
		v = v*38 + uint64(indexOf(byte(c)))
	}

	// Act
	got := unpack38(v)

	// Assert
	assert.Equal(t, "BOBIC", got)
}

func indexOf(c byte) int {
	for i := 0; i < len(base38); i++ {
		if base38[i] == c {
			return i
		}
	}
	return -1
}

// With the simulator installed this reads the real files; elsewhere it is skipped.
func TestReadReal(t *testing.T) {
	// Arrange
	paths := Locate()
	if len(paths) == 0 {
		t.Skip("no simulator packages on this computer")
	}

	// Act
	list, err := Read(paths, nil)

	// Assert
	require.NoError(t, err)
	t.Logf("%d archives, %d identifiers", len(paths), len(list))
	assert.GreaterOrEqual(t, len(list), 100000, "identifiers")
}

// The airways are the part of the layout that is easiest to read wrongly without noticing: the names
// are right and the entries are not.
func TestReadFixesReal(t *testing.T) {
	// Arrange
	paths := Locate()
	if len(paths) == 0 {
		t.Skip("no simulator packages on this computer")
	}

	// Act
	fixes, err := Waypoints(paths, nil)

	// Assert
	require.NoError(t, err)
	routes, named, badNeighbours := 0, 0, 0
	for _, f := range fixes {
		routes += len(f.Routes)
		for _, r := range f.Routes {
			if r.Airway != "" && (r.NextIdent != "" || r.PrevIdent != "") {
				named++
			}
			if (r.NextIdent != "" && !validRegion(r.NextRegion)) || (r.PrevIdent != "" && !validRegion(r.PrevRegion)) {
				badNeighbours++
			}
		}
	}
	t.Logf("%d waypoints, %d airway entries", len(fixes), routes)
	assert.GreaterOrEqual(t, len(fixes), 80000, "waypoints")
	assert.GreaterOrEqual(t, routes, 50000, "airway entries")
	assert.Equal(t, routes, named, "every entry has an airway name and a neighbour")
	assert.Zero(t, badNeighbours, "airways lead to fixes with a usable region")
	for _, f := range fixes {
		require.True(t, validRegion(f.Region), f.Ident+" "+f.Region)
	}
}
