package server

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/route"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/savedroutes"
)

// The saved routes: written in the manager, one tap in the viewer. What a saved route is, and the
// check that a route is valid when it is saved, is in package savedroutes; this is its HTTP face.

// routeResolver reads the text of a route with the navigation data, one fetch at a time.
type routeResolver struct{ s *Server }

func (r routeResolver) Resolve(text string, choices []route.Choice) (*route.Result, error) {
	lk, err := r.s.lookup()
	if err != nil {
		return nil, err
	}
	r.s.nav.mu.Lock()
	defer r.s.nav.mu.Unlock()
	return route.ResolveWith(text, lk, choices)
}

// savedRoutes is the service of the saved routes, made for each use because the database can be
// replaced while the program runs.
func (s *Server) savedRoutes() (*savedroutes.Service, error) {
	db := s.navDB()
	if db == nil {
		return nil, errors.New("the navigation database is not available")
	}
	return savedroutes.New(db, routeResolver{s}), nil
}

// routeErrStatus is the HTTP status of an error of the saved routes.
func routeErrStatus(err error) int {
	var notFound *savedroutes.NotFoundError
	var resolve *savedroutes.ResolveError
	switch {
	case errors.Is(err, savedroutes.ErrEmpty), errors.Is(err, navdb.ErrInvalid):
		return 400
	case errors.As(err, &notFound), errors.Is(err, savedroutes.ErrNothingFound):
		return 422
	case errors.As(err, &resolve):
		return 502
	case errors.Is(err, navdb.ErrNotFound):
		return 404
	}
	return 500
}

// GET /api/routes/name?text=...: the name a route would get (the manager fills the field with it).
func (s *Server) handleRouteName(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"name": savedroutes.Name(r.URL.Query().Get("text"))})
}

// GET /api/routes
func (s *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	svc, err := s.savedRoutes()
	if err != nil {
		navUnavailable(w)
		return
	}
	list, err := svc.List()
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"routes": list})
}

// POST /api/routes and PUT /api/routes/{id}   {"name": "...", "text": "SBSJ BOBI3A ... SBGR"}
func (s *Server) handleRouteSave(w http.ResponseWriter, r *http.Request) {
	svc, err := s.savedRoutes()
	if err != nil {
		navUnavailable(w)
		return
	}
	var body navdb.SavedRoute
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	var saved savedroutes.View
	if idText := r.PathValue("id"); idText != "" {
		id, perr := strconv.ParseInt(idText, 10, 64)
		if perr != nil {
			writeErr(w, 400, perr)
			return
		}
		saved, err = svc.Update(id, body)
	} else {
		saved, err = svc.Create(body)
	}
	if err != nil {
		writeErr(w, routeErrStatus(err), err)
		return
	}
	writeJSON(w, 200, saved)
}

// DELETE /api/routes/{id}
func (s *Server) handleRouteDelete(w http.ResponseWriter, r *http.Request) {
	svc, err := s.savedRoutes()
	if err != nil {
		navUnavailable(w)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := svc.Delete(id); err != nil {
		writeErr(w, routeErrStatus(err), err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}
