package efbapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// community is an empty folder named Community.
func community(t *testing.T) string {
	t.Helper()
	c := filepath.Join(t.TempDir(), "Community")
	require.NoError(t, os.MkdirAll(c, 0o755))
	return c
}

func TestTheEFBAppIsNotInstalledInAnEmptyCommunityFolder(t *testing.T) {
	// Arrange
	c := community(t)

	// Act
	st := EFBState(c)

	// Assert
	assert.False(t, st.Installed)
}

func TestInstallingTheEFBAppLeavesItUpToDate(t *testing.T) {
	// Arrange
	c := community(t)

	// Act
	err := InstallEFB(c, "http://127.0.0.1:9000")

	// Assert
	require.NoError(t, err)
	st := EFBState(c)
	assert.True(t, st.Installed)
	assert.True(t, st.UpToDate)
	assert.Equal(t, EFBVersion, st.Version)
}

func TestInstallingTheEFBAppWritesTheServerURL(t *testing.T) {
	// Arrange
	c := community(t)

	// Act
	require.NoError(t, InstallEFB(c, "http://127.0.0.1:9000"))

	// Assert
	js, err := os.ReadFile(filepath.Join(c, EFBPackageName, "html_ui/efb_ui/efb_apps/OpenCharts/OpenCharts.js"))
	require.NoError(t, err)
	assert.Contains(t, string(js), "var SERVER_URL = 'http://127.0.0.1:9000/';")
}

func TestTheLayoutPathsAreLowercase(t *testing.T) {
	// Arrange
	c := community(t)

	// Act
	require.NoError(t, InstallEFB(c, "http://127.0.0.1:9000"))

	// Assert
	lay, err := os.ReadFile(filepath.Join(c, EFBPackageName, "layout.json"))
	require.NoError(t, err)
	assert.Contains(t, string(lay), "html_ui/efb_ui/efb_apps/opencharts/opencharts.js")
}

func TestRemovingTheEFBApp(t *testing.T) {
	// Arrange
	c := community(t)
	require.NoError(t, InstallEFB(c, "http://127.0.0.1:9000"))

	// Act
	err := RemoveEFB(c)

	// Assert
	require.NoError(t, err)
	assert.False(t, EFBState(c).Installed)
}

func TestRemovingRefusesAFolderThatIsNotCommunity(t *testing.T) {
	// Arrange
	other := filepath.Join(t.TempDir(), "Other")

	// Act
	err := RemoveEFB(other)

	// Assert
	assert.Error(t, err)
}

const otherAddon = "<?xml version=\"1.0\" encoding=\"Windows-1252\"?>\r\n<SimBase.Document Type=\"Launch\" version=\"1,0\">\r\n" +
	"\t<Launch.Addon>\r\n\t\t<Name>Other Tool</Name>\r\n\t\t<Path>C:/x/other.exe</Path>\r\n\t</Launch.Addon>\r\n</SimBase.Document>\r\n"

// exeXML is an EXE.xml that has another addon in it.
func exeXML(t *testing.T) string {
	t.Helper()
	x := filepath.Join(t.TempDir(), "EXE.xml")
	require.NoError(t, os.WriteFile(x, []byte(otherAddon), 0o644))
	return x
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func TestAutoStartIsAddedAndTheOtherAddonsStay(t *testing.T) {
	// Arrange
	x := exeXML(t)

	// Act
	err := SetAutoStart(x, `C:\Apps\opencharts.exe`, []string{"-data", `C:\d`}, true)

	// Assert
	require.NoError(t, err)
	assert.True(t, AutoStartEnabled(x))
	assert.Contains(t, readFile(t, x), "Other Tool")
}

func TestAutoStartTwiceDoesNotDuplicateTheEntry(t *testing.T) {
	// Arrange
	x := exeXML(t)
	require.NoError(t, SetAutoStart(x, `C:\Apps\opencharts.exe`, nil, true))

	// Act
	err := SetAutoStart(x, `C:\Apps\opencharts.exe`, nil, true)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 1, countOf(readFile(t, x), "<Name>Open Charts</Name>"))
}

func countOf(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}

func TestAutoStartRemovedLeavesTheOtherAddonsAndABackup(t *testing.T) {
	// Arrange
	x := exeXML(t)
	require.NoError(t, SetAutoStart(x, `C:\Apps\opencharts.exe`, nil, true))

	// Act
	err := SetAutoStart(x, "", nil, false)

	// Assert
	require.NoError(t, err)
	assert.False(t, AutoStartEnabled(x))
	assert.Contains(t, readFile(t, x), "Other Tool")
	assert.FileExists(t, x+".opencharts.bak")
}

func TestAutoStartRefusesAPathWithNonASCIICharacters(t *testing.T) {
	// Arrange
	x := exeXML(t)

	// Act
	err := SetAutoStart(x, "C:/Usu\u00e1rio/o.exe", nil, true)

	// Assert
	assert.Error(t, err)
}
