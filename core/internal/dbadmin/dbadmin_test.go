package dbadmin_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/dbadmin"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb/navdbmock"
)

const path = "/data/navdata.db"

var errBoom = errors.New("boom")

// fixture is a manager with its mocks. Replace runs the change at once, as the program's start-up
// code does between closing the database and opening it again.
type fixture struct {
	db       *navdbmock.Database
	disk     *navdbmock.Disk
	replaced int
	replace  func(change func() error) error
	busy     bool
	problem  *dbadmin.Problem
}

func newFixture() *fixture {
	f := &fixture{db: new(navdbmock.Database), disk: new(navdbmock.Disk)}
	f.replace = func(change func() error) error {
		f.replaced++
		return change()
	}
	return f
}

// manager builds the manager; withDB false is a database that cannot be used.
func (f *fixture) manager(withDB bool) *dbadmin.Manager {
	d := dbadmin.Deps{Path: path, Disk: f.disk, Replace: f.replace, Busy: func() bool { return f.busy }, Problem: f.problem}
	if withDB {
		d.DB = f.db
	}
	return dbadmin.New(d)
}

func TestStatusOfAHealthyDatabase(t *testing.T) {
	// Arrange
	f := newFixture()
	f.disk.On("Size", path).Return(1234, true)
	f.disk.On("List", path).Return([]navdb.BackupInfo{{Name: "navdata-manual-20260101-000000.db", Kind: "manual"}}, nil)
	f.db.On("SchemaVersion").Return(1, nil)
	f.db.On("Migrated").Return(navdb.Migration{From: 0, To: 1})
	f.busy = true

	// Act
	st := f.manager(true).Status()

	// Assert
	assert.True(t, st.OK)
	assert.Equal(t, path, st.Path)
	assert.EqualValues(t, 1234, st.Size)
	require.NotNil(t, st.Schema)
	assert.Equal(t, 1, *st.Schema)
	assert.Equal(t, navdb.LatestSchema(), st.LatestSchema)
	assert.Equal(t, navdb.Migration{From: 0, To: 1}, *st.Migrated)
	assert.Len(t, st.Backups, 1)
	assert.True(t, st.Loading)
	assert.Nil(t, st.Problem)
}

func TestStatusOfADatabaseThatCannotBeUsed(t *testing.T) {
	// Arrange
	f := newFixture()
	f.problem = &dbadmin.Problem{Kind: "corrupt", Detail: "file is not a database"}
	f.disk.On("Size", path).Return(0, false)
	f.disk.On("List", path).Return(nil, nil)

	// Act
	st := f.manager(false).Status()

	// Assert
	assert.False(t, st.OK)
	assert.Equal(t, "corrupt", st.Problem.Kind)
	assert.Nil(t, st.Schema)
	assert.Nil(t, st.Migrated)
	assert.NotNil(t, st.Backups, "an empty list, not nothing")
	assert.Empty(t, st.Backups)
}

func TestBackupMakesACopyTheUserAskedFor(t *testing.T) {
	// Arrange
	f := newFixture()
	f.db.On("Backup", "manual").Return("/data/backups/navdata-manual-x.db", nil)

	// Act
	file, err := f.manager(true).Backup()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/data/backups/navdata-manual-x.db", file)
}

func TestBackupOfADatabaseNotInUse(t *testing.T) {
	// Arrange
	f := newFixture()

	// Act
	_, err := f.manager(false).Backup()

	// Assert
	assert.ErrorIs(t, err, dbadmin.ErrNoDatabase)
}

func TestNothingIsDoneWhenTheFileIsNotManaged(t *testing.T) {
	// Arrange
	m := dbadmin.New(dbadmin.Deps{Disk: new(navdbmock.Disk)}) // no path, no way to reopen

	// Act
	_, backupErr := m.Backup()
	_, resetErr := m.Reset()
	deleteErr := m.DeleteBackup("navdata-manual-20260101-000000.db")

	// Assert
	assert.ErrorIs(t, backupErr, dbadmin.ErrNotManaged)
	assert.ErrorIs(t, resetErr, dbadmin.ErrNotManaged)
	assert.ErrorIs(t, deleteErr, dbadmin.ErrNotManaged)
}

func TestResetCopiesTheDatabaseFirstThenRemovesIt(t *testing.T) {
	// Arrange
	f := newFixture()
	f.db.On("Backup", "reset").Return("/data/backups/navdata-reset-x.db", nil)
	f.disk.On("Remove", path).Return(nil)

	// Act
	out, err := f.manager(true).Reset()

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/data/backups/navdata-reset-x.db", out.Backup)
	assert.Equal(t, 1, f.replaced)
	f.disk.AssertCalled(t, "Remove", path)
	f.disk.AssertNotCalled(t, "SetAside", mock.Anything)
}

func TestResetOfADamagedDatabaseSetsItAsideInsteadOfRemovingIt(t *testing.T) {
	// Arrange
	f := newFixture()
	f.disk.On("SetAside", path).Return("/data/backups/navdata-corrupt-x.db", nil)

	// Act
	out, err := f.manager(false).Reset()

	// Assert
	require.NoError(t, err)
	assert.Empty(t, out.Backup, "there is no copy of a database that is not in use")
	f.disk.AssertCalled(t, "SetAside", path)
	f.disk.AssertNotCalled(t, "Remove", mock.Anything)
}

func TestResetDoesNothingWhileALoadRuns(t *testing.T) {
	// Arrange
	f := newFixture()
	f.busy = true

	// Act
	_, err := f.manager(true).Reset()

	// Assert
	assert.ErrorIs(t, err, dbadmin.ErrBusy)
	assert.Zero(t, f.replaced)
	f.db.AssertNotCalled(t, "Backup", mock.Anything)
}

func TestResetDoesNotGoOnWithoutTheCopy(t *testing.T) {
	// Arrange
	f := newFixture()
	f.db.On("Backup", "reset").Return("", errBoom)

	// Act
	_, err := f.manager(true).Reset()

	// Assert
	assert.ErrorIs(t, err, errBoom)
	assert.Contains(t, err.Error(), "could not copy the database first")
	assert.Zero(t, f.replaced, "what was there is still there")
}

func TestRestoreChecksCopiesAndSwapsAFile(t *testing.T) {
	// Arrange
	name := "navdata-manual-20260101-000000.db"
	f := newFixture()
	f.disk.On("File", path, name).Return("/data/backups/"+name, nil)
	f.disk.On("Prepare", path, "/data/backups/"+name).Return(path+".incoming", nil)
	f.disk.On("InspectIncoming", path).Return(1, nil)
	f.db.On("Backup", "restore").Return("/data/backups/navdata-restore-x.db", nil)
	f.disk.On("Install", path).Return(nil)

	// Act
	out, err := f.manager(true).Restore(name)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/data/backups/navdata-restore-x.db", out.Backup)
	assert.Equal(t, 1, f.replaced)
	f.disk.AssertExpectations(t)
}

func TestRestoreOfACopyThatIsNotThere(t *testing.T) {
	// Arrange
	f := newFixture()
	f.disk.On("File", path, "navdata-manual-20260101-000000.db").Return("", navdb.ErrNoBackup)

	// Act
	_, err := f.manager(true).Restore("navdata-manual-20260101-000000.db")

	// Assert
	assert.ErrorIs(t, err, navdb.ErrNoBackup)
	f.disk.AssertNotCalled(t, "Prepare", mock.Anything, mock.Anything)
}

func TestRestoreRefusesAFileThatCannotBeUsed(t *testing.T) {
	cases := []struct {
		name string
		err  error
		text string
	}{
		{"a damaged file", navdb.ErrCorrupt, "damaged"},
		{"a file of a newer version", navdb.ErrNewerSchema, "newer version"},
		{"a file that is not a navigation database", navdb.ErrNotNavigation, "not a navigation database"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			f := newFixture()
			f.disk.On("File", path, "n").Return("/b/n", nil)
			f.disk.On("Prepare", path, "/b/n").Return("x", nil)
			f.disk.On("InspectIncoming", path).Return(0, c.err)
			f.disk.On("Discard", path).Return()

			// Act
			_, err := f.manager(true).Restore("n")

			// Assert
			var invalid *dbadmin.InvalidFileError
			require.ErrorAs(t, err, &invalid)
			assert.ErrorIs(t, err, c.err)
			assert.Contains(t, err.Error(), c.text)
			f.disk.AssertCalled(t, "Discard", path)
			f.db.AssertNotCalled(t, "Backup", mock.Anything)
			assert.Zero(t, f.replaced, "the database in use is untouched")
		})
	}
}

func TestRestoreSetsAsideADamagedDatabaseBeforeInstalling(t *testing.T) {
	// Arrange
	f := newFixture()
	f.disk.On("File", path, "n").Return("/b/n", nil)
	f.disk.On("Prepare", path, "/b/n").Return("x", nil)
	f.disk.On("InspectIncoming", path).Return(1, nil)
	f.disk.On("SetAside", path).Return("/data/backups/navdata-corrupt-x.db", nil)
	f.disk.On("Install", path).Return(nil)

	// Act
	out, err := f.manager(false).Restore("n")

	// Assert
	require.NoError(t, err)
	assert.Empty(t, out.Backup)
	f.disk.AssertCalled(t, "SetAside", path)
	f.disk.AssertCalled(t, "Install", path)
}

func TestRestoreDiscardsTheFileWhenTheSwapFails(t *testing.T) {
	// Arrange
	f := newFixture()
	f.disk.On("File", path, "n").Return("/b/n", nil)
	f.disk.On("Prepare", path, "/b/n").Return("x", nil)
	f.disk.On("InspectIncoming", path).Return(1, nil)
	f.db.On("Backup", "restore").Return("/b/r", nil)
	f.disk.On("Install", path).Return(errBoom)
	f.disk.On("Discard", path).Return()

	// Act
	_, err := f.manager(true).Restore("n")

	// Assert
	assert.ErrorIs(t, err, errBoom)
	f.disk.AssertCalled(t, "Discard", path)
}

func TestRestoreDoesNotGoOnWithoutTheCopyOfTheCurrentDatabase(t *testing.T) {
	// Arrange
	f := newFixture()
	f.disk.On("File", path, "n").Return("/b/n", nil)
	f.disk.On("Prepare", path, "/b/n").Return("x", nil)
	f.disk.On("InspectIncoming", path).Return(1, nil)
	f.db.On("Backup", "restore").Return("", errBoom)
	f.disk.On("Discard", path).Return()

	// Act
	_, err := f.manager(true).Restore("n")

	// Assert
	assert.ErrorIs(t, err, errBoom)
	assert.Contains(t, err.Error(), "could not copy the current database first")
	assert.Zero(t, f.replaced)
}

func TestImportWritesTheFileAndSwapsItIn(t *testing.T) {
	// Arrange
	f := newFixture()
	content := strings.NewReader("a database file")
	f.disk.On("WriteIncoming", path, content).Return(nil)
	f.disk.On("InspectIncoming", path).Return(1, nil)
	f.db.On("Backup", "restore").Return("/b/r", nil)
	f.disk.On("Install", path).Return(nil)

	// Act
	out, err := f.manager(true).Import(content)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "/b/r", out.Backup)
	f.disk.AssertExpectations(t)
}

func TestImportWhileALoadRunsDoesNotEvenReadTheFile(t *testing.T) {
	// Arrange
	f := newFixture()
	f.busy = true

	// Act
	_, err := f.manager(true).Import(strings.NewReader("x"))

	// Assert
	assert.ErrorIs(t, err, dbadmin.ErrBusy)
	f.disk.AssertNotCalled(t, "WriteIncoming", mock.Anything, mock.Anything)
}

func TestImportReportsAFileThatCouldNotBeWritten(t *testing.T) {
	// Arrange
	f := newFixture()
	f.disk.On("WriteIncoming", path, mock.Anything).Return(errBoom)

	// Act
	_, err := f.manager(true).Import(strings.NewReader("x"))

	// Assert
	assert.ErrorIs(t, err, errBoom)
	f.disk.AssertNotCalled(t, "InspectIncoming", mock.Anything)
}

func TestDeleteBackupGoesToTheDisk(t *testing.T) {
	// Arrange
	f := newFixture()
	f.disk.On("Delete", path, "navdata-manual-20260101-000000.db").Return(nil)

	// Act
	err := f.manager(true).DeleteBackup("navdata-manual-20260101-000000.db")

	// Assert
	assert.NoError(t, err)
	f.disk.AssertExpectations(t)
}
