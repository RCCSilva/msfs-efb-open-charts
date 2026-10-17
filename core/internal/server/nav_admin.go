package server

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/lnmpoints"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navlookup"
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
		// a database with no load in its history (a new one): nothing is shown of the last one
		s.nav.job.set(func(j *navJob) {
			j.Running, j.Phase, j.Message, j.Total, j.Done, j.Started, j.Ended, j.stop = false, "", "", 0, 0, time.Time{}, time.Time{}, false
		})
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

// navDB returns the database without waiting for a fetch in progress (the bulk load holds the
// fetch lock for the whole time a batch is asked to the simulator).
func (s *Server) navDB() *navdb.DB {
	s.nav.ptrMu.RLock()
	defer s.nav.ptrMu.RUnlock()
	return s.nav.db
}

func (s *Server) canAskSim() bool { return s.sim.Nav != nil }

// seedFiles are the files of the simulator the identifiers can be read from (none when it has no
// such files).
func (s *Server) seedFiles() []string {
	if s.sim.Seed == nil {
		return []string{}
	}
	if files := s.sim.Seed.Locate(); files != nil {
		return files
	}
	return []string{}
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
	canAsk := s.canAskSim()
	files := s.seedFiles()
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
		Mode   string `json:"mode"`   // files, fixes, airports or all (fixes, then airports)
		Prefix string `json:"prefix"` // only airports whose code starts with this ("SB"); empty: every airport
	}
	_ = readJSON(r, &body)
	switch body.Mode {
	case "files", "fixes", "airports":
	default:
		body.Mode = "all"
	}
	body.Prefix = strings.ToUpper(strings.TrimSpace(body.Prefix))
	canAsk := s.canAskSim()
	if body.Mode != "files" && !canAsk {
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
	go s.runNavLoad(db, body.Mode, body.Prefix)
	writeJSON(w, 202, j.snapshot())
}

// DELETE /api/nav/load: stop it (what was loaded stays).
func (s *Server) handleNavLoadStop(w http.ResponseWriter, r *http.Request) {
	s.nav.job.set(func(j *navJob) { j.stop = true })
	writeJSON(w, 200, s.nav.job.snapshot())
}

func (s *Server) runNavLoad(db *navdb.DB, mode, prefix string) {
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

	if mode != "airports" {
		if !s.loadFixes(db, mode, finish, stopped) {
			return
		}
	}
	if mode == "all" || mode == "airports" {
		if !s.loadAirports(db, prefix, finish, stopped) {
			return
		}
	}
	finish("done", "")
}

// loadFixes is steps 1 and 2: the identifiers from the simulator's files, then each of them asked
// to the simulator as a whole. It returns false when the load ended (finish was called).
func (s *Server) loadFixes(db *navdb.DB, mode string, finish func(phase, msg string), stopped func() bool) bool {
	j := &s.nav.job

	// 1. the identifiers, from the simulator's files
	paths := s.seedFiles()
	if len(paths) > 0 {
		list, err := s.sim.Seed.Read(paths, func(done, total int) bool {
			j.set(func(j *navJob) { j.Total, j.Done = total, done })
			return !stopped()
		})
		if err != nil {
			if stopped() {
				finish("stopped", "")
				return false
			}
			finish("error", err.Error())
			return false
		}
		if err := db.SaveSeed(list); err != nil {
			finish("error", err.Error())
			return false
		}
	}
	if mode == "files" {
		if len(paths) == 0 {
			finish("error", "no navigation package of the simulator was found on this computer")
			return false
		}
		finish("done", "")
		return false
	}

	// 2. each identifier is asked to the simulator, as a whole
	c, err := db.Counts()
	if err != nil {
		finish("error", err.Error())
		return false
	}
	if c.SeedIdents == 0 {
		finish("error", "no identifiers to ask for: the simulator's files were not found")
		return false
	}
	j.set(func(j *navJob) { j.Phase, j.Total, j.Done = "asking", c.SeedIdents, c.SeedIdents-c.SeedPending })
	const batch = 40
	answered, empty := false, 0
	for {
		if stopped() {
			finish("stopped", "")
			return false
		}
		idents, err := db.SeedPending(batch)
		if err != nil {
			finish("error", err.Error())
			return false
		}
		if len(idents) == 0 {
			// the names of the files are done: the fixes that airways cite and the files did not
			// have are asked too, until no airway leads to a place the database does not have
			idents, err = db.MissingNeighbors(batch)
			if err != nil {
				finish("error", err.Error())
				return false
			}
			if len(idents) == 0 {
				return true
			}
			j.set(func(j *navJob) { j.Total += len(idents) })
		}
		// the route page and the load take turns: a batch holds the lock, then lets it go
		s.nav.mu.Lock()
		n, err := navlookup.New(db, s.sim.Nav).LoadIdentsCount(idents, answered)
		s.nav.mu.Unlock()
		if err != nil {
			finish("error", err.Error())
			return false
		}
		if n > 0 {
			answered = true
		} else if !answered {
			// nothing came back and nothing ever did in this load: the simulator is probably at its
			// menu, not in a flight. Nothing was marked as "does not exist"; stop instead of doing so.
			empty++
			if empty >= 3 {
				finish("error", "the simulator answered nothing for 120 identifiers in a row: load a flight and try again")
				return false
			}
		}
		j.set(func(j *navJob) { j.Done += len(idents) })
		s.saveJob()
	}
}

// loadAirports is step 3: the simulator lists its airports, and each one that is not stored yet
// (and starts with the prefix, when there is one) is asked in full, with its procedures.
func (s *Server) loadAirports(db *navdb.DB, prefix string, finish func(phase, msg string), stopped func() bool) bool {
	j := &s.nav.job
	if s.sim.Nav == nil {
		finish("error", "there is no link to the simulator to list its airports")
		return false
	}
	j.set(func(j *navJob) { j.Phase, j.Message, j.Total, j.Done = "listing", "", 0, 0 })
	s.saveJob()
	all, err := s.sim.Nav.ListAirports()
	if err != nil {
		finish("error", err.Error())
		return false
	}
	have := map[string]bool{}
	if stored, err := db.Airports(""); err == nil {
		for _, a := range stored {
			have[a.ICAO] = true
		}
	}
	var todo []string
	for _, a := range all {
		icao := strings.ToUpper(a.ICAO)
		if !icaoRe.MatchString(icao) || have[icao] || (prefix != "" && !strings.HasPrefix(icao, prefix)) {
			continue
		}
		have[icao] = true // a repeat in the list is asked once
		todo = append(todo, icao)
	}
	sort.Strings(todo)
	j.set(func(j *navJob) { j.Phase, j.Total, j.Done = "airports", len(todo), 0 })
	s.saveJob()
	// the simulator answers each request after a fixed delay, so many are asked at once
	const chunk = 200
	failures := 0
	for from := 0; from < len(todo); from += chunk {
		if stopped() {
			finish("stopped", "")
			return false
		}
		codes := todo[from:min(len(todo), from+chunk)]
		s.nav.mu.Lock()
		answers, err := s.sim.Nav.Airports(codes)
		if err == nil && len(answers) != len(codes) {
			err = errors.New("the simulator answered a different number of airports than asked")
		}
		if err == nil {
			for _, ans := range answers {
				if ans.Err == nil {
					ans.Err = db.SaveAirport(ans.Airport)
				}
				switch {
				case ans.Err == nil:
					failures = 0
				case errors.Is(ans.Err, navdata.ErrUnknown):
					// the simulator listed it and then did not know it: skip
				default:
					failures++
					err = ans.Err
				}
			}
		} else {
			failures++
		}
		s.nav.mu.Unlock()
		if failures >= 5 {
			finish("error", err.Error())
			return false
		}
		j.set(func(j *navJob) { j.Done = from + len(codes) })
		s.saveJob()
	}
	return true
}

// GET /api/nav/area?minLat=&minLon=&maxLat=&maxLon=&fixes=1&segments=1
// What is stored inside a rectangle of the map, for the planner.
func (s *Server) handleNavArea(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	q := r.URL.Query()
	num := func(k string) (float64, bool) {
		v, err := strconv.ParseFloat(q.Get(k), 64)
		return v, err == nil
	}
	minLat, ok1 := num("minLat")
	minLon, ok2 := num("minLon")
	maxLat, ok3 := num("maxLat")
	maxLon, ok4 := num("maxLon")
	if !(ok1 && ok2 && ok3 && ok4) || minLat > maxLat || minLon > maxLon {
		writeErr(w, 400, errors.New("minLat, minLon, maxLat and maxLon are required"))
		return
	}
	a, err := db.InArea(minLat, minLon, maxLat, maxLon, 1500, 3000, q.Get("fixes") == "1", q.Get("segments") == "1")
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, a)
}

// GET /api/nav/search?q=BOB&limit=30
// Places by identifier (starts with) or name (contains): airports, fixes and the user's points.
func (s *Server) handleNavSearch(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, err := db.Search(r.URL.Query().Get("q"), limit)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"results": list})
}
