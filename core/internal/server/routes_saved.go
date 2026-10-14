package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/route"
)

// The saved routes: written in the manager, one tap in the viewer. The server only keeps the name
// and the text; turning the text into places is what /api/route/resolve does, each time.

type savedRouteView struct {
	navdb.SavedRoute
	Summary string `json:"summary"` // "SBSJ > SBGR"
	Names   int    `json:"names"`
}

// routeName is the name a route gets from what it says: its first and last place.
func routeName(text string) string {
	toks := route.Tokens(text)
	switch {
	case len(toks) == 1:
		return toks[0]
	case len(toks) > 1:
		return toks[0] + " > " + toks[len(toks)-1]
	}
	return ""
}

func summarize(r navdb.SavedRoute) savedRouteView {
	return savedRouteView{SavedRoute: r, Names: len(route.Tokens(r.Text)), Summary: routeName(r.Text)}
}

// GET /api/routes/name?text=...: the name a route would get (the manager fills the field with it).
func (s *Server) handleRouteName(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"name": routeName(r.URL.Query().Get("text"))})
}

// GET /api/routes
func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	list, err := db.SavedRoutes()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	out := make([]savedRouteView, 0, len(list))
	for _, x := range list {
		out = append(out, summarize(x))
	}
	writeJSON(w, 200, map[string]any{"routes": out})
}

// POST /api/routes and PUT /api/routes/{id}   {"name": "...", "text": "SBSJ BOBI3A ... SBGR"}
func (s *Server) handleRouteSave(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	var body navdb.SavedRoute
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		body.Name = routeName(body.Text) // a route without a name is called after where it goes
	}
	var saved navdb.SavedRoute
	var err error
	if idText := r.PathValue("id"); idText != "" {
		body.ID, err = strconv.ParseInt(idText, 10, 64)
		if err == nil {
			saved, err = db.UpdateSavedRoute(body)
		}
	} else {
		saved, err = db.CreateSavedRoute(body)
	}
	switch {
	case errors.Is(err, navdb.ErrInvalid):
		writeErr(w, 400, err)
	case errors.Is(err, navdb.ErrNotFound):
		writeErr(w, 404, err)
	case err != nil:
		writeErr(w, 500, err)
	default:
		writeJSON(w, 200, summarize(saved))
	}
}

// DELETE /api/routes/{id}
func (s *Server) handleRouteDelete(w http.ResponseWriter, r *http.Request) {
	db := s.navDB()
	if db == nil {
		navUnavailable(w)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		err = db.DeleteSavedRoute(id)
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
