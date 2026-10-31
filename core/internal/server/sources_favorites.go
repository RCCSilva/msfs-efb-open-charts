package server

import (
	"errors"
	"net/http"
	"slices"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sources"
)

// sourceView is a source as the manager lists it: its description and whether the user favourited it. A favourite source
// gets a tab of its own in the manager.
type sourceView struct {
	sources.Info
	Favorite bool `json:"favorite"`
}

// sourceViews lists the sources, in the registry's order, marking the favourites.
func (s *Server) sourceViews() []sourceView {
	fav := s.settingsSnapshot().FavoriteSources
	all := sources.All()
	out := make([]sourceView, 0, len(all))
	for _, in := range all {
		out = append(out, sourceView{Info: in, Favorite: slices.Contains(fav, in.ID)})
	}
	return out
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.sourceViews())
}

// handleFavoriteSource marks (PUT) or unmarks (DELETE) a source as a favourite.
func (s *Server) handleFavoriteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := sources.Get(id); !ok {
		writeErr(w, http.StatusNotFound, errors.New("unknown source"))
		return
	}
	on := r.Method == http.MethodPut
	s.net.mu.Lock()
	dir := s.net.dataDir
	fav := slices.DeleteFunc(slices.Clone(s.net.cfg.FavoriteSources), func(x string) bool { return x == id })
	if on {
		fav = append(fav, id)
	}
	s.net.cfg.FavoriteSources = fav
	cfg := s.net.cfg
	s.net.mu.Unlock()
	if err := cfg.Save(dir); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, 200, s.sourceViews())
}
