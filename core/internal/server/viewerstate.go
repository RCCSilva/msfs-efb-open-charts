package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// viewerState is what the in-sim viewer should come back to after its page is reloaded.
// The EFB app always loads the same URL, so the state is kept here, not in the page.
type viewerState struct {
	Chart string `json:"chart"`
	Mode  string `json:"mode"` // "overlay" | "chart"
	Lang  string `json:"lang"` // "pt" | "en" | ""
}

var viewerStateMu sync.Mutex

func (s *Server) viewerStatePath() string { return filepath.Join(s.lib.Dir(), "viewer-state.json") }

func (s *Server) handleViewerState(w http.ResponseWriter, r *http.Request) {
	viewerStateMu.Lock()
	defer viewerStateMu.Unlock()
	st := viewerState{}
	if b, err := os.ReadFile(s.viewerStatePath()); err == nil {
		_ = json.Unmarshal(b, &st)
	}
	// A chart that was deleted or renamed must not be offered back.
	if st.Chart != "" {
		if _, ok := s.lib.Get(st.Chart); !ok {
			st.Chart = ""
		}
	}
	writeJSON(w, 200, st)
}

func (s *Server) handleSetViewerState(w http.ResponseWriter, r *http.Request) {
	var st viewerState
	if err := readJSON(r, &st); err != nil {
		writeErr(w, 400, err)
		return
	}
	if st.Mode != "" && st.Mode != "overlay" && st.Mode != "chart" {
		writeErr(w, 400, errors.New("invalid mode"))
		return
	}
	if st.Lang != "" && st.Lang != "pt" && st.Lang != "en" {
		writeErr(w, 400, errors.New("invalid lang"))
		return
	}
	if st.Chart != "" && !store.ValidID(st.Chart) {
		writeErr(w, 400, errors.New("invalid chart id"))
		return
	}
	b, _ := json.Marshal(st)
	viewerStateMu.Lock()
	defer viewerStateMu.Unlock()
	tmp := s.viewerStatePath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		writeErr(w, 500, err)
		return
	}
	if err := os.Rename(tmp, s.viewerStatePath()); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, st)
}
