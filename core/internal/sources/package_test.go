package sources

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeZip(t *testing.T, members map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pacote-full-20261001.zip")
	f, err := os.Create(path)
	require.NoError(t, err)
	zw := zip.NewWriter(f)
	for name, body := range members {
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, _ = w.Write([]byte(body))
	}
	require.NoError(t, zw.Close())
	require.NoError(t, f.Close())
	return path
}

func deceaZip(t *testing.T) string {
	t.Helper()
	return writeZip(t, map[string]string{
		"SBSP/IAC/sbsp_ils-x-rwy-17r_iac_20261001.pdf": "%PDF a",
		"SBSP/ADC/sbsp_sbsp-adc_adc_20261001.pdf":      "%PDF b",
		`SBGR\IAC\sbgr_vor-rwy-09l_iac_20261001.pdf`:   "%PDF c", // written by a Windows tool
		"README.txt": "not a chart",
	})
}

func TestPackageListsAirportsWhateverTheSeparator(t *testing.T) {
	// Arrange
	path := deceaZip(t)

	// Act
	p, err := openDeceaPackage(path)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 3, p.Charts)
	assert.Len(t, p.Airports, 2)
	assert.Equal(t, "2026-10-01", p.Date)
}

func TestPackageReadsTheChartsOfAnAirport(t *testing.T) {
	// Arrange
	p, err := openDeceaPackage(deceaZip(t))
	require.NoError(t, err)

	// Act
	sel := p.Select("SBGR", nil)

	// Assert
	require.Len(t, sel, 1)
	b, err := p.Read(sel[0])
	require.NoError(t, err)
	assert.Equal(t, "%PDF c", string(b))
}

func TestNotAPackageIsRefused(t *testing.T) {
	// Arrange
	path := writeZip(t, map[string]string{"photo.png": "x"})

	// Act
	_, err := openDeceaPackage(path)

	// Assert
	assert.Equal(t, ErrNotPackage, err)
}
