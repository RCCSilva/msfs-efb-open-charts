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
	assert.GreaterOrEqual(t, len(list), 1000, "identifiers")
}
