package navdb

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const notADatabase = "this is not a database, it is only text that is long enough to look like a file"

func TestGarbageFileIsCorrupt(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "navdata.db")
	require.NoError(t, os.WriteFile(path, []byte(notADatabase), 0o644))

	// Act
	_, err := Open(path)

	// Assert
	assert.True(t, errors.Is(err, ErrCorrupt), "%v", err)
}

func TestDamagedPageMakesTheFileCorrupt(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "navdata.db")
	d, err := Open(path)
	require.NoError(t, err)
	for i := 0; i < 200; i++ { // enough rows to spread over several pages
		require.NoError(t, d.SaveAirport(sample()))
		_, err := d.db.Exec(`INSERT INTO user_point (ident, name, lat, lon) VALUES (?,?,0,0)`, fmt.Sprintf("P%d", i), "a point with a name that takes room in the page")
		require.NoError(t, err)
	}
	d.Close()
	fi, err := os.Stat(path)
	require.NoError(t, err)
	require.GreaterOrEqual(t, fi.Size(), int64(3*4096))
	// the second page is overwritten with something that is not a page of this database
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	junk := make([]byte, 4096)
	for i := range junk {
		junk[i] = 0xAB
	}
	_, err = f.WriteAt(junk, 4096)
	require.NoError(t, err)
	f.Close()

	// Act
	_, err = Open(path)

	// Assert
	assert.True(t, errors.Is(err, ErrCorrupt), "a page of the file is damaged: %v", err)
}

// setAsideFixture has a damaged file, its log, and two older damaged files already set aside.
func setAsideFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "navdata.db")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "backups"), 0o755))
	for i := 0; i < 2; i++ {
		old := filepath.Join(dir, "backups", fmt.Sprintf("navdata-corrupt-2020010%d-000000.db", i+1))
		require.NoError(t, os.WriteFile(old, []byte("x"), 0o644))
		when := time.Now().Add(-time.Duration(48-i) * time.Hour)
		_ = os.Chtimes(old, when, when)
	}
	require.NoError(t, os.WriteFile(path, []byte("damaged"), 0o644))
	require.NoError(t, os.WriteFile(path+"-wal", []byte("w"), 0o644))
	return path
}

func TestSetAsideMovesTheDamagedFileAndItsLogOutOfThePlace(t *testing.T) {
	// Arrange
	path := setAsideFixture(t)

	// Act
	_, err := SetAside(path)

	// Assert
	require.NoError(t, err)
	assert.NoFileExists(t, path, "the damaged file is gone from its place")
	assert.NoFileExists(t, path+"-wal", "and so is the log beside it")
}

func TestSetAsideKeepsTheNewestDamagedFile(t *testing.T) {
	// Arrange
	path := setAsideFixture(t)

	// Act
	to, err := SetAside(path)

	// Assert
	require.NoError(t, err)
	list, _ := ListBackups(path)
	require.Len(t, list, 1, "only the newest damaged file is kept")
	assert.Equal(t, filepath.Base(to), list[0].Name)
	assert.Equal(t, "corrupt", list[0].Kind)
}

func TestListBackupsWithoutTheFolderIsEmpty(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "navdata.db")

	// Act
	list, err := ListBackups(path)

	// Assert
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestABackupIsListedWithItsKindAndSize(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "navdata.db")
	d, err := Open(path)
	require.NoError(t, err)
	defer d.Close()
	made, err := d.Backup("manual")
	require.NoError(t, err)

	// Act
	list, err := ListBackups(path)

	// Assert
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "manual", list[0].Kind)
	assert.Equal(t, filepath.Base(made), list[0].Name)
	assert.NotZero(t, list[0].Size)
}

// Only the program's own names are reachable: no folders, nothing else in the folder.
func TestOnlyTheProgramsOwnBackupNamesAreReachable(t *testing.T) {
	for _, bad := range []string{"../navdata.db", "notes.txt", "navdata-manual-1.db", `..\navdata.db`, ""} {
		t.Run(bad, func(t *testing.T) {
			// Arrange
			path := filepath.Join(t.TempDir(), "navdata.db")
			d, err := Open(path)
			require.NoError(t, err)
			defer d.Close()
			_, err = d.Backup("manual")
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(filepath.Join(backupDir(path), "notes.txt"), []byte("x"), 0o644))

			// Act
			_, err = BackupFile(path, bad)

			// Assert
			assert.True(t, errors.Is(err, ErrNoBackup), "%v", err)
		})
	}
}

func TestDeletingABackupRemovesItFromTheList(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "navdata.db")
	d, err := Open(path)
	require.NoError(t, err)
	defer d.Close()
	_, err = d.Backup("manual")
	require.NoError(t, err)
	list, _ := ListBackups(path)
	require.Len(t, list, 1)

	// Act
	err = DeleteBackup(path, list[0].Name)

	// Assert
	require.NoError(t, err)
	left, _ := ListBackups(path)
	assert.Empty(t, left)
}

// A copy made by Backup can take the place of the database: it is inspected, installed and opened.
// afterCopy is a database with a copy made, and then changed: the airport removed and a point made.
func afterCopy(t *testing.T) (path, made string) {
	t.Helper()
	path = filepath.Join(t.TempDir(), "navdata.db")
	d, err := Open(path)
	require.NoError(t, err)
	require.NoError(t, d.SaveAirport(sample()))
	made, err = d.Backup("manual")
	require.NoError(t, err)
	require.NoError(t, d.DeleteAirport("SBSJ"))
	_, err = d.db.Exec(`INSERT INTO user_point (ident, name, lat, lon) VALUES ('LATER','x',1,1)`)
	require.NoError(t, err)
	d.Close()
	return path, made
}

func installCopy(t *testing.T, path, made string) {
	t.Helper()
	incoming, err := PrepareIncoming(path, made)
	require.NoError(t, err)
	v, err := Inspect(incoming)
	require.NoError(t, err)
	assert.Equal(t, latestVersion(), v)
	require.NoError(t, InstallIncoming(path))
}

func TestRestoringACopyBringsTheDataBack(t *testing.T) {
	// Arrange
	path, made := afterCopy(t)

	// Act
	installCopy(t, path, made)

	// Assert
	d, err := Open(path)
	require.NoError(t, err)
	defer d.Close()
	_, err = d.Airport("SBSJ")
	assert.NoError(t, err, "the airport is back")
	assert.Zero(t, count(t, d, "user_point"), "what was done after the copy is gone")
}

func TestRestoringACopyDoesNotConsumeIt(t *testing.T) {
	// Arrange
	path, made := afterCopy(t)

	// Act
	installCopy(t, path, made)

	// Assert
	assert.FileExists(t, made, "the backup the user keeps is not consumed")
}

func TestInspectAcceptsAnOldFileAndMigratesItWhenInstalled(t *testing.T) {
	// Arrange
	legacy := legacyFile(t, "3")

	// Act
	v, err := Inspect(legacy)

	// Assert
	require.NoError(t, err)
	assert.Zero(t, v)
}

func TestInspectRefusesWhatIsNotADatabase(t *testing.T) {
	// Arrange
	garbage := filepath.Join(t.TempDir(), "x.db")
	require.NoError(t, os.WriteFile(garbage, []byte(notADatabase), 0o644))

	// Act
	_, err := Inspect(garbage)

	// Assert
	assert.True(t, errors.Is(err, ErrCorrupt), "%v", err)
}

func TestInspectRefusesAHealthySQLiteFileThatIsSomethingElse(t *testing.T) {
	// Arrange
	other := filepath.Join(t.TempDir(), "other.db")
	db, err := sql.Open("sqlite", "file:"+other)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE contacts (name TEXT)`)
	require.NoError(t, err)
	db.Close()

	// Act
	_, err = Inspect(other)

	// Assert
	assert.True(t, errors.Is(err, ErrNotNavigation), "%v", err)
}

func TestInspectRefusesAFileOfANewerVersion(t *testing.T) {
	// Arrange
	newer := legacyFile(t, "3")
	db, err := sql.Open("sqlite", "file:"+newer)
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, latestVersion()+1))
	require.NoError(t, err)
	db.Close()

	// Act
	_, err = Inspect(newer)

	// Assert
	assert.True(t, errors.Is(err, ErrNewerSchema), "%v", err)
}
