package server

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/lnmpoints"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navload"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navlookup"
)

// The "Navigation" page of the manager: what is stored, small edits, the user's own points, and the
// bulk load. The bulk load has two parts that are always in this order:
//
//  1. the identifiers are read from the simulator's own package files, when they are on this
//     computer (no region, no airways: only the names of what exists);
//  2. every identifier is then asked to the simulator, as a whole, through SimConnect: that is what
//     brings the region, the position and the airways. The simulator is always the source of data.

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
	writeJSON(w, 200, map[string]any{"counts": c, "simLink": canAsk, "files": files, "job": s.nav.job.Snapshot()})
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
	q := r.URL.Query()
	limit, offset := 25, 0
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 {
		limit = min(v, 500)
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil && v > 0 {
		offset = v
	}
	list, total, err := db.AirwaysPage(q.Get("q"), limit, offset)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"airways": list, "total": total})
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
	// without limit: every point (the map planner draws them all); with it, a page and the total
	q := r.URL.Query()
	limit, offset := 0, 0
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 {
		limit = min(v, 500)
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil && v > 0 {
		offset = v
	}
	list, total, err := db.UserPointsPage(q.Get("q"), limit, offset)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"points": list, "total": total})
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
	writeJSON(w, 200, s.nav.job.Snapshot())
}

// POST /api/nav/load  {"mode": "files" | "fixes" | "airports" | "all", "prefix": "SB"}
// "files" only reads the identifiers from the simulator's files; the others also ask the simulator
// (it must be running). The steps are in package navload.
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
	mode := navload.ParseMode(body.Mode)
	if mode != navload.Files && !s.canAskSim() {
		writeErr(w, 503, errors.New("there is no link to the simulator (SimConnect.dll was not found)"))
		return
	}
	if err := s.nav.job.Begin(); err != nil {
		writeErr(w, 409, err)
		return
	}
	s.nav.job.Save(db)
	timings := navload.NewTimings() // where the time of this load goes, written to the log when it ends
	loader := navload.New(&s.nav.job, navload.Deps{
		Store: db, Airports: s.sim.Nav, Seed: s.sim.Seed, Idents: navlookup.New(db, s.sim.Nav).WithTimer(timings.Add), Turn: &s.nav.mu,
		Timings: timings, Logf: log.Printf,
	})
	go loader.Run(mode, body.Prefix)
	writeJSON(w, 202, s.nav.job.Snapshot())
}

// DELETE /api/nav/load: stop it (what was loaded stays).
func (s *Server) handleNavLoadStop(w http.ResponseWriter, r *http.Request) {
	s.nav.job.Stop()
	writeJSON(w, 200, s.nav.job.Snapshot())
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
	a, err := db.InAreaView(minLat, minLon, maxLat, maxLon, 1500, 3000, q.Get("fixes") == "1", q.Get("segments") == "1")
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
