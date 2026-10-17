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
	var raw json.RawMessage
	if body.ID != 0 {
		if db := s.navDB(); db != nil {
			if sr, err := db.SavedRoute(body.ID); err == nil {
				raw = sr.Result
				if len(raw) == 0 {
					body.Route = sr.Text
					body.Choices = nil
					_ = json.Unmarshal(sr.Choices, &body.Choices)
				}
			}
		}
	}
	if len(raw) == 0 {
		choices, _ := json.Marshal(body.Choices)
		var code int
		var err error
		if raw, code, err = s.validRoute(body.Route, choices); err != nil {
			writeErr(w, code, err)
			return nil, false
		}
	}
	var res route.Result
	if err := json.Unmarshal(raw, &res); err != nil {
		writeErr(w, 500, err)
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
