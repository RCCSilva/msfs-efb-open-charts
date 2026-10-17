package navdb

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sqlite "modernc.org/sqlite"
)

// ErrCorrupt is returned (wrapped) when the file is damaged: it is not a database, or SQLite finds
// that it is malformed. The program then tells the user, and with the user's consent sets the file
// aside (SetAside) and starts a new one.
var ErrCorrupt = errors.New("the navigation database is corrupted")

// SQLite result codes that mean a damaged file (SQLITE_CORRUPT, SQLITE_NOTADB).
const (
	sqliteCorrupt = 11
	sqliteNotADB  = 26
)

// IsCorrupt tells whether the error comes from a damaged file.
func IsCorrupt(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrCorrupt) {
		return true
	}
	var se *sqlite.Error
	if errors.As(err, &se) {
		if c := se.Code() & 0xff; c == sqliteCorrupt || c == sqliteNotADB {
			return true
		}
	}
	s := err.Error()
	return strings.Contains(s, "malformed") || strings.Contains(s, "not a database")
}

// corruptOr marks an error that comes from a damaged file as ErrCorrupt.
func corruptOr(err error) error {
	if err != nil && IsCorrupt(err) && !errors.Is(err, ErrCorrupt) {
		return fmt.Errorf("%w: %v", ErrCorrupt, err)
	}
	return err
}

// checkHealth reads the file's structure before anything else is done to it. A new file passes. It
// takes under a second for a database with every airport of the simulator.
func (d *DB) checkHealth() error {
	var tables int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&tables); err != nil {
		return corruptOr(err)
	}
	if tables == 0 {
		return nil
	}
	var verdict string
	if err := d.db.QueryRow(`PRAGMA quick_check(1)`).Scan(&verdict); err != nil {
		return corruptOr(err)
	}
	if verdict != "ok" {
		return fmt.Errorf("%w: %s", ErrCorrupt, verdict)
	}
	return nil
}

// SetAside moves a damaged database (and the files SQLite keeps beside it) to the "backups" folder,
// named navdata-corrupt-<time>.db, and returns where it went. It keeps only the newest of these.
func SetAside(path string) (string, error) {
	dir := filepath.Join(filepath.Dir(path), "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	to := filepath.Join(dir, "navdata-corrupt-"+time.Now().Format("20060102-150405")+".db")
	if err := os.Rename(path, to); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil // nothing to set aside
		}
		return "", err
	}
	_ = os.Remove(path + "-wal")
	_ = os.Remove(path + "-shm")
	pruneBackups(dir, "corrupt")
	return to, nil
}
