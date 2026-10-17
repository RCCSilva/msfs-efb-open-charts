package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/route"
)

// PlanLoader asks the simulator to load a flight plan file (see simconnect.LoadFlightPlan).
type PlanLoader func(file string) error

// SetPlanLoader gives the server the way to load a flight plan in the simulator.
func (s *Server) SetPlanLoader(l PlanLoader) {
	s.nav.ptrMu.Lock()
	s.nav.planLoad = l
	s.nav.ptrMu.Unlock()
}

type exportRequest struct {
	Route   string         `json:"route"`
	Choices []route.Choice `json:"choices"`
}

// exported resolves the route of the request for an export: it needs both ends to be airports and
// every name to be found, because a part that was not found would be missing from the file.
func (s *Server) exported(w http.ResponseWriter, r *http.Request) ([]route.Point, bool) {
	var body exportRequest
	if err := readJSON(r, &body); err != nil || strings.TrimSpace(body.Route) == "" {
		writeErr(w, 400, errors.New("send the route as {route, choices}"))
		return nil, false
	}
	s.nav.mu.Lock()
	res, err := route.ResolveWith(body.Route, routeProvider{s}, body.Choices)
	s.nav.mu.Unlock()
	if err != nil {
		writeErr(w, 502, err)
		return nil, false
	}
	var missing []string
	for _, p := range res.Problems {
		if len(p.Candidates) == 0 {
			missing = append(missing, p.Token)
		}
	}
	if len(missing) > 0 {
		writeErr(w, 422, errors.New("these names were not found: "+strings.Join(missing, ", ")))
		return nil, false
	}
	return res.Points, true
}

// POST /api/route/ivao  {"route": "...", "choices": [...]}
// The address that opens the IVAO flight plan form with the origin, the destination and the route.
func (s *Server) handleRouteIVAO(w http.ResponseWriter, r *http.Request) {
	pts, ok := s.exported(w, r)
	if !ok {
		return
	}
	link, plan, err := route.IVAOLink(pts)
	if err != nil {
		writeErr(w, 422, err)
		return
	}
	writeJSON(w, 200, map[string]any{"url": link, "departure": plan.DepartureID, "arrival": plan.ArrivalID, "route": plan.Route})
}

var fileSafe = regexp.MustCompile(`[^A-Za-z0-9_-]+`)

func planName(pts []route.Point) string {
	return fileSafe.ReplaceAllString(pts[0].Ident+"-"+pts[len(pts)-1].Ident, "") + ".pln"
}

// POST /api/route/pln  {"route": "...", "choices": [...]}
// The route as a flight plan file of the simulator, to save.
func (s *Server) handleRoutePLN(w http.ResponseWriter, r *http.Request) {
	pts, ok := s.exported(w, r)
	if !ok {
		return
	}
	data, err := route.PLN(pts, 0)
	if err != nil {
		writeErr(w, 422, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+planName(pts)+`"`)
	_, _ = w.Write(data)
}

// POST /api/route/pln/load  {"route": "...", "choices": [...]}
// The same file is kept in the data folder and the simulator is asked to load it.
func (s *Server) handleRoutePLNLoad(w http.ResponseWriter, r *http.Request) {
	s.nav.ptrMu.RLock()
	load := s.nav.planLoad
	s.nav.ptrMu.RUnlock()
	if load == nil {
		writeErr(w, 503, errors.New("there is no link to the simulator (SimConnect.dll was not found)"))
		return
	}
	pts, ok := s.exported(w, r)
	if !ok {
		return
	}
	data, err := route.PLN(pts, 0)
	if err != nil {
		writeErr(w, 422, err)
		return
	}
	s.net.mu.Lock()
	dir := s.net.dataDir
	s.net.mu.Unlock()
	dir = filepath.Join(dir, "flightplans")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		writeErr(w, 500, err)
		return
	}
	file, err := filepath.Abs(filepath.Join(dir, planName(pts)))
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	if err := os.WriteFile(file, data, 0o644); err != nil {
		writeErr(w, 500, err)
		return
	}
	if err := load(file); err != nil {
		writeErr(w, 502, err)
		return
	}
	writeJSON(w, 200, map[string]any{"file": file})
}
