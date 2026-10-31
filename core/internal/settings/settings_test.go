package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeSettings puts a settings file with this content in a new folder.
func writeSettings(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o644))
	return dir
}

func TestDefaultsAreThePrivatePortAndNoNetworkAccess(t *testing.T) {
	// Act
	s := Load(t.TempDir())

	// Assert
	assert.Equal(t, DefaultPort, s.Port)
	assert.False(t, s.LAN)
	assert.GreaterOrEqual(t, len(s.Token), 20)
	assert.Equal(t, "127.0.0.1:43210", s.ListenAddr())
}

func TestTheDefaultPortIsNotACommonDevelopmentPort(t *testing.T) {
	for _, busy := range []int{80, 3000, 5000, 8000, 8080, 8888, 9000} {
		assert.NotEqual(t, busy, DefaultPort)
	}
}

func TestSettingsSurviveASaveAndALoad(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	in := Settings{Port: 45000, LAN: true, Token: "abc123", Detail: DetailHigh, LargeCharts: LargeBalanced, Basemap: BasemapDark, Aircraft: AircraftHelicopter, AircraftColor: "blue",
		NamePatterns: []string{"{icao}_{type}_{title}"}}

	// Act
	require.NoError(t, in.Save(dir))
	out := Load(dir)

	// Assert
	assert.Equal(t, in, out)
}

func TestNetworkAccessListensOnEveryInterface(t *testing.T) {
	// Arrange
	s := Settings{Port: 45000, LAN: true}

	// Act
	addr := s.ListenAddr()

	// Assert
	assert.Equal(t, "0.0.0.0:45000", addr)
}

func TestAPrivilegedPortInTheFileIsIgnoredAndTheRestIsRead(t *testing.T) {
	// Arrange
	dir := writeSettings(t, `{"port": 80, "lan": true}`)

	// Act
	s := Load(dir)

	// Assert
	assert.Equal(t, DefaultPort, s.Port)
	assert.True(t, s.LAN, "valid fields must still be read")
}

func TestACorruptFileGivesTheDefaults(t *testing.T) {
	// Arrange
	dir := writeSettings(t, `not json`)

	// Act
	s := Load(dir)

	// Assert
	assert.Equal(t, DefaultPort, s.Port)
	assert.False(t, s.LAN)
}

func TestValidatePortRefusesThePrivilegedAndTheOutOfRange(t *testing.T) {
	for _, p := range []int{0, 80, 1023, 65536, -1} {
		assert.Error(t, ValidatePort(p), "port %d", p)
	}
}

func TestValidatePortAcceptsTheUnprivileged(t *testing.T) {
	for _, p := range []int{1024, 8080, 43210, 65535} {
		assert.NoError(t, ValidatePort(p), "port %d", p)
	}
}

func TestTokensAreUnique(t *testing.T) {
	assert.NotEqual(t, NewToken(), NewToken(), "tokens must be random")
}

func TestDetailDefaultsToNormal(t *testing.T) {
	// Act
	s := Load(t.TempDir())

	// Assert
	assert.Equal(t, DetailNormal, s.Detail)
}

func TestDetailGivesTheLongEdgeInPixels(t *testing.T) {
	assert.Equal(t, 3200, LongEdgePx(DetailNormal))
	assert.Equal(t, 4800, LongEdgePx(DetailHigh))
	assert.Equal(t, 3200, LongEdgePx("whatever"))
}

func TestAnUnknownDetailFallsBackToNormal(t *testing.T) {
	// Arrange
	dir := writeSettings(t, `{"port": 43210, "detail": "ultra"}`)

	// Act
	s := Load(dir)

	// Assert
	assert.Equal(t, DetailNormal, s.Detail)
}

func TestBasemapDefaultsToStandard(t *testing.T) {
	// Act
	s := Load(t.TempDir())

	// Assert
	assert.Equal(t, BasemapStandard, s.Basemap)
}

func TestValidBasemap(t *testing.T) {
	for _, ok := range []string{BasemapStandard, BasemapLight, BasemapDark, BasemapNone} {
		assert.True(t, ValidBasemap(ok), ok)
	}
	for _, bad := range []string{"neon", ""} {
		assert.False(t, ValidBasemap(bad), bad)
	}
}

func TestAnUnknownBasemapFallsBackToStandard(t *testing.T) {
	// Arrange
	dir := writeSettings(t, `{"port": 43210, "basemap": "neon"}`)

	// Act
	s := Load(dir)

	// Assert
	assert.Equal(t, BasemapStandard, s.Basemap)
}

func TestAircraftSymbolDefaultsToAYellowPlane(t *testing.T) {
	// Act
	s := Load(t.TempDir())

	// Assert
	assert.Equal(t, AircraftPlane, s.Aircraft)
	assert.Equal(t, "yellow", s.AircraftColor)
}

func TestValidAircraft(t *testing.T) {
	assert.True(t, ValidAircraft(AircraftHelicopter))
	assert.False(t, ValidAircraft("rocket"))
	assert.False(t, ValidAircraft(""))
}

func TestAircraftHexKnowsOnlyTheColorsOfTheProgram(t *testing.T) {
	assert.NotEmpty(t, AircraftHex("yellow"))
	assert.Empty(t, AircraftHex("plaid"))
}

func TestAnUnknownAircraftSymbolFallsBackToTheDefaults(t *testing.T) {
	// Arrange
	dir := writeSettings(t, `{"port":43210,"aircraft":"rocket","aircraftColor":"plaid"}`)

	// Act
	s := Load(dir)

	// Assert
	assert.Equal(t, AircraftPlane, s.Aircraft)
	assert.Equal(t, "yellow", s.AircraftColor)
}

func TestLargeChartsDefaultToBalanced(t *testing.T) {
	// Act
	s := Load(t.TempDir())

	// Assert
	assert.Equal(t, LargeBalanced, s.LargeCharts)
	assert.Equal(t, 70.0, DetailMPerPx(s.LargeCharts))
}

func TestStandardLargeChartsMeanTheAutomaticResolution(t *testing.T) {
	assert.Zero(t, DetailMPerPx(LargeStandard))
}

func TestEachLargeChartsLevelAsksForAGroundResolution(t *testing.T) {
	assert.Equal(t, 70.0, DetailMPerPx(LargeBalanced))
	assert.Equal(t, 35.0, DetailMPerPx(LargeSharp))
	assert.Equal(t, 17.5, DetailMPerPx(LargeMaximum))
	assert.Zero(t, DetailMPerPx("whatever"))
}

func TestAnUnknownLargeChartsLevelFallsBackToTheDefault(t *testing.T) {
	// Arrange
	dir := writeSettings(t, `{"port": 43210, "largeCharts": "ultra"}`)

	// Act
	s := Load(dir)

	// Assert
	assert.Equal(t, LargeBalanced, s.LargeCharts)
}

func TestAStoredStandardLevelIsKept(t *testing.T) {
	// Arrange
	dir := writeSettings(t, `{"port": 43210, "largeCharts": "standard"}`)

	// Act
	s := Load(dir)

	// Assert
	assert.Equal(t, LargeStandard, s.LargeCharts)
}

func TestTheLargeChartsLevelSurvivesARestart(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	s := Default()
	s.LargeCharts = LargeSharp

	// Act
	require.NoError(t, s.Save(dir))

	// Assert
	assert.Equal(t, LargeSharp, Load(dir).LargeCharts)
}

func TestFavoriteSourcesSurviveASaveAndALoad(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	s := Default()
	s.FavoriteSources = []string{"chartfox", "decea-br"}

	// Act
	require.NoError(t, s.Save(dir))
	got := Load(dir)

	// Assert
	assert.Equal(t, []string{"chartfox", "decea-br"}, got.FavoriteSources)
}

func TestNoFavoriteSourcesByDefault(t *testing.T) {
	// Act
	s := Load(t.TempDir())

	// Assert
	assert.Empty(t, s.FavoriteSources)
}
