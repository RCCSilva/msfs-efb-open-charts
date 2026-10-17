package navdb

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Backups of the database file live in the "backups" folder next to it, named
// navdata-<kind>-<date>-<time>.db. The kind says why the copy was made:
//
//	schemaN   before a migration from schema N (the newest 3 are kept)
//	manual    the user asked for it (kept until the user removes it)
//	reset     before the user started over (the newest 3 are kept)
//	restore   before the user restored another copy (the newest 3 are kept)
//	corrupt   the file that was damaged, set aside (only the newest is kept)
//	imported  a file the user brought (kept until the user removes it)

// validBackupName is a file the program may list, restore or remove: no folders, only its own names.
var validBackupName = regexp.MustCompile(`^navdata-[a-z]+[0-9]*-[0-9]{8}-[0-9]{6}\.db$`)

// BackupInfo is one copy in the backups folder.
type BackupInfo struct {
	Name string    `json:"name"`
	Kind string    `json:"kind"`
	Size int64     `json:"size"`
	Time time.Time `json:"time"`
}

func backupDir(dbPath string) string { return filepath.Join(filepath.Dir(dbPath), "backups") }

func backupKind(name string) string {
	parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(name, "navdata-"), ".db"), "-")
	if len(parts) < 3 {
		return ""
	}
	return strings.Join(parts[:len(parts)-2], "-")
}

// keepOf is how many copies of a kind are kept (0 = all).
func keepOf(kind string) int {
	switch strings.TrimRight(kind, "0123456789") {
	case "manual", "imported":
		return 0
	case "corrupt":
		return 1
	}
	return keepBackups
}

func backupFileName(kind string) string {
	return "navdata-" + kind + "-" + time.Now().Format("20060102-150405") + ".db"
}

// Backup copies the open database into the backups folder (VACUUM INTO is consistent, the
// write-ahead log included) and removes the oldest copies of that kind.
func (d *DB) Backup(kind string) (string, error) {
	dir := backupDir(d.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	file := filepath.Join(dir, backupFileName(kind))
	if _, err := d.db.Exec(`VACUUM INTO ?`, file); err != nil {
		return "", err
	}
	pruneBackups(dir, kind)
	return file, nil
}

// pruneBackups removes the oldest copies of the same group as kind.
func pruneBackups(dir, kind string) {
	keep := keepOf(kind)
	if keep == 0 {
		return
	}
	group := strings.TrimRight(kind, "0123456789")
	old, _ := filepath.Glob(filepath.Join(dir, "navdata-"+group+"*.db"))
	var mine []string
	for _, f := range old {
		if validBackupName.MatchString(filepath.Base(f)) && strings.TrimRight(backupKind(filepath.Base(f)), "0123456789") == group {
			mine = append(mine, f)
		}
	}
	sort.Slice(mine, func(i, j int) bool { return modTime(mine[i]).After(modTime(mine[j])) })
	for i := keep; i < len(mine); i++ {
		_ = os.Remove(mine[i])
	}
}

// ListBackups lists the copies, the newest first.
func ListBackups(dbPath string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(backupDir(dbPath))
	if errors.Is(err, os.ErrNotExist) {
		return []BackupInfo{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []BackupInfo{}
	for _, e := range entries {
		if e.IsDir() || !validBackupName.MatchString(e.Name()) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, BackupInfo{Name: e.Name(), Kind: backupKind(e.Name()), Size: fi.Size(), Time: fi.ModTime()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Time.After(out[j].Time) })
	return out, nil
}

// ErrNoBackup is returned for a name that is not a copy in the backups folder.
var ErrNoBackup = errors.New("there is no such backup")

// BackupFile is the path of a copy, checked: only the program's own names, and it must exist.
func BackupFile(dbPath, name string) (string, error) {
	if !validBackupName.MatchString(name) {
		return "", ErrNoBackup
	}
	p := filepath.Join(backupDir(dbPath), name)
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return "", ErrNoBackup
	}
	return p, nil
}

// DeleteBackup removes a copy.
func DeleteBackup(dbPath, name string) error {
	p, err := BackupFile(dbPath, name)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// ErrNotNavigation is returned for a healthy SQLite file that is not a navigation database.
var ErrNotNavigation = errors.New("this file is not a navigation database of Open Charts")

// Inspect looks at a database file without changing it and says its schema version. It returns
// ErrCorrupt for a damaged file, ErrNotNavigation for another kind of database and ErrNewerSchema
// for one made by a newer version. The file is read as it is: no migration runs on it.
func Inspect(path string) (int, error) {
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	d := &DB{db: db, path: path}
	if err := d.checkHealth(); err != nil {
		return 0, err
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('airport', 'meta')`).Scan(&n); err != nil {
		return 0, corruptOr(err)
	}
	if n != 2 {
		return 0, ErrNotNavigation
	}
	v, err := d.SchemaVersion()
	if err != nil {
		return 0, corruptOr(err)
	}
	if v > latestVersion() {
		return v, fmt.Errorf("%w (file: schema %d, this program: %d)", ErrNewerSchema, v, latestVersion())
	}
	return v, nil
}

func incomingPath(dbPath string) string { return dbPath + ".incoming" }

// IncomingPath is where a file that will take the database's place is written (an upload goes
// straight there); see InstallIncoming.
func IncomingPath(dbPath string) string { return incomingPath(dbPath) }

// PrepareIncoming copies a file next to the database, ready to take its place. The copy is what is
// inspected and installed: the original (a backup the user keeps) is never touched.
func PrepareIncoming(dbPath, src string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	to := incomingPath(dbPath)
	out, err := os.Create(to)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(to)
		return "", err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(to)
		return "", err
	}
	return to, nil
}

// DiscardIncoming removes a prepared copy that will not be used.
func DiscardIncoming(dbPath string) { _ = os.Remove(incomingPath(dbPath)) }

// InstallIncoming puts the prepared copy where the database is. The database must be closed.
func InstallIncoming(dbPath string) error {
	if err := RemoveFiles(dbPath); err != nil {
		return err
	}
	return os.Rename(incomingPath(dbPath), dbPath)
}

// RemoveFiles removes the database and the files SQLite keeps beside it. It must be closed.
func RemoveFiles(dbPath string) error {
	for _, p := range []string{dbPath + "-wal", dbPath + "-shm", dbPath} {
		if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
