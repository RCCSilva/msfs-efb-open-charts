package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/route"
)

// IdentFetcher asks the simulator what an identifier means, as a whole: every waypoint, VOR and
// NDB of that name, in every region (see simconnect.LookupFixes).
type IdentFetcher func(idents []string) ([]navdata.Fix, error)

// routeProvider looks names up for the route package: the database first; the simulator for what
// the database has never been asked about, and only once for each name.
type routeProvider struct{ s *Server }

func (p routeProvider) Airport(icao string) (*navdata.Airport, error) {
	s := p.s
	if s.nav.db == nil {
		return nil, errors.New("the navigation database is not available")
	}
	a, err := s.nav.db.Airport(icao)
	if err == nil {
		return s.resolveFixes(a), nil
	}
	if !errors.Is(err, navdb.ErrNotFound) {
		return nil, err
	}
	if s.nav.fetch == nil {
		return nil, nil // without a link to the simulator an unknown airport is just unknown
	}
	got, err := s.nav.fetch(icao)
	if errors.Is(err, navdata.ErrUnknown) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.nav.db.SaveAirport(got); err != nil {
		return nil, err
	}
	stored, err := s.nav.db.Airport(icao)
	if err != nil {
		return nil, err
	}
	return s.resolveFixes(stored), nil
}

func (p routeProvider) Fixes(ident string) ([]navdata.Fix, error) {
	s := p.s
	if s.nav.db == nil {
		return nil, errors.New("the navigation database is not available")
	}
	checked, err := s.nav.db.IdentChecked(ident)
	if err != nil {
		return nil, err
	}
	if !checked {
		// the simulator may not be reachable: then what is stored is the answer, and the name stays
		// unchecked so that the next time asks again
		_ = s.loadIdents([]string{ident})
	}
	return s.nav.db.FixesByIdent(ident)
}

func (p routeProvider) FixAt(ref navdata.FixRef) (*navdata.Fix, error) {
	s := p.s
	if s.nav.db == nil {
		return nil, errors.New("the navigation database is not available")
	}
	if checked, err := s.nav.db.IdentChecked(ref.Ident); err == nil && !checked {
		_ = s.loadIdents([]string{ref.Ident})
	}
	return s.nav.db.FixAt(ref)
}

// POST /api/route/resolve  {"route": "SBSJ BOBI3A DAGUB UZ6 ..."}  (GET with ?route= as well)
func (s *Server) handleRouteResolve(w http.ResponseWriter, r *http.Request) {
	var text string
	if r.Method == http.MethodPost {
		var body struct {
			Route string `json:"route"`
		}
		if err := readJSON(r, &body); err != nil {
			writeErr(w, 400, err)
			return
		}
		text = body.Route
	} else {
		text = r.URL.Query().Get("route")
	}
	if strings.TrimSpace(text) == "" {
		writeErr(w, 400, errors.New("empty route"))
		return
	}
	s.nav.mu.Lock()
	defer s.nav.mu.Unlock()
	res, err := route.Resolve(text, routeProvider{s})
	if err != nil {
		writeErr(w, 502, err)
		return
	}
	writeJSON(w, 200, res)
}
