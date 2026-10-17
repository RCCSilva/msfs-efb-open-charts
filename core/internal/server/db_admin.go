package server

import (
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

// The maintenance of the navigation database file, on the Setup page: its state, copies of it,
// starting over and restoring a copy. Only the computer itself reaches these (see localOnlyPath).

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
type NavProblem struct {
	Kind   string `json:"kind"`
	Detail string `json:"detail"`
}

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

func (s *Server) loadRunning() bool {
	j := &s.nav.job
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.Running
}

// GET /api/db/status
func (s *Server) handleDBStatus(w http.ResponseWriter, r *http.Request) {
	f := s.navFile()
	out := map[string]any{"ok": s.navDB() != nil, "path": f.Path, "problem": s.navProblem(), "latestSchema": navdb.LatestSchema()}
	if fi, err := os.Stat(f.Path); err == nil {
		out["size"] = fi.Size()
	}
	if db := s.navDB(); db != nil {
		if v, err := db.SchemaVersion(); err == nil {
			out["schema"] = v
		}
		out["migrated"] = db.Migrated()
	}
	if f.Path != "" {
		if list, err := navdb.ListBackups(f.Path); err == nil {
			out["backups"] = list
		}
	}
	out["loading"] = s.loadRunning()
	writeJSON(w, 200, out)
}

// guard returns the file control, or answers why the operation cannot be done now.
func (s *Server) dbGuard(w http.ResponseWriter, changes bool) (NavFile, bool) {
	f := s.navFile()
	if f.Path == "" || f.Replace == nil {
		writeErr(w, 503, errors.New("the navigation database is not managed here"))
		return f, false
	}
	if changes && s.loadRunning() {
		writeErr(w, 409, errors.New("a load from the simulator is running: stop it first"))
		return f, false
	}
	return f, true
}

// POST /api/db/backup: a copy the user asked for.
func (s *Server) handleDBBackup(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.dbGuard(w, false); !ok {
		return
	}
	db := s.navDB()
	if db == nil {
		writeErr(w, 409, errors.New("the database is not in use: there is nothing to copy"))
		return
	}
	file, err := db.Backup("manual")
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"file": file})
}

// setAsideOrRemove is what is done to the current file before a new one takes its place: a file
// that could not be used is set aside (the user may want it), a healthy one was copied first.
func setAsideOrRemove(path string, usable bool) error {
	if !usable {
		_, err := navdb.SetAside(path)
		return err
	}
	return navdb.RemoveFiles(path)
}

// POST /api/db/reset: starts over. A healthy database is copied first.
func (s *Server) handleDBReset(w http.ResponseWriter, r *http.Request) {
	f, ok := s.dbGuard(w, true)
	if !ok {
		return
	}
	db := s.navDB()
	backup := ""
	if db != nil {
		b, err := db.Backup("reset")
		if err != nil {
			writeErr(w, 500, errors.New("could not copy the database first: "+err.Error()))
			return
		}
		backup = b
	}
	if err := f.Replace(func() error { return setAsideOrRemove(f.Path, db != nil) }); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "backup": backup})
}

// restoreIncoming puts the file prepared at navdb.IncomingPath in the place of the database, after
// checking it. status is the HTTP status of a refusal.
func (s *Server) restoreIncoming(f NavFile) (status int, backup string, err error) {
	if _, e := navdb.Inspect(navdb.IncomingPath(f.Path)); e != nil {
		navdb.DiscardIncoming(f.Path)
		switch {
		case errors.Is(e, navdb.ErrCorrupt):
			return 422, "", errors.New("this file is damaged: it cannot be used")
		case errors.Is(e, navdb.ErrNewerSchema):
			return 422, "", e
		case errors.Is(e, navdb.ErrNotNavigation):
			return 422, "", e
		}
		return 500, "", e
	}
	db := s.navDB()
	if db != nil {
		b, e := db.Backup("restore")
		if e != nil {
			navdb.DiscardIncoming(f.Path)
			return 500, "", errors.New("could not copy the current database first: " + e.Error())
		}
		backup = b
	}
	if e := f.Replace(func() error {
		if db == nil {
			if _, err := navdb.SetAside(f.Path); err != nil {
				return err
			}
		}
		return navdb.InstallIncoming(f.Path)
	}); e != nil {
		navdb.DiscardIncoming(f.Path)
		return 500, backup, e
	}
	return 0, backup, nil
}

// POST /api/db/restore  {"name": "navdata-manual-....db"}
// A copy of the backups folder takes the place of the database (after being checked).
func (s *Server) handleDBRestore(w http.ResponseWriter, r *http.Request) {
	f, ok := s.dbGuard(w, true)
	if !ok {
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	src, err := navdb.BackupFile(f.Path, body.Name)
	if err != nil {
		writeErr(w, 404, err)
		return
	}
	if _, err := navdb.PrepareIncoming(f.Path, src); err != nil {
		writeErr(w, 500, err)
		return
	}
	status, backup, err := s.restoreIncoming(f)
	if err != nil {
		writeErr(w, status, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "backup": backup, "migrated": s.migrated()})
}

func (s *Server) migrated() any {
	if db := s.navDB(); db != nil {
		return db.Migrated()
	}
	return nil
}

// POST /api/db/import  (multipart, field "file")
// A database file the user brought takes the place of the current one, after being checked.
func (s *Server) handleDBImport(w http.ResponseWriter, r *http.Request) {
	f, ok := s.dbGuard(w, true)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<30)
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || params["boundary"] == "" {
		writeErr(w, 400, errors.New("send the file as multipart/form-data"))
		return
	}
	mr := multipart.NewReader(r.Body, params["boundary"])
	var saved bool
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			navdb.DiscardIncoming(f.Path)
			writeErr(w, 400, err)
			return
		}
		if part.FormName() != "file" {
			continue
		}
		out, err := os.Create(navdb.IncomingPath(f.Path))
		if err != nil {
			writeErr(w, 500, err)
			return
		}
		_, cerr := io.Copy(out, part)
		if e := out.Close(); cerr == nil {
			cerr = e
		}
		if cerr != nil {
			navdb.DiscardIncoming(f.Path)
			writeErr(w, 400, cerr)
			return
		}
		saved = true
		break
	}
	if !saved {
		writeErr(w, 400, errors.New("no file was sent"))
		return
	}
	status, backup, err := s.restoreIncoming(f)
	if err != nil {
		writeErr(w, status, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "backup": backup, "migrated": s.migrated()})
}

// DELETE /api/db/backups/{name}
func (s *Server) handleDBBackupDelete(w http.ResponseWriter, r *http.Request) {
	f, ok := s.dbGuard(w, false)
	if !ok {
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if err := navdb.DeleteBackup(f.Path, name); err != nil {
		if errors.Is(err, navdb.ErrNoBackup) {
			writeErr(w, 404, err)
			return
		}
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
