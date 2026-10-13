package server

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

// NavFetcher asks the simulator for one airport (see simconnect.FetchAirport).
type NavFetcher func(icao string) (*navdata.Airport, error)

// FixFetcher asks the simulator for waypoints, VORs and NDBs (see simconnect.FetchFixes).
type FixFetcher func(refs []navdata.FixRef) ([]navdata.Fix, error)

type navState struct {
	mu    sync.Mutex // one fetch at a time: the simulator answers one conversation, and a repeat is wasted work
	db    *navdb.DB
	fetch NavFetcher
	fixes FixFetcher
}

// SetNav gives the server the navigation database and, when the simulator link exists, the way
// to fill it. Either may be nil.
func (s *Server) SetNav(db *navdb.DB, fetch NavFetcher, fixes FixFetcher) {
	s.nav.mu.Lock()
	s.nav.db, s.nav.fetch, s.nav.fixes = db, fetch, fixes
	s.nav.mu.Unlock()
}

// resolveFixes asks the simulator for the fixes the procedures of an airport cite and the database
// does not know yet, stores them, and returns the airport again with every leg placed. Without a
// link to the simulator (or when it does not know a fix) the legs simply stay without a position.
func (s *Server) resolveFixes(a *navdata.Airport) *navdata.Airport {
	if s.nav.fixes == nil {
		return a
	}
	missing, err := s.nav.db.MissingFixes(a.ICAO)
	if err != nil || len(missing) == 0 {
		return a
	}
	found, err := s.nav.fixes(missing)
	if len(found) > 0 {
		if serr := s.nav.db.SaveFixes(found); serr != nil {
			return a
		}
	}
	_ = err // a partial answer is kept: what was found is saved, the rest stays missing
	if again, err := s.nav.db.Airport(a.ICAO); err == nil {
		return again
	}
	return a
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
	if s.nav.db == nil {
		writeErr(w, 503, errors.New("the navigation database is not available"))
		return
	}
	if r.URL.Query().Get("refresh") != "1" {
		if a, err := s.nav.db.Airport(icao); err == nil {
			writeJSON(w, 200, s.resolveFixes(a))
			return
		} else if !errors.Is(err, navdb.ErrNotFound) {
			writeErr(w, 500, err)
			return
		}
	}
	if s.nav.fetch == nil {
		writeErr(w, 503, errors.New("not in the database, and there is no link to the simulator to ask it"))
		return
	}
	a, err := s.nav.fetch(icao)
	if err != nil {
		writeErr(w, 502, err)
		return
	}
	if err := s.nav.db.SaveAirport(a); err != nil {
		writeErr(w, 500, err)
		return
	}
	if stored, err := s.nav.db.Airport(icao); err == nil {
		a = s.resolveFixes(stored)
	}
	writeJSON(w, 200, a)
}

// GET /api/nav/fix/{region}/{ident}: a stored fix with the airways through it.
func (s *Server) handleNavFix(w http.ResponseWriter, r *http.Request) {
	s.nav.mu.Lock()
	db := s.nav.db
	s.nav.mu.Unlock()
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

// GET /api/nav/airports[?q=SB]: the airports already stored.
func (s *Server) handleNavAirports(w http.ResponseWriter, r *http.Request) {
	s.nav.mu.Lock()
	db := s.nav.db
	s.nav.mu.Unlock()
	if db == nil {
		writeErr(w, 503, errors.New("the navigation database is not available"))
		return
	}
	list, err := db.Airports(r.URL.Query().Get("q"))
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	if list == nil {
		list = []navdb.Summary{}
	}
	writeJSON(w, 200, map[string]any{"airports": list})
}
