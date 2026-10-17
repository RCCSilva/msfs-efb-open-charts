package navdb

import (
	"io"
	"os"
)

// Disk works on the files of the database and of its copies. It keeps no state: the path of the
// database is given to each method. It is what the maintenance of the database takes as an
// interface (see navdbmock.Disk for a mock).
type Disk struct{}

// List lists the copies, the newest first.
func (Disk) List(dbPath string) ([]BackupInfo, error) { return ListBackups(dbPath) }

// File is the path of a copy, checked.
func (Disk) File(dbPath, name string) (string, error) { return BackupFile(dbPath, name) }

// Delete removes a copy.
func (Disk) Delete(dbPath, name string) error { return DeleteBackup(dbPath, name) }

// Prepare copies a file next to the database, ready to take its place.
func (Disk) Prepare(dbPath, src string) (string, error) { return PrepareIncoming(dbPath, src) }

// WriteIncoming writes what r gives next to the database, ready to take its place.
func (Disk) WriteIncoming(dbPath string, r io.Reader) error {
	out, err := os.Create(IncomingPath(dbPath))
	if err != nil {
		return err
	}
	_, cerr := io.Copy(out, r)
	if e := out.Close(); cerr == nil {
		cerr = e
	}
	if cerr != nil {
		DiscardIncoming(dbPath)
	}
	return cerr
}

// Discard removes a prepared file that will not be used.
func (Disk) Discard(dbPath string) { DiscardIncoming(dbPath) }

// InspectIncoming looks at the prepared file: its schema version, or why it cannot be used.
func (Disk) InspectIncoming(dbPath string) (int, error) { return Inspect(IncomingPath(dbPath)) }

// Install puts the prepared file where the database is. The database must be closed.
func (Disk) Install(dbPath string) error { return InstallIncoming(dbPath) }

// Remove removes the database and the files beside it. It must be closed.
func (Disk) Remove(dbPath string) error { return RemoveFiles(dbPath) }

// SetAside moves a damaged database to the backups folder and says where it went.
func (Disk) SetAside(dbPath string) (string, error) { return SetAside(dbPath) }

// Size is the size of a file, and whether it exists.
func (Disk) Size(path string) (int64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	return fi.Size(), true
}
