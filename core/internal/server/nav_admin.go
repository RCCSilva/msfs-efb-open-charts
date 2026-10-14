package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/lnmpoints"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navseed"
)

// The "Navigation" page of the manager: what is stored, small edits, the user's own points, and the
// bulk load. The bulk load has two parts that are always in this order:
//
//  1. the identifiers are read from the simulator's own package files, when they are on this
//     computer (no region, no airways: only the names of what exists);
//  2. every identifier is then asked to the simulator, as a whole, through SimConnect: that is what
//     brings the region, the position and the airways. The simulator is always the source of data.

// navJob is the state of the bulk load, as the page shows it.
type navJob struct {
	mu      sync.Mutex
	Running bool      `json:"running"`
	Phase   string    `json:"phase"` // idle, reading, asking, done, stopped, error
	Message string    `json:"message"`
	Total   int       `json:"total"`
	Done    int       `json:"done"`
	Started time.Time `json:"startedAt"`
	Ended   time.Time `json:"endedAt"`
	stop    bool
}

// navJobView is what is sent to the page.
type navJobView struct {
	Running bool      `json:"running"`
	Phase   string    `json:"phase"`
	Message string    `json:"message"`
	Total   int       `json:"total"`
	Done    int       `json:"done"`
	Started time.Time `json:"startedAt"`
	Ended   time.Time `json:"endedAt"`
}

func (j *navJob) snapshot() navJobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	phase := j.Phase
	if phase == "" {
		phase = "idle"
	}
	return navJobView{Running: j.Running, Phase: phase, Message: j.Message, Total: j.Total, Done: j.Done, Started: j.Started, Ended: j.Ended}
}

func (j *navJob) set(f func(j *navJob)) {
	j.mu.Lock()
	f(j)
	j.mu.Unlock()
}

const navJobKey = "navload"

// saveJob keeps the state of the load with the database, so that a restart of the program does not
// make the page forget that one was running.
func (s *Server) saveJob() {
	db := s.nav.db
	if db == nil {
		return
	}
	if b, err := json.Marshal(s.nav.job.snapshot()); err == nil {
		_ = db.SetMeta(navJobKey, string(b))
	}
}

// restoreJob is called when the database is given to the server: a load that was running when the
// program went away is shown as interrupted (it is not resumed by itself).
func (s *Server) restoreJob() {
	db := s.nav.db
	if db == nil {
		return
	}
	var v navJobView
	if err := json.Unmarshal([]byte(db.Meta(navJobKey)), &v); err != nil {
		return
	}
	if v.Running {
		v.Running, v.Phase = false, "interrupted"
		v.Message = ""
	}
	s.nav.job.set(func(j *navJob) {
		j.Running, j.Phase, j.Message, j.Total, j.Done, j.Started, j.Ended = v.Running, v.Phase, v.Message, v.Total, v.Done, v.Started, v.Ended
	})
}

func (s *Server) navDB() *navdb.DB {
	s.nav.mu.Lock()
	defer s.nav.mu.Unlock()
	return s.nav.db
}

func navUnavailable(w http.ResponseWriter) {
	writeErr(w, 503, errors.New("the navigation database is not available"))
}

// GET /api/nav/summary
func (s *Server) handleNavSummary(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	c, err := db.Counts()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	s.nav.mu.Lock()
	canAsk := s.nav.lookup != nil
	s.nav.mu.Unlock()
	files := navseed.Locate()
	if files == nil {
		files = []string{}
	}
	writeJSON(w, 200, map[string]any{"counts": c, "simLink": canAsk, "files": files, "job": s.nav.job.snapshot()})
}

// GET /api/nav/fixes?q=&kind=&limit=&offset=
func (s *Server) handleNavFixes(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	list, total, err := db.ListFixes(q.Get("q"), q.Get("kind"), limit, offset)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"fixes": list, "total": total})
}

// DELETE /api/nav/fixes?ident=&region=&kind=
func (s *Server) handleNavFixDelete(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	q := r.URL.Query()
	err := db.DeleteFix(q.Get("ident"), q.Get("region"), q.Get("kind"))
	if errors.Is(err, navdb.ErrNotFound) {
		writeErr(w, 404, err)
		return
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// DELETE /api/nav/airport/{icao}
func (s *Server) handleNavAirportDelete(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	err := db.DeleteAirport(r.PathValue("icao"))
	if errors.Is(err, navdb.ErrNotFound) {
		writeErr(w, 404, err)
		return
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// GET /api/nav/airways?q=
func (s *Server) handleNavAirways(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	list, err := db.Airways(r.URL.Query().Get("q"), 0)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"airways": list})
}

// GET /api/nav/airway/{name}
func (s *Server) handleNavAirway(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	list, err := db.AirwaySegments(r.PathValue("name"))
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"name": strings.ToUpper(r.PathValue("name")), "segments": list})
}

// ---- the user's points ----

// GET /api/nav/points?q=
func (s *Server) handleNavPoints(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	list, err := db.UserPoints(r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"points": list})
}

// POST /api/nav/points and PUT /api/nav/points/{id}
func (s *Server) handleNavPointSave(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	var p navdb.UserPoint
	if err := readJSON(r, &p); err != nil {
		writeErr(w, 400, err)
		return
	}
	var err error
	if idText := r.PathValue("id"); idText != "" {
		p.ID, err = strconv.ParseInt(idText, 10, 64)
		if err == nil {
			err = db.UpdateUserPoint(p)
		}
	} else {
		p, err = db.CreateUserPoint(p)
	}
	switch {
	case errors.Is(err, navdb.ErrInvalid):
		writeErr(w, 400, err)
	case errors.Is(err, navdb.ErrNotFound):
		writeErr(w, 404, err)
	case err != nil:
		writeErr(w, 500, err)
	default:
		writeJSON(w, 200, p)
	}
}

// POST /api/nav/points/import: the CSV that Little Navmap exports for user points, as a multipart
// field "file" or as the body of the request.
func (s *Server) handleNavPointsImport(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, lnmpoints.MaxBytes+1<<20)
	var src io.Reader = r.Body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		f, _, err := r.FormFile("file")
		if err != nil {
			writeErr(w, 400, errors.New("send the file in the field \"file\""))
			return
		}
		defer f.Close()
		src = f
	}
	points, skipped, err := lnmpoints.Parse(src)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	res, err := db.ImportUserPoints(points)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"added": res.Added, "duplicates": res.Skipped, "ignored": skipped})
}

// DELETE /api/nav/points/{id}
func (s *Server) handleNavPointDelete(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		err = db.DeleteUserPoint(id)
	}
	if errors.Is(err, navdb.ErrNotFound) {
		writeErr(w, 404, err)
		return
	}
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// ---- the bulk load ----

// GET /api/nav/load: how the load is going.
func (s *Server) handleNavLoadStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.nav.job.snapshot())
}

// POST /api/nav/load  {"mode": "files" | "all"}
// "files" only reads the identifiers from the simulator's files; "all" also asks the simulator for
// each of them (it must be running).
func (s *Server) handleNavLoadStart(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	var body struct {
		Mode string `json:"mode"`
	}
	_ = readJSON(r, &body)
	if body.Mode != "files" {
		body.Mode = "all"
	}
	s.nav.mu.Lock()
	canAsk := s.nav.lookup != nil
	s.nav.mu.Unlock()
	if body.Mode == "all" && !canAsk {
		writeErr(w, 503, errors.New("there is no link to the simulator (SimConnect.dll was not found)"))
		return
	}
	j := &s.nav.job
	j.mu.Lock()
	if j.Running {
		j.mu.Unlock()
		writeErr(w, 409, errors.New("a load is already running"))
		return
	}
	j.Running, j.Phase, j.Message, j.Total, j.Done, j.stop = true, "reading", "", 0, 0, false
	j.Started, j.Ended = time.Now(), time.Time{}
	j.mu.Unlock()
	s.saveJob()
	go s.runNavLoad(db, body.Mode)
	writeJSON(w, 202, j.snapshot())
}

// DELETE /api/nav/load: stop it (what was loaded stays).
func (s *Server) handleNavLoadStop(w http.ResponseWriter, r *http.Request) {
	s.nav.job.set(func(j *navJob) { j.stop = true })
	writeJSON(w, 200, s.nav.job.snapshot())
}

func (s *Server) runNavLoad(db *navdb.DB, mode string) {
	j := &s.nav.job
	finish := func(phase, msg string) {
		j.set(func(j *navJob) { j.Running, j.Phase, j.Message, j.Ended = false, phase, msg, time.Now() })
		s.saveJob()
	}
	stopped := func() bool {
		j.mu.Lock()
		defer j.mu.Unlock()
		return j.stop
	}

	// 1. the identifiers, from the simulator's files
	paths := navseed.Locate()
	if len(paths) > 0 {
		list, err := navseed.Read(paths, func(done, total int) bool {
			j.set(func(j *navJob) { j.Total, j.Done = total, done })
			return !stopped()
		})
		if err != nil {
			if stopped() {
				finish("stopped", "")
				return
			}
			finish("error", err.Error())
			return
		}
		if err := db.SaveSeed(list); err != nil {
			finish("error", err.Error())
			return
		}
	}
	if mode == "files" {
		if len(paths) == 0 {
			finish("error", "no navigation package of the simulator was found on this computer")
			return
		}
		finish("done", "")
		return
	}

	// 2. each identifier is asked to the simulator, as a whole
	c, err := db.Counts()
	if err != nil {
		finish("error", err.Error())
		return
	}
	if c.SeedIdents == 0 {
		finish("error", "no identifiers to ask for: the simulator's files were not found")
		return
	}
	j.set(func(j *navJob) { j.Phase, j.Total, j.Done = "asking", c.SeedIdents, c.SeedIdents-c.SeedPending })
	const batch = 40
	answered, empty := false, 0
	for {
		if stopped() {
			finish("stopped", "")
			return
		}
		idents, err := db.SeedPending(batch)
		if err != nil {
			finish("error", err.Error())
			return
		}
		if len(idents) == 0 {
			finish("done", "")
			return
		}
		// the route page and the load take turns: a batch holds the lock, then lets it go
		s.nav.mu.Lock()
		n, err := s.loadIdentsCount(idents, answered)
		s.nav.mu.Unlock()
		if err != nil {
			finish("error", err.Error())
			return
		}
		if n > 0 {
			answered = true
		} else if !answered {
			// nothing came back and nothing ever did in this load: the simulator is probably at its
			// menu, not in a flight. Nothing was marked as "does not exist"; stop instead of doing so.
			empty++
			if empty >= 3 {
				finish("error", "the simulator answered nothing for 120 identifiers in a row: load a flight and try again")
				return
			}
		}
		j.set(func(j *navJob) { j.Done += len(idents) })
		s.saveJob()
	}
}
