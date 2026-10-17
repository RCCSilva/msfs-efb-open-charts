// Package dbadmin looks after the navigation database file: its state, copies of it, starting over
// and restoring a copy (or a file the user brings). The file is only ever replaced after it was
// checked and a copy of the one in use was made, so that what the user had can be had back.
package dbadmin

import (
	"errors"
	"fmt"
	"io"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

// Database is the open database as the maintenance uses it.
type Database interface {
	Backup(kind string) (string, error)
	SchemaVersion() (int, error)
	Migrated() navdb.Migration
}

// Disk works on the files of the database and of its copies (navdb.Disk).
type Disk interface {
	List(dbPath string) ([]navdb.BackupInfo, error)
	File(dbPath, name string) (string, error)
	Delete(dbPath, name string) error
	Prepare(dbPath, src string) (string, error)
	WriteIncoming(dbPath string, r io.Reader) error
	Discard(dbPath string)
	InspectIncoming(dbPath string) (int, error)
	Install(dbPath string) error
	Remove(dbPath string) error
	SetAside(dbPath string) (string, error)
	Size(path string) (int64, bool)
}

// Problem is why the database is not in use: "corrupt" (the file is damaged), "newer" (made by a
// newer version of the program) or "failed" (a migration could not be done).
type Problem struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

// Deps is what a Manager needs.
type Deps struct {
	Path string // the database file
	// DB is the database in use; nil when it cannot be used (its file is damaged, for one).
	DB      Database
	Problem *Problem
	Disk    Disk
	// Replace closes the database, runs change on the closed file and opens it again (applying the
	// migrations). It returns the error of change, else of the opening.
	Replace func(change func() error) error
	// Busy tells whether something is using the database for long (a load from the simulator).
	Busy func() bool
}

var (
	// ErrNotManaged is returned when the program does not manage the file.
	ErrNotManaged = errors.New("the navigation database is not managed here")
	// ErrBusy is returned when a load from the simulator is running.
	ErrBusy = errors.New("a load from the simulator is running: stop it first")
	// ErrNoDatabase is returned when there is nothing to copy because the database is not in use.
	ErrNoDatabase = errors.New("the database is not in use: there is nothing to copy")
)

// InvalidFileError is returned for a file that cannot take the place of the database.
type InvalidFileError struct{ Err error }

func (e *InvalidFileError) Error() string {
	if errors.Is(e.Err, navdb.ErrCorrupt) {
		return "this file is damaged: it cannot be used"
	}
	return e.Err.Error()
}
func (e *InvalidFileError) Unwrap() error { return e.Err }

// Status is the state of the database, as the Setup page shows it.
type Status struct {
	OK           bool               `json:"ok"`
	Path         string             `json:"path"`
	Size         int64              `json:"size,omitempty"`
	Schema       *int               `json:"schema,omitempty"`
	LatestSchema int                `json:"latestSchema"`
	Problem      *Problem           `json:"problem"`
	Migrated     *navdb.Migration   `json:"migrated,omitempty"`
	Backups      []navdb.BackupInfo `json:"backups"`
	Loading      bool               `json:"loading"`
}

// Outcome is what a change of the database file did.
type Outcome struct {
	Backup string `json:"backup"` // the copy of what was in use, "" when there was none
}

// Manager does the maintenance.
type Manager struct{ d Deps }

// New builds a Manager.
func New(d Deps) *Manager { return &Manager{d: d} }

func (m *Manager) busy() bool { return m.d.Busy != nil && m.d.Busy() }

func (m *Manager) managed() error {
	if m.d.Path == "" || m.d.Replace == nil {
		return ErrNotManaged
	}
	return nil
}

// guard says why a change cannot be made now.
func (m *Manager) guard() error {
	if err := m.managed(); err != nil {
		return err
	}
	if m.busy() {
		return ErrBusy
	}
	return nil
}

// Status is the state of the database.
func (m *Manager) Status() Status {
	st := Status{OK: m.d.DB != nil, Path: m.d.Path, Problem: m.d.Problem, LatestSchema: navdb.LatestSchema(), Loading: m.busy()}
	if size, ok := m.d.Disk.Size(m.d.Path); ok {
		st.Size = size
	}
	if m.d.DB != nil {
		if v, err := m.d.DB.SchemaVersion(); err == nil {
			st.Schema = &v
		}
		mig := m.d.DB.Migrated()
		st.Migrated = &mig
	}
	if m.d.Path != "" {
		st.Backups = []navdb.BackupInfo{}
		if list, err := m.d.Disk.List(m.d.Path); err == nil && list != nil {
			st.Backups = list
		}
	}
	return st
}

// Backup makes a copy the user asked for and says where it is.
func (m *Manager) Backup() (string, error) {
	if err := m.managed(); err != nil {
		return "", err
	}
	if m.d.DB == nil {
		return "", ErrNoDatabase
	}
	return m.d.DB.Backup("manual")
}

// setAsideOrRemove is what is done to the file in use before a new one takes its place: a file that
// could not be used is set aside (the user may want it), a healthy one was copied first.
func (m *Manager) setAsideOrRemove() error {
	if m.d.DB == nil {
		_, err := m.d.Disk.SetAside(m.d.Path)
		return err
	}
	return m.d.Disk.Remove(m.d.Path)
}

// Reset starts over: the database in use is copied first, then a new, empty one takes its place.
func (m *Manager) Reset() (Outcome, error) {
	if err := m.guard(); err != nil {
		return Outcome{}, err
	}
	var out Outcome
	if m.d.DB != nil {
		b, err := m.d.DB.Backup("reset")
		if err != nil {
			return out, fmt.Errorf("could not copy the database first: %w", err)
		}
		out.Backup = b
	}
	if err := m.d.Replace(m.setAsideOrRemove); err != nil {
		return out, err
	}
	return out, nil
}

// Restore puts a copy of the backups folder in the place of the database, after checking it.
func (m *Manager) Restore(name string) (Outcome, error) {
	if err := m.guard(); err != nil {
		return Outcome{}, err
	}
	src, err := m.d.Disk.File(m.d.Path, name)
	if err != nil {
		return Outcome{}, err
	}
	// the copy is what is inspected and installed: the original is never touched
	if _, err := m.d.Disk.Prepare(m.d.Path, src); err != nil {
		return Outcome{}, err
	}
	return m.installIncoming()
}

// Import puts a database file the user brought in the place of the database, after checking it.
func (m *Manager) Import(file io.Reader) (Outcome, error) {
	if err := m.guard(); err != nil {
		return Outcome{}, err
	}
	if err := m.d.Disk.WriteIncoming(m.d.Path, file); err != nil {
		return Outcome{}, err
	}
	return m.installIncoming()
}

// installIncoming checks the prepared file, copies what is in use and swaps the file in.
func (m *Manager) installIncoming() (Outcome, error) {
	if _, err := m.d.Disk.InspectIncoming(m.d.Path); err != nil {
		m.d.Disk.Discard(m.d.Path)
		if errors.Is(err, navdb.ErrCorrupt) || errors.Is(err, navdb.ErrNewerSchema) || errors.Is(err, navdb.ErrNotNavigation) {
			return Outcome{}, &InvalidFileError{Err: err}
		}
		return Outcome{}, err
	}
	var out Outcome
	if m.d.DB != nil {
		b, err := m.d.DB.Backup("restore")
		if err != nil {
			m.d.Disk.Discard(m.d.Path)
			return out, fmt.Errorf("could not copy the current database first: %w", err)
		}
		out.Backup = b
	}
	err := m.d.Replace(func() error {
		if m.d.DB == nil {
			if _, err := m.d.Disk.SetAside(m.d.Path); err != nil {
				return err
			}
		}
		return m.d.Disk.Install(m.d.Path)
	})
	if err != nil {
		m.d.Disk.Discard(m.d.Path)
		return out, err
	}
	return out, nil
}

// DeleteBackup removes a copy.
func (m *Manager) DeleteBackup(name string) error {
	if err := m.managed(); err != nil {
		return err
	}
	return m.d.Disk.Delete(m.d.Path, name)
}
