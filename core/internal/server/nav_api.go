package server

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navlookup"
)

type navState struct {
	mu      sync.Mutex   // one fetch at a time: the simulator answers one conversation, and a repeat is wasted work
	ptrMu   sync.RWMutex // guards only the fields below that are set once (never held while the simulator is asked)
	db      *navdb.DB
	job     navJob // the bulk load of the Navigation page
	file    NavFile
	problem *NavProblem
}

// SetNav gives the server the navigation database (nil while it cannot be used).
func (s *Server) SetNav(db *navdb.DB) {
	s.nav.mu.Lock()
	s.nav.ptrMu.Lock()
	s.nav.db = db
	s.nav.ptrMu.Unlock()
	s.nav.mu.Unlock()
	s.restoreJob()
}

// lookup answers the questions a route raises, from the database and the simulator. It is made for
// each use because the database can be replaced while the program runs.
func (s *Server) lookup() (*navlookup.Lookup, error) {
	db := s.navDB()
	if db == nil {
		return nil, errors.New("the navigation database is not available")
	}
	return navlookup.New(db, s.sim.Nav), nil
}

var icaoRe = regexp.MustCompile(`^[A-Z0-9]{2,8}$`)

// GET /api/nav/airport/{icao}[?refresh=1]
// The stored copy when there is one; otherwise (or with refresh) the airport is asked from the
// simulator, stored, and returned.
func (s *Server) handleNavAirport(w http.ResponseWriter, r *http.Request) {
	icao := strings.ToUpper(strings.TrimSpace(r.PathValue("icao")))
	if !icaoRe.MatchString(icao) {
		writeErr(w, 400, errors.New("not an ICAO identifier"))
		return
	}
	s.nav.mu.Lock()
	defer s.nav.mu.Unlock()
	lk, err := s.lookup()
	if err != nil {
		writeErr(w, 503, err)
		return
	}
	if r.URL.Query().Get("refresh") != "1" {
		if a, err := s.nav.db.Airport(icao); err == nil {
			writeJSON(w, 200, lk.ResolveFixes(a))
			return
		} else if !errors.Is(err, navdb.ErrNotFound) {
			writeErr(w, 500, err)
			return
		}
	}
	if s.sim.Nav == nil {
		writeErr(w, 503, errors.New("not in the database, and there is no link to the simulator to ask it"))
		return
	}
	a, err := s.sim.Nav.Airport(icao)
	if err != nil {
		writeErr(w, 502, err)
		return
	}
	if err := s.nav.db.SaveAirport(a); err != nil {
		writeErr(w, 500, err)
		return
	}
	if stored, err := s.nav.db.Airport(icao); err == nil {
		a = lk.ResolveFixes(stored)
	}
	writeJSON(w, 200, a)
}

// GET /api/nav/fix/{region}/{ident}: a stored fix with the airways through it.
func (s *Server) handleNavFix(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		writeErr(w, 503, errors.New("the navigation database is not available"))
		return
	}
	f, err := db.Fix(r.PathValue("ident"), r.PathValue("region"))
	if errors.Is(err, navdb.ErrNotFound) {
		writeErr(w, 404, err)
		return
	}
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, f)
}

// GET /api/nav/airports[?q=SB&limit=100&offset=0]: a page of the airports already stored, and how many match.
func (s *Server) handleNavAirports(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		writeErr(w, 503, errors.New("the navigation database is not available"))
		return
	}
	q := r.URL.Query()
	limit, offset := 100, 0
	if v, err := strconv.Atoi(q.Get("limit")); err == nil && v > 0 {
		limit = min(v, 500)
	}
	if v, err := strconv.Atoi(q.Get("offset")); err == nil && v > 0 {
		offset = v
	}
	list, total, err := db.AirportsPage(q.Get("q"), limit, offset)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"airports": list, "total": total})
}
