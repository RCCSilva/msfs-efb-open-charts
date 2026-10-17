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

// legacyFile makes a file the way 0.3 and 0.4 left it: tables from the schema constants (the saved
// routes without the later columns), "schema = 3" in meta, and no version in the header.
func legacyFile(t *testing.T, metaSchema string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "navdata.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	require.NoError(t, err)
	defer db.Close()
	for _, q := range []string{
		schema, extraSchema,
		`CREATE TABLE saved_route (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, text TEXT NOT NULL, created_at TEXT, updated_at TEXT)`,
		`INSERT OR REPLACE INTO meta (key, value) VALUES ('schema', '` + metaSchema + `')`,
		`INSERT INTO airport (icao, region, name, lat, lon, alt_m, trans_alt_m, trans_level, fetched_at, source) VALUES ('SBSP','SB','Congonhas',-23.6,-46.6,800,0,0,'2026-10-01T00:00:00Z','x')`,
		`INSERT INTO runway (airport, primary_name, secondary_name) VALUES ('SBSP','17R','35L')`,
		`INSERT INTO user_point (ident, name, lat, lon, note, created_at) VALUES ('MYPT','My point',-23,-46,'n','2026-10-01T00:00:00Z')`,
		`INSERT INTO saved_route (name, text, created_at, updated_at) VALUES ('SBSP > SBGR','SBSP SBGR','2026-10-01T00:00:00Z','2026-10-01T00:00:00Z')`,
	} {
		_, err := db.Exec(q)
		require.NoError(t, err, q)
	}
	return path
}

func count(t *testing.T, d *DB, table string) int {
	t.Helper()
	var n int
	require.NoError(t, d.db.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&n), table)
	return n
}

func hasColumn(t *testing.T, d *DB, table, col string) bool {
	t.Helper()
	rows, err := d.db.Query(`PRAGMA table_info(` + table + `)`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		require.NoError(t, rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk))
		if name == col {
			return true
		}
	}
	return false
}

// withMigrations runs a test with another list of migrations.
func withMigrations(t *testing.T, list []migration) {
	t.Helper()
	old := migrations
	migrations = list
	t.Cleanup(func() { migrations = old })
}

// openLegacy opens (and so migrates) a legacy file, then closes it: it is at the baseline version.
func openLegacy(t *testing.T, metaSchema string) string {
	t.Helper()
	path := legacyFile(t, metaSchema)
	d, err := Open(path)
	require.NoError(t, err)
	d.Close()
	return path
}

func TestNewFileGetsTheLatestVersion(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "navdata.db")

	// Act
	d, err := Open(path)

	// Assert
	require.NoError(t, err)
	defer d.Close()
	v, _ := d.SchemaVersion()
	assert.Equal(t, latestVersion(), v)
	m := d.Migrated()
	assert.Zero(t, m.From)
	assert.Equal(t, latestVersion(), m.To)
	assert.Empty(t, m.Backup, "a new file has nothing to copy")
	assert.True(t, hasColumn(t, d, "saved_route", "result"), "the columns the baseline adds")
	assert.True(t, hasColumn(t, d, "user_point", "alt_ft"), "the columns the baseline adds")
}

// What 0.4 left on the user's disk opens with everything in it.
func TestFileOfTheReleasesKeepsItsData(t *testing.T) {
	// Arrange
	path := legacyFile(t, "3")

	// Act
	d, err := Open(path)

	// Assert
	require.NoError(t, err)
	defer d.Close()
	v, _ := d.SchemaVersion()
	assert.Equal(t, latestVersion(), v)
	for table, want := range map[string]int{"airport": 1, "runway": 1, "user_point": 1, "saved_route": 1} {
		assert.Equal(t, want, count(t, d, table), table)
	}
	for table, col := range map[string]string{"saved_route": "result", "user_point": "tags"} {
		assert.True(t, hasColumn(t, d, table, col), "%s.%s was not added", table, col)
	}
	m := d.Migrated()
	assert.Zero(t, m.From)
	assert.Empty(t, m.Backup, "the baseline only adds, there is nothing to protect")
}

func TestOpeningAMigratedFileAgainChangesNothingAndCopiesNothing(t *testing.T) {
	// Arrange
	path := openLegacy(t, "3")

	// Act
	d, err := Open(path)

	// Assert
	require.NoError(t, err)
	defer d.Close()
	m := d.Migrated()
	assert.Equal(t, latestVersion(), m.From)
	assert.Equal(t, latestVersion(), m.To)
	assert.Empty(t, m.Backup)
	assert.NoDirExists(t, filepath.Join(filepath.Dir(path), "backups"), "no backups folder is needed")
}

// pinnedRoutes is a later migration that adds a column and fills it.
func pinnedRoutes() migration {
	return migration{version: 2, name: "pinned routes", up: func(tx *sql.Tx) error {
		if _, err := tx.Exec(`ALTER TABLE saved_route ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE saved_route SET pinned = 1`)
		return err
	}}
}

// A later migration runs on a file that is at the version before, after a copy of the file.
func TestLaterMigrationConvertsTheData(t *testing.T) {
	// Arrange
	path := openLegacy(t, "3")
	withMigrations(t, append(append([]migration{}, migrations...), pinnedRoutes()))

	// Act
	d, err := Open(path)

	// Assert
	require.NoError(t, err)
	defer d.Close()
	m := d.Migrated()
	assert.Equal(t, 1, m.From)
	assert.Equal(t, 2, m.To)
	v, _ := d.SchemaVersion()
	assert.Equal(t, 2, v)
	var pinned int
	require.NoError(t, d.db.QueryRow(`SELECT pinned FROM saved_route`).Scan(&pinned))
	assert.Equal(t, 1, pinned, "the data was converted")
}

func TestLaterMigrationMakesACopyOfTheFileBeforeIt(t *testing.T) {
	// Arrange
	path := openLegacy(t, "3")
	withMigrations(t, append(append([]migration{}, migrations...), pinnedRoutes()))

	// Act
	d, err := Open(path)

	// Assert
	require.NoError(t, err)
	defer d.Close()
	m := d.Migrated()
	require.NotEmpty(t, m.Backup)
	b, err := sql.Open("sqlite", "file:"+m.Backup)
	require.NoError(t, err)
	defer b.Close()
	var n int
	require.NoError(t, b.QueryRow(`SELECT COUNT(*) FROM saved_route`).Scan(&n))
	assert.Equal(t, 1, n, "the copy has the route")
	_, err = b.Exec(`SELECT pinned FROM saved_route`)
	assert.Error(t, err, "the copy was made before the migration")
}

func TestFailedMigrationLeavesTheFileAsItWas(t *testing.T) {
	// Arrange
	path := openLegacy(t, "3")
	base := migrations
	withMigrations(t, append(append([]migration{}, base...), migration{version: 2, name: "broken", up: func(tx *sql.Tx) error {
		if _, err := tx.Exec(`CREATE TABLE half_done (x INTEGER)`); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM saved_route`); err != nil {
			return err
		}
		return errors.New("something went wrong")
	}}))

	// Act
	_, err := Open(path)

	// Assert
	require.Error(t, err, "the failure must be reported")
	migrations = base
	d, err := Open(path)
	require.NoError(t, err)
	defer d.Close()
	v, _ := d.SchemaVersion()
	assert.Equal(t, 1, v)
	assert.Equal(t, 1, count(t, d, "saved_route"), "the route is still there")
	_, err = d.db.Exec(`SELECT * FROM half_done`)
	assert.Error(t, err, "what the migration did before failing was undone")
}

func TestNewerFileIsNotOpened(t *testing.T) {
	// Arrange
	path := legacyFile(t, "3")
	db, err := sql.Open("sqlite", "file:"+path)
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, latestVersion()+5))
	require.NoError(t, err)
	db.Close()

	// Act
	_, err = Open(path)

	// Assert
	assert.True(t, errors.Is(err, ErrNewerSchema), "%v", err)
	db, _ = sql.Open("sqlite", "file:"+path)
	defer db.Close()
	var v int
	require.NoError(t, db.QueryRow(`PRAGMA user_version`).Scan(&v))
	assert.Equal(t, latestVersion()+5, v, "the file was not touched")
}

// A file of an unreleased development schema starts over for what the simulator gives, after a copy;
// what is the user's stays.
func TestDevelopmentSchemaIsCopiedFirst(t *testing.T) {
	// Arrange
	path := legacyFile(t, "2")

	// Act
	d, err := Open(path)

	// Assert
	require.NoError(t, err)
	defer d.Close()
	assert.NotEmpty(t, d.Migrated().Backup, "a copy is made before something is emptied")
}

func TestDevelopmentSchemaEmptiesWhatTheSimulatorGaveAndKeepsTheUsersData(t *testing.T) {
	// Arrange
	path := legacyFile(t, "2")

	// Act
	d, err := Open(path)

	// Assert
	require.NoError(t, err)
	defer d.Close()
	assert.Zero(t, count(t, d, "airport"), "the fetched data starts over")
	assert.Equal(t, 1, count(t, d, "user_point"), "the user's own data stays")
	assert.Equal(t, 1, count(t, d, "saved_route"), "the user's own data stays")
}

func TestOnlyTheNewestCopiesAreKept(t *testing.T) {
	// Arrange
	path := legacyFile(t, "3")
	d, err := Open(path)
	require.NoError(t, err)
	defer d.Close()
	dir := filepath.Join(filepath.Dir(path), "backups")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	start := time.Now().Add(-10 * time.Hour)
	for i := 0; i < 5; i++ {
		f := filepath.Join(dir, fmt.Sprintf("navdata-schema%d-20200101-00000%d.db", i, i))
		require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))
		_ = os.Chtimes(f, start.Add(time.Duration(i)*time.Hour), start.Add(time.Duration(i)*time.Hour))
	}

	// Act
	made, err := d.Backup("schema7")

	// Assert
	require.NoError(t, err)
	left, _ := filepath.Glob(filepath.Join(dir, "navdata-schema*.db"))
	assert.Len(t, left, keepBackups)
	assert.FileExists(t, made, "the new copy is kept")
	assert.NoFileExists(t, filepath.Join(dir, "navdata-schema0-20200101-000000.db"), "the oldest is removed")
}
