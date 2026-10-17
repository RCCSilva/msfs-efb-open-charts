package buildinfo

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDevelopmentBuildShowsASuffix(t *testing.T) {
	// Act
	got := Get("0.1.1")

	// Assert
	assert.False(t, got.Release)
	assert.True(t, strings.HasPrefix(got.Display, "0.1.1-"), got.Display)
	assert.Equal(t, "0.1.1", got.Version)
}

func TestReleaseBuildShowsTheBareVersion(t *testing.T) {
	// Arrange
	Release = "1"
	t.Cleanup(func() { Release = "" })

	// Act
	got := Get("0.1.1")

	// Assert
	assert.True(t, got.Release)
	assert.Equal(t, "0.1.1", got.Display)
}
