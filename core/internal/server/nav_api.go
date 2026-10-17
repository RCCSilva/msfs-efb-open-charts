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

// AirportAnswer is the answer for one airport of an AirportBatchFetcher.
type AirportAnswer struct {
	Airport *navdata.Airport
	Err     error // navdata.ErrUnknown when the simulator does not know it
}

// AirportBatchFetcher asks the simulator for many airports at once (see simconnect.FetchAirports):
// the answers are in the order of the codes.
type AirportBatchFetcher func(icaos []string) ([]AirportAnswer, error)

// AirportRef is one line of the simulator's list of airports.
type AirportRef struct {
	ICAO   string
	Region string
	Lat    float64
	Lon    float64
}

// AirportLister asks the simulator for the list of all its airports (see simconnect.ListAirports).
type AirportLister func() ([]AirportRef, error)

// SetAirportLister gives the server the way to list every airport of the simulator.
func (s *Server) SetAirportLister(l AirportLister) {
	s.nav.ptrMu.Lock()
	s.nav.list = l
	s.nav.ptrMu.Unlock()
}

// SetAirportBatcher gives the server the way to ask for many airports at once; the bulk load uses it.
func (s *Server) SetAirportBatcher(b AirportBatchFetcher) {
	s.nav.ptrMu.Lock()
	s.nav.batch = b
	s.nav.ptrMu.Unlock()
}

// FixFetcher asks the simulator for waypoints, VORs and NDBs (see simconnect.FetchFixes).
type FixFetcher func(refs []navdata.FixRef) ([]navdata.Fix, error)

type navState struct {
	mu     sync.Mutex   // one fetch at a time: the simulator answers one conversation, and a repeat is wasted work
	ptrMu  sync.RWMutex // guards only the fields below that are set once (never held while the simulator is asked)
	db     *navdb.DB
	fetch  NavFetcher
	fixes  FixFetcher
	lookup IdentFetcher
	job    navJob // the bulk load of the Navigation page
	list   AirportLister
	batch  AirportBatchFetcher
}

// SetNav gives the server the navigation database and, when the simulator link exists, the way
// to fill it. Either may be nil.
func (s *Server) SetNav(db *navdb.DB, fetch NavFetcher, fixes FixFetcher, lookup IdentFetcher) {
	s.nav.mu.Lock()
	s.nav.ptrMu.Lock()
	s.nav.db, s.nav.fetch, s.nav.fixes, s.nav.lookup = db, fetch, fixes, lookup
	s.nav.ptrMu.Unlock()
	s.nav.mu.Unlock()
	s.restoreJob()
}

// resolveFixes asks the simulator for the fixes the procedures of an airport cite and the database
// does not know yet, stores them, and returns the airport again with every leg placed. Without a
// link to the simulator (or when it does not know a fix) the legs simply stay without a position.
func (s *Server) resolveFixes(a *navdata.Airport) *navdata.Airport {
	idents, err := s.nav.db.UncheckedIdents(a.ICAO)
	if err != nil || len(idents) == 0 {
		return a
	}
	if err := s.loadIdents(idents); err != nil {
		return a
	}
	if again, err := s.nav.db.Airport(a.ICAO); err == nil {
		return again
	}
	return a
}

// loadIdents asks the simulator for identifiers as a whole (every place of that name, any region
// and kind), stores what it knows and marks them checked. This is the only way fixes enter the
// database, so that "is in the database" always means "all of them are".
func (s *Server) loadIdents(idents []string) error {
	_, err := s.loadIdentsCount(idents, true)
	return err
}

// loadIdentsCount is loadIdents that also says how many places the simulator knew. With
// markEmpty false, an answer with nothing in it is not recorded (the caller doubts the simulator).
func (s *Server) loadIdentsCount(idents []string, markEmpty bool) (int, error) {
	if s.nav.lookup == nil || len(idents) == 0 {
		return 0, nil
	}
	found, err := s.nav.lookup(idents)
	if err != nil {
		return 0, err // not asked: nothing is marked, the next time asks again
	}
	if len(found) == 0 && !markEmpty {
		return 0, nil
	}
	if err := s.nav.db.SaveFixes(found); err != nil {
		return 0, err
	}
	count := map[string]int{}
	for _, f := range found {
		count[strings.ToUpper(f.Ident)]++
	}
	for _, id := range idents {
		if err := s.nav.db.MarkIdentChecked(id, count[strings.ToUpper(id)]); err != nil {
			return 0, err
		}
	}
	return len(found), nil
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

// GET /api/nav/airports[?q=SB]: the airports already stored.
func (s *Server) handleNavAirports(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
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
