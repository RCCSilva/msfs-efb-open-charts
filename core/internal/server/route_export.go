package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/route"
)

type exportRequest struct {
	Route   string         `json:"route"`
	Choices []route.Choice `json:"choices"`
	ID      int64          `json:"id"` // a saved route: its places are used as they were kept
}

// exported gives the places of the route of the request for an export. A saved route (id) is used
// as it was kept; a written one is resolved here, and it needs every name to be found, because a
// part that was not found would be missing from the file. Both ends must be airports.
func (s *Server) exported(w http.ResponseWriter, r *http.Request) ([]route.Point, bool) {
	var body exportRequest
	if err := readJSON(r, &body); err != nil || (body.ID == 0 && strings.TrimSpace(body.Route) == "") {
		writeErr(w, 400, errors.New("send the route as {route, choices} or {id}"))
		return nil, false
	}
	svc, err := s.savedRoutes()
	if err != nil {
		writeErr(w, 502, err)
		return nil, false
	}
	text := body.Route
	choices, _ := json.Marshal(body.Choices)
	if body.ID != 0 {
		if saved, err := svc.Get(body.ID); err == nil {
			if len(saved.Result) > 0 {
				var res route.Result
				if err := json.Unmarshal(saved.Result, &res); err != nil {
					writeErr(w, 500, err)
					return nil, false
				}
				return res.Points, true
			}
			text, choices = saved.Text, saved.Choices // saved before the places were kept
		}
	}
	res, err := svc.Check(text, choices)
	if err != nil {
		writeErr(w, routeErrStatus(err), err)
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
	if s.sim.Plans == nil {
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
	if err := s.sim.Plans.Load(file); err != nil {
		writeErr(w, 502, err)
		return
	}
	writeJSON(w, 200, map[string]any{"file": file})
}
