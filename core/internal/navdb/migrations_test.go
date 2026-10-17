package navdb

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// legacyFile makes a file the way 0.3 and 0.4 left it: tables from the schema constants (the saved
// routes without the later columns), "schema = 3" in meta, and no version in the header.
func legacyFile(t *testing.T, metaSchema string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "navdata.db")
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
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
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%v\n%s", err, q)
		}
	}
	return path
}

func count(t *testing.T, d *DB, table string) int {
	t.Helper()
	var n int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("%s: %v", table, err)
	}
	return n
}

func hasColumn(t *testing.T, d *DB, table, col string) bool {
	t.Helper()
	rows, err := d.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
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

func TestNewFileGetsTheLatestVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "navdata.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if v, _ := d.SchemaVersion(); v != latestVersion() {
		t.Fatalf("version %d", v)
	}
	if m := d.Migrated(); m.From != 0 || m.To != latestVersion() || m.Backup != "" {
		t.Fatalf("%+v: a new file has nothing to copy", m)
	}
	if !hasColumn(t, d, "saved_route", "result") || !hasColumn(t, d, "user_point", "alt_ft") {
		t.Fatal("the columns the baseline adds")
	}
}

// What 0.4 left on the user's disk opens with everything in it.
func TestFileOfTheReleasesKeepsItsData(t *testing.T) {
	path := legacyFile(t, "3")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := d.SchemaVersion(); v != latestVersion() {
		t.Fatalf("version %d", v)
	}
	for table, want := range map[string]int{"airport": 1, "runway": 1, "user_point": 1, "saved_route": 1} {
		if n := count(t, d, table); n != want {
			t.Fatalf("%s: %d rows, want %d", table, n, want)
		}
	}
	for table, col := range map[string]string{"saved_route": "result", "user_point": "tags"} {
		if !hasColumn(t, d, table, col) {
			t.Fatalf("%s.%s was not added", table, col)
		}
	}
	if m := d.Migrated(); m.From != 0 || m.Backup != "" {
		t.Fatalf("%+v: the baseline only adds, there is nothing to protect", m)
	}
	d.Close()

	d, err = Open(path) // opening again changes nothing and copies nothing
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if m := d.Migrated(); m.From != latestVersion() || m.To != latestVersion() || m.Backup != "" {
		t.Fatalf("%+v", m)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "backups")); err == nil {
		t.Fatal("no backups folder is needed")
	}
}

// A later migration: it runs on a file that is at the version before, after a copy of the file.
func TestLaterMigrationRunsAfterACopy(t *testing.T) {
	path := legacyFile(t, "3")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

	base := migrations
	withMigrations(t, append(append([]migration{}, base...), migration{version: 2, name: "pinned routes", up: func(tx *sql.Tx) error {
		if _, err := tx.Exec(`ALTER TABLE saved_route ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0`); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE saved_route SET pinned = 1`)
		return err
	}}))
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	m := d.Migrated()
	if m.From != 1 || m.To != 2 || m.Backup == "" {
		t.Fatalf("%+v", m)
	}
	if v, _ := d.SchemaVersion(); v != 2 {
		t.Fatalf("version %d", v)
	}
	var pinned int
	if err := d.db.QueryRow(`SELECT pinned FROM saved_route`).Scan(&pinned); err != nil || pinned != 1 {
		t.Fatalf("the data was converted: %d %v", pinned, err)
	}
	// the copy is the file as it was before: a database with the route and without the new column
	b, err := sql.Open("sqlite", "file:"+m.Backup)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	var n int
	if err := b.QueryRow(`SELECT COUNT(*) FROM saved_route`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("copy: %d %v", n, err)
	}
	if _, err := b.Exec(`SELECT pinned FROM saved_route`); err == nil {
		t.Fatal("the copy was made before the migration")
	}
}

func TestFailedMigrationLeavesTheFileAsItWas(t *testing.T) {
	path := legacyFile(t, "3")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	d.Close()

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
	if _, err := Open(path); err == nil {
		t.Fatal("the failure must be reported")
	}
	migrations = base
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if v, _ := d.SchemaVersion(); v != 1 {
		t.Fatalf("version %d", v)
	}
	if count(t, d, "saved_route") != 1 {
		t.Fatal("the route is still there")
	}
	if _, err := d.db.Exec(`SELECT * FROM half_done`); err == nil {
		t.Fatal("what the migration did before failing was undone")
	}
}

func TestNewerFileIsNotOpened(t *testing.T) {
	path := legacyFile(t, "3")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, latestVersion()+5)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Open(path); !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("%v", err)
	}
	db, _ = sql.Open("sqlite", "file:"+path)
	defer db.Close()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil || v != latestVersion()+5 {
		t.Fatalf("the file was not touched: %d %v", v, err)
	}
}

// A file of an unreleased development schema starts over for what the simulator gives, after a copy;
// what is the user's stays.
func TestDevelopmentSchemaIsCopiedAndKeepsTheUsersData(t *testing.T) {
	path := legacyFile(t, "2")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.Migrated().Backup == "" {
		t.Fatal("a copy is made before something is emptied")
	}
	if count(t, d, "airport") != 0 {
		t.Fatal("the fetched data starts over")
	}
	if count(t, d, "user_point") != 1 || count(t, d, "saved_route") != 1 {
		t.Fatal("the user's own data stays")
	}
}

func TestOnlyTheNewestCopiesAreKept(t *testing.T) {
	path := legacyFile(t, "3")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	dir := filepath.Join(filepath.Dir(path), "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-10 * time.Hour)
	for i := 0; i < 5; i++ {
		f := filepath.Join(dir, fmt.Sprintf("navdata-schema%d-20200101-00000%d.db", i, i))
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(f, start.Add(time.Duration(i)*time.Hour), start.Add(time.Duration(i)*time.Hour))
	}
	made, err := d.Backup("schema7")
	if err != nil {
		t.Fatal(err)
	}
	left, _ := filepath.Glob(filepath.Join(dir, "navdata-schema*.db"))
	if len(left) != keepBackups {
		t.Fatalf("%d copies left: %v", len(left), left)
	}
	if _, err := os.Stat(made); err != nil {
		t.Fatal("the new copy is kept")
	}
	if _, err := os.Stat(filepath.Join(dir, "navdata-schema0-20200101-000000.db")); err == nil {
		t.Fatal("the oldest is removed")
	}
}
