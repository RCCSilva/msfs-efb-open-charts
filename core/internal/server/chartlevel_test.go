package server

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// chartDir has a chart.png of 40x20.
func chartDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		for y := 0; y < 20; y++ {
			src.Set(x, y, color.RGBA{uint8(x * 6), uint8(y * 12), 90, 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, src))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chart.png"), buf.Bytes(), 0o644))
	return dir
}

func TestChartLevelIsMadeOnDemandAtHalfTheSize(t *testing.T) {
	// Arrange
	dir := chartDir(t)

	// Act
	p, ok := chartLevelPath(dir, "chart@2.png")

	// Assert
	require.True(t, ok, "level 2 was not made")
	f, err := os.Open(p)
	require.NoError(t, err)
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	require.NoError(t, err)
	assert.Equal(t, 20, cfg.Width)
	assert.Equal(t, 10, cfg.Height)
}

func TestChartLevelRefusesLevelsThatDoNotExist(t *testing.T) {
	// Arrange
	dir := chartDir(t)

	// Act
	_, ok := chartLevelPath(dir, "chart@3.png")

	// Assert
	assert.False(t, ok, "only levels 2 and 4 exist")
}

func TestChartLevelNeverAcceptsAPath(t *testing.T) {
	// Arrange
	dir := chartDir(t)

	// Act
	_, ok := chartLevelPath(dir, "chart@../x.png")

	// Assert
	assert.False(t, ok)
}
