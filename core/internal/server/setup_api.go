package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/buildinfo"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/setup"
)

func (s *Server) registerSetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/setup", s.handleSetup)
	mux.HandleFunc("POST /api/setup/efb", s.handleInstallEFB)
	mux.HandleFunc("DELETE /api/setup/efb", s.handleRemoveEFB)
	mux.HandleFunc("PUT /api/setup/autostart", s.handleAutoStart)
}

func (s *Server) exePath() string {
	p, err := os.Executable()
	if err != nil {
		return ""
	}
	return p
}

// serverURL is the address the EFB app must use: this computer, the current port.
func (s *Server) serverURL() string { return s.LocalURL() }

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	installs := setup.Detect(s.exePath())
	for i := range installs {
		e := &installs[i].EFB
		e.URLMismatch = e.Installed && e.ServerURL != "" && e.ServerURL != s.serverURL()
	}
	if installs == nil {
		installs = []setup.Install{}
	}
	p := s.position()
	writeJSON(w, 200, map[string]any{
		"exe": s.exePath(), "dataDir": s.lib.Dir(), "addr": s.listenAddr(), "serverURL": s.serverURL(),
		"efbVersion": setup.EFBVersion, "program": buildinfo.Get(setup.EFBVersion), "installs": installs,
		"sim": map[string]any{"source": p.Source, "connected": p.Connected || p.Valid, "receiving": p.Valid, "detail": p.Detail},
	})
}

// communityFor accepts a folder reported by Detect, or any existing folder named Community*.
func communityFor(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("community folder is required")
	}
	p = filepath.Clean(p)
	return p, setup.ValidCommunity(p)
}

func (s *Server) handleInstallEFB(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Community string `json:"community"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	c, err := communityFor(body.Community)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := setup.InstallEFB(c, s.serverURL()); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, setup.EFBState(c))
}

func (s *Server) handleRemoveEFB(w http.ResponseWriter, r *http.Request) {
	c, err := communityFor(r.URL.Query().Get("community"))
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := setup.RemoveEFB(c); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, setup.EFBState(c))
}

func (s *Server) handleAutoStart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ExeXML  string `json:"exeXml"`
		Enabled bool   `json:"enabled"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	// Only the EXE.xml of an installation that was detected: this endpoint must not write
	// arbitrary files.
	ok := false
	for _, in := range setup.Detect(s.exePath()) {
		if in.ExeXML == body.ExeXML {
			ok = true
		}
	}
	if !ok {
		writeErr(w, 400, errors.New("unknown simulator installation"))
		return
	}
	// Only what differs from the defaults goes into EXE.xml: the port and the network option
	// come from settings.json, so changing them in the manager needs no EXE.xml edit.
	var args []string
	s.net.mu.RLock()
	defDir := s.net.defaultDataDir
	s.net.mu.RUnlock()
	if dir := s.lib.Dir(); dir != defDir {
		if strings.ContainsAny(dir, " \t") {
			dir = `"` + dir + `"`
		}
		args = []string{"-data", dir}
	}
	if err := setup.SetAutoStart(body.ExeXML, s.exePath(), args, body.Enabled); err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"autoStart": setup.AutoStartEnabled(body.ExeXML)})
}
