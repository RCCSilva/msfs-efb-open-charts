package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/route"
)

// The saved routes: written in the manager, one tap in the viewer. A route is checked when it is
// saved (every name must be found) and is kept with the places it was resolved to, so that loading
// it, drawing it or sending it somewhere does not resolve the text again.

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
		if len(x.Result) == 0 {
			// saved by an earlier version: it is resolved once and kept
			if raw, _, err := s.validRoute(x.Text, x.Choices); err == nil {
				if db.SetSavedRouteResult(x.ID, raw) == nil {
					x.Result = raw
				}
			}
		}
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
	// a saved route is a valid one: every name found, the places kept with it
	raw, code, err := s.validRoute(body.Text, body.Choices)
	if err != nil {
		writeErr(w, code, err)
		return
	}
	body.Result = raw
	var saved navdb.SavedRoute
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

// validRoute resolves the text of a route and refuses it when a name was not found (a part that was
// not found would be missing). It returns the resolved route as JSON, ready to be kept.
// The second value is the HTTP status of the refusal.
func (s *Server) validRoute(text string, rawChoices json.RawMessage) (json.RawMessage, int, error) {
	var choices []route.Choice
	if len(rawChoices) > 0 {
		_ = json.Unmarshal(rawChoices, &choices)
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return nil, 400, errors.New("the route is empty")
	}
	lk, err := s.lookup()
	if err != nil {
		return nil, 502, err
	}
	s.nav.mu.Lock()
	res, err := route.ResolveWith(text, lk, choices)
	s.nav.mu.Unlock()
	if err != nil {
		return nil, 502, err
	}
	var missing []string
	for _, p := range res.Problems {
		if len(p.Candidates) == 0 {
			missing = append(missing, p.Token)
		}
	}
	if len(missing) > 0 {
		return nil, 422, errors.New("these names were not found: " + strings.Join(missing, ", "))
	}
	if len(res.Points) == 0 {
		return nil, 422, errors.New("no place of this route was found")
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return nil, 500, err
	}
	return raw, 0, nil
}
