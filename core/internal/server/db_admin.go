package server

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/dbadmin"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

// The maintenance of the navigation database file, on the Setup page: its state, copies of it,
// starting over and restoring a copy. What is done is in package dbadmin; this is its HTTP face.
// Only the computer itself reaches these (see localOnlyPath).

// NavFile is how the server works on the database file, which only the program's start-up code can
// close and open again.
type NavFile struct {
	Path string
	// Replace closes the database, runs change on the closed file and opens it again (applying the
	// migrations, and setting or clearing the problem). It returns the error of change, else of the
	// opening.
	Replace func(change func() error) error
}

// NavProblem is why the database is not in use: "corrupt" (the file is damaged), "newer" (made by a
// newer version of the program) or "failed" (a migration could not be done).
type NavProblem = dbadmin.Problem

// SetNavFile tells the server where the database file is and how to reopen it.
func (s *Server) SetNavFile(f NavFile) {
	s.nav.ptrMu.Lock()
	s.nav.file = f
	s.nav.ptrMu.Unlock()
}

// SetNavProblem says why the database is not in use; an empty kind says that it is.
func (s *Server) SetNavProblem(kind, detail string) {
	s.nav.ptrMu.Lock()
	defer s.nav.ptrMu.Unlock()
	if kind == "" {
		s.nav.problem = nil
		return
	}
	s.nav.problem = &NavProblem{Kind: kind, Detail: detail}
}

func (s *Server) navFile() NavFile {
	s.nav.ptrMu.RLock()
	defer s.nav.ptrMu.RUnlock()
	return s.nav.file
}

func (s *Server) navProblem() *NavProblem {
	s.nav.ptrMu.RLock()
	defer s.nav.ptrMu.RUnlock()
	return s.nav.problem
}

func (s *Server) loadRunning() bool { return s.nav.job.Running() }

// dbManager does the maintenance of the database as it is now: made for each use because the
// database is replaced while the program runs.
func (s *Server) dbManager() *dbadmin.Manager {
	f := s.navFile()
	var db dbadmin.Database // stays nil when the database cannot be used
	if d := s.navDB(); d != nil {
		db = d
	}
	return dbadmin.New(dbadmin.Deps{Path: f.Path, DB: db, Problem: s.navProblem(), Disk: navdb.Disk{}, Replace: f.Replace, Busy: s.loadRunning})
}

// dbErrStatus is the HTTP status of an error of the maintenance.
func dbErrStatus(err error) int {
	var invalid *dbadmin.InvalidFileError
	switch {
	case errors.Is(err, dbadmin.ErrNotManaged):
		return 503
	case errors.Is(err, dbadmin.ErrBusy), errors.Is(err, dbadmin.ErrNoDatabase):
		return 409
	case errors.Is(err, navdb.ErrNoBackup):
		return 404
	case errors.As(err, &invalid):
		return 422
	}
	return 500
}

// GET /api/db/status
func (s *Server) handleDBStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.dbManager().Status())
}

// POST /api/db/backup: a copy the user asked for.
func (s *Server) handleDBBackup(w http.ResponseWriter, r *http.Request) {
	file, err := s.dbManager().Backup()
	if err != nil {
		writeErr(w, dbErrStatus(err), err)
		return
	}
	writeJSON(w, 200, map[string]any{"file": file})
}

// POST /api/db/reset: starts over. A healthy database is copied first.
func (s *Server) handleDBReset(w http.ResponseWriter, r *http.Request) {
	out, err := s.dbManager().Reset()
	if err != nil {
		writeErr(w, dbErrStatus(err), err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "backup": out.Backup})
}

// migrated says what opening the database did, when it is in use.
func (s *Server) migrated() any {
	if db := s.navDB(); db != nil {
		return db.Migrated()
	}
	return nil
}

// POST /api/db/restore  {"name": "navdata-manual-....db"}
// A copy of the backups folder takes the place of the database (after being checked).
func (s *Server) handleDBRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	out, err := s.dbManager().Restore(body.Name)
	if err != nil {
		writeErr(w, dbErrStatus(err), err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "backup": out.Backup, "migrated": s.migrated()})
}

// POST /api/db/import  (multipart, field "file")
// A database file the user brought takes the place of the current one, after being checked.
func (s *Server) handleDBImport(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<30)
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || params["boundary"] == "" {
		writeErr(w, 400, errors.New("send the file as multipart/form-data"))
		return
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	var file io.Reader
	for file == nil {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeErr(w, 400, err)
			return
		}
		if part.FormName() == "file" {
			file = part
		}
	}
	if file == nil {
		writeErr(w, 400, errors.New("no file was sent"))
		return
	}
	out, err := s.dbManager().Import(file)
	if err != nil {
		writeErr(w, dbErrStatus(err), err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "backup": out.Backup, "migrated": s.migrated()})
}

// DELETE /api/db/backups/{name}
func (s *Server) handleDBBackupDelete(w http.ResponseWriter, r *http.Request) {
	if err := s.dbManager().DeleteBackup(strings.TrimSpace(r.PathValue("name"))); err != nil {
		writeErr(w, dbErrStatus(err), err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
