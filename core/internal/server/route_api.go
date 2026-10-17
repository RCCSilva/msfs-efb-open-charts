package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/route"
)

// POST /api/route/distance  {"points": [{"lat": .., "lon": ..}, ...]}
// The straight-line length of each leg and of the whole route, in nautical miles (the planner asks
// this while the route is being built, without resolving any name).
func (s *Server) handleRouteDistance(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Points []struct {
			Lat float64 `json:"lat"`
			Lon float64 `json:"lon"`
		} `json:"points"`
	}
	if err := readJSON(r, &body); err != nil || len(body.Points) > 2000 {
		writeErr(w, 400, errors.New("send up to 2000 points as {lat, lon}"))
		return
	}
	lat := make([]float64, len(body.Points))
	lon := make([]float64, len(body.Points))
	for i, p := range body.Points {
		lat[i], lon[i] = p.Lat, p.Lon
	}
	legs, total := route.Legs(lat, lon)
	writeJSON(w, 200, map[string]any{"legs": legs, "totalNm": total})
}

// POST /api/route/resolve  {"route": "SBSJ BOBI3A DAGUB UZ6 ..."}  (GET with ?route= as well)
func (s *Server) handleRouteResolve(w http.ResponseWriter, r *http.Request) {
	var text string
	var choices []route.Choice
	if r.Method == http.MethodPost {
		var body struct {
			Route   string         `json:"route"`
			Choices []route.Choice `json:"choices"`
		}
		if err := readJSON(r, &body); err != nil {
			writeErr(w, 400, err)
			return
		}
		text, choices = body.Route, body.Choices
	} else {
		text = r.URL.Query().Get("route")
	}
	if strings.TrimSpace(text) == "" {
		writeErr(w, 400, errors.New("empty route"))
		return
	}
	s.nav.mu.Lock()
	defer s.nav.mu.Unlock()
	lk, err := s.lookup()
	if err != nil {
		writeErr(w, 502, err)
		return
	}
	res, err := route.ResolveWith(text, lk, choices)
	if err != nil {
		writeErr(w, 502, err)
		return
	}
	writeJSON(w, 200, res)
}
