// Package savedroutes keeps the routes the user saves. A route is checked when it is saved (every
// name must be found) and is kept with the places it was resolved to, so that opening it, drawing it
// or sending it somewhere does not resolve the text again.
package savedroutes

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/route"
)

// Store is where the saved routes are kept.
type Store interface {
	SavedRoutes() ([]navdb.SavedRoute, error)
	SavedRoute(id int64) (navdb.SavedRoute, error)
	CreateSavedRoute(r navdb.SavedRoute) (navdb.SavedRoute, error)
	UpdateSavedRoute(r navdb.SavedRoute) (navdb.SavedRoute, error)
	DeleteSavedRoute(id int64) error
	SetSavedRouteResult(id int64, result json.RawMessage) error
}

// Resolver reads the text of a route, finding each of its places.
type Resolver interface {
	Resolve(text string, choices []route.Choice) (*route.Result, error)
}

// ErrEmpty is returned for a route with nothing written.
var ErrEmpty = errors.New("the route is empty")

// ErrNothingFound is returned for a route none of whose places was found.
var ErrNothingFound = errors.New("no place of this route was found")

// NotFoundError is returned for a route with names that were not found: a part that was not found
// would be missing from the route.
type NotFoundError struct{ Names []string }

func (e *NotFoundError) Error() string {
	return "these names were not found: " + strings.Join(e.Names, ", ")
}

// ResolveError is returned when the route could not be read at all (the navigation data failed).
type ResolveError struct{ Err error }

func (e *ResolveError) Error() string { return e.Err.Error() }
func (e *ResolveError) Unwrap() error { return e.Err }

// View is a saved route as the manager and the viewer read it.
type View struct {
	navdb.SavedRoute
	Summary string `json:"summary"` // "SBSJ > SBGR"
	Names   int    `json:"names"`
}

// Name is the name a route gets from what it says: its first and last place.
func Name(text string) string {
	toks := route.Tokens(text)
	switch {
	case len(toks) == 1:
		return toks[0]
	case len(toks) > 1:
		return toks[0] + " > " + toks[len(toks)-1]
	}
	return ""
}

func view(r navdb.SavedRoute) View {
	return View{SavedRoute: r, Names: len(route.Tokens(r.Text)), Summary: Name(r.Text)}
}

// Service saves, lists and deletes routes.
type Service struct {
	store    Store
	resolver Resolver
}

// New builds a Service.
func New(store Store, resolver Resolver) *Service {
	return &Service{store: store, resolver: resolver}
}

// Check resolves the text of a route and refuses it when a name was not found or nothing was.
func (s *Service) Check(text string, rawChoices json.RawMessage) (*route.Result, error) {
	var choices []route.Choice
	if len(rawChoices) > 0 {
		_ = json.Unmarshal(rawChoices, &choices)
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return nil, ErrEmpty
	}
	res, err := s.resolver.Resolve(text, choices)
	if err != nil {
		return nil, &ResolveError{Err: err}
	}
	var missing []string
	for _, p := range res.Problems {
		if len(p.Candidates) == 0 {
			missing = append(missing, p.Token)
		}
	}
	if len(missing) > 0 {
		return nil, &NotFoundError{Names: missing}
	}
	if len(res.Points) == 0 {
		return nil, ErrNothingFound
	}
	return res, nil
}

// Get returns one saved route.
func (s *Service) Get(id int64) (navdb.SavedRoute, error) { return s.store.SavedRoute(id) }

// List returns the saved routes. One saved by an earlier version, with no places kept, is
// resolved once and kept.
func (s *Service) List() ([]View, error) {
	list, err := s.store.SavedRoutes()
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(list))
	for _, r := range list {
		if len(r.Result) == 0 {
			if res, err := s.Check(r.Text, r.Choices); err == nil {
				if raw, err := json.Marshal(res); err == nil && s.store.SetSavedRouteResult(r.ID, raw) == nil {
					r.Result = raw
				}
			}
		}
		out = append(out, view(r))
	}
	return out, nil
}

// prepare gives a route its name, when it has none, and its places, which are checked.
func (s *Service) prepare(r navdb.SavedRoute) (navdb.SavedRoute, error) {
	if strings.TrimSpace(r.Name) == "" {
		r.Name = Name(r.Text) // a route without a name is called after where it goes
	}
	res, err := s.Check(r.Text, r.Choices)
	if err != nil {
		return r, err
	}
	r.Result, err = json.Marshal(res)
	return r, err
}

// Create checks a route and keeps it, with its places, as a new one.
func (s *Service) Create(r navdb.SavedRoute) (View, error) {
	r, err := s.prepare(r)
	if err != nil {
		return View{}, err
	}
	saved, err := s.store.CreateSavedRoute(r)
	if err != nil {
		return View{}, err
	}
	return view(saved), nil
}

// Update checks a route and keeps it, with its places, in place of the one with that id.
func (s *Service) Update(id int64, r navdb.SavedRoute) (View, error) {
	r.ID = id
	r, err := s.prepare(r)
	if err != nil {
		return View{}, err
	}
	saved, err := s.store.UpdateSavedRoute(r)
	if err != nil {
		return View{}, err
	}
	return view(saved), nil
}

// Delete removes a saved route.
func (s *Service) Delete(id int64) error { return s.store.DeleteSavedRoute(id) }
