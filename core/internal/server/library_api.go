package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// maxLibraryPage bounds one page of the library.
const maxLibraryPage = 200

// handleLibrary is the library as the manager shows it: a page of charts plus how many there are
// in all, so the screen can page, select everything that matches and say how many that is.
//
//	GET /api/library?q=&type=&icao=&source=&offset=&limit=&ids=1
//
// With ids=1 the answer carries every matching id (and no items): "select all N results".
func (s *Server) handleLibrary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := store.Query{Text: q.Get("q"), Type: q.Get("type"), ICAO: q.Get("icao"), Source: q.Get("source")}
	// icao=- asks for the charts that have no ICAO (visual charts, for example)
	noICAO := query.ICAO == "-"
	if noICAO {
		query.ICAO = ""
	}
	res := s.lib.Search(query)
	if noICAO {
		kept := res[:0]
		for _, c := range res {
			if c.ICAO == "" {
				kept = append(kept, c)
			}
		}
		res = kept
	}
	out := map[string]any{"total": len(res), "types": s.lib.Types(), "all": s.lib.Count()}
	if q.Get("ids") == "1" {
		ids := make([]string, len(res))
		for i, c := range res {
			ids[i] = c.ID
		}
		out["ids"] = ids
		writeJSON(w, 200, out)
		return
	}
	offset, limit := 0, 50
	if n, err := strconv.Atoi(q.Get("offset")); err == nil && n > 0 {
		offset = n
	}
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = min(n, maxLibraryPage)
	}
	if offset > len(res) {
		offset = len(res)
	}
	end := min(offset+limit, len(res))
	items := res[offset:end]
	if items == nil {
		items = []*store.Chart{}
	}
	out["items"], out["offset"], out["limit"] = items, offset, limit
	writeJSON(w, 200, out)
}

// handleAirports lists the aerodromes of the library (optionally those starting with ?q=).
func (s *Server) handleAirports(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"airports": s.lib.Airports(strings.TrimSpace(r.URL.Query().Get("q")))})
}

// handleDeleteMany removes the charts the user selected. It is the one place a bulk delete is
// decided: the screen only sends the ids.
func (s *Server) handleDeleteMany(w http.ResponseWriter, r *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	if len(body.IDs) == 0 {
		writeErr(w, 400, errNoIDs)
		return
	}
	n, err := s.lib.DeleteMany(body.IDs)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"deleted": n, "groups": s.lib.Groups()})
}

var errNoIDs = errors.New("no charts selected")
