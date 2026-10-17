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

func TestGarbageFileIsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "navdata.db")
	if err := os.WriteFile(path, []byte("this is not a database, it is only text that is long enough to look like a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(path); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("%v", err)
	}
}

func TestDamagedFileIsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "navdata.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 200; i++ { // enough rows to spread over several pages
		if err := d.SaveAirport(sample()); err != nil {
			t.Fatal(err)
		}
		if _, err := d.db.Exec(`INSERT INTO user_point (ident, name, lat, lon) VALUES (?,?,0,0)`, fmt.Sprintf("P%d", i), "a point with a name that takes room in the page"); err != nil {
			t.Fatal(err)
		}
	}
	d.Close()
	fi, err := os.Stat(path)
	if err != nil || fi.Size() < 3*4096 {
		t.Fatalf("%v %d", err, fi.Size())
	}
	// the second page is overwritten with something that is not a page of this database
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	junk := make([]byte, 4096)
	for i := range junk {
		junk[i] = 0xAB
	}
	if _, err := f.WriteAt(junk, 4096); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if _, err := Open(path); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("a page of the file is damaged: %v", err)
	}
}

func TestSetAsideKeepsTheNewestDamagedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "navdata.db")
	if err := os.MkdirAll(filepath.Join(dir, "backups"), 0o755); err != nil {
		t.Fatal(err)
	}
	// two older damaged files are already there
	for i := 0; i < 2; i++ {
		old := filepath.Join(dir, "backups", fmt.Sprintf("navdata-corrupt-2020010%d-000000.db", i+1))
		if err := os.WriteFile(old, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-time.Duration(48-i) * time.Hour)
		_ = os.Chtimes(old, when, when)
	}
	if err := os.WriteFile(path, []byte("damaged"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(path+"-wal", []byte("w"), 0o644)
	to, err := SetAside(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("the damaged file is gone from its place")
	}
	if _, err := os.Stat(path + "-wal"); err == nil {
		t.Fatal("and so is the log beside it")
	}
	list, _ := ListBackups(path)
	if len(list) != 1 || list[0].Name != filepath.Base(to) || list[0].Kind != "corrupt" {
		t.Fatalf("only the newest damaged file is kept: %+v", list)
	}
}

func TestBackupListingAndNames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "navdata.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if list, err := ListBackups(path); err != nil || len(list) != 0 {
		t.Fatalf("no folder yet: %v %v", list, err)
	}
	made, err := d.Backup("manual")
	if err != nil {
		t.Fatal(err)
	}
	list, _ := ListBackups(path)
	if len(list) != 1 || list[0].Kind != "manual" || list[0].Name != filepath.Base(made) || list[0].Size == 0 {
		t.Fatalf("%+v", list)
	}
	// only the program's own names are reachable: no folders, nothing else in the folder
	_ = os.WriteFile(filepath.Join(backupDir(path), "notes.txt"), []byte("x"), 0o644)
	for _, bad := range []string{"../navdata.db", "notes.txt", "navdata-manual-1.db", `..\navdata.db`, ""} {
		if _, err := BackupFile(path, bad); !errors.Is(err, ErrNoBackup) {
			t.Fatalf("%q: %v", bad, err)
		}
	}
	if err := DeleteBackup(path, list[0].Name); err != nil {
		t.Fatal(err)
	}
	if l, _ := ListBackups(path); len(l) != 0 {
		t.Fatalf("%+v", l)
	}
}

// A copy made by Backup can take the place of the database: it is inspected, installed and opened.
func TestRestoreAFileInPlaceOfTheDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "navdata.db")
	d, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := d.SaveAirport(sample()); err != nil {
		t.Fatal(err)
	}
	made, err := d.Backup("manual")
	if err != nil {
		t.Fatal(err)
	}
	// afterwards the database changes: the airport is removed and a point is made
	if err := d.DeleteAirport("SBSJ"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.db.Exec(`INSERT INTO user_point (ident, name, lat, lon) VALUES ('LATER','x',1,1)`); err != nil {
		t.Fatal(err)
	}
	d.Close()

	incoming, err := PrepareIncoming(path, made)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := Inspect(incoming); err != nil || v != latestVersion() {
		t.Fatalf("%d %v", v, err)
	}
	if err := InstallIncoming(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(made); err != nil {
		t.Fatal("the backup the user keeps is not consumed")
	}
	d, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if _, err := d.Airport("SBSJ"); err != nil {
		t.Fatalf("the airport is back: %v", err)
	}
	if count(t, d, "user_point") != 0 {
		t.Fatal("what was done after the copy is gone")
	}
}

func TestInspect(t *testing.T) {
	// a healthy file of an older version
	legacy := legacyFile(t, "3")
	if v, err := Inspect(legacy); err != nil || v != 0 {
		t.Fatalf("an old file is fine, and is migrated when installed: %d %v", v, err)
	}
	// a file that is not a database
	garbage := filepath.Join(t.TempDir(), "x.db")
	_ = os.WriteFile(garbage, []byte("this is not a database, only a text that is long enough to look like a file"), 0o644)
	if _, err := Inspect(garbage); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("%v", err)
	}
	// a healthy SQLite file that is something else
	other := filepath.Join(t.TempDir(), "other.db")
	db, _ := sql.Open("sqlite", "file:"+other)
	if _, err := db.Exec(`CREATE TABLE contacts (name TEXT)`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Inspect(other); !errors.Is(err, ErrNotNavigation) {
		t.Fatalf("%v", err)
	}
	// a file of a newer version
	newer := legacyFile(t, "3")
	db, _ = sql.Open("sqlite", "file:"+newer)
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, latestVersion()+1)); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := Inspect(newer); !errors.Is(err, ErrNewerSchema) {
		t.Fatalf("%v", err)
	}
}
