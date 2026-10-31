package server

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/buildinfo"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
)

func (s *Server) registerSetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sim", s.handleSim)
	mux.HandleFunc("GET /api/setup", s.handleSetup)
	mux.HandleFunc("POST /api/setup/efb", s.handleInstallEFB)
	mux.HandleFunc("DELETE /api/setup/efb", s.handleRemoveEFB)
	mux.HandleFunc("PUT /api/setup/autostart", s.handleAutoStart)
	mux.HandleFunc("POST /api/setup/open-data-folder", s.handleOpenDataFolder)
}

// UseOpener gives the server the way to show a folder in the file explorer of the computer. Without it the
// manager's "Open folder" answers that it is not available.
func (s *Server) UseOpener(open func(path string)) { s.openPath = open }

// handleOpenDataFolder shows the data folder in the file explorer. It takes no path from the request: this
// is the only folder it can open (and, like all of /api/setup, it is refused to a device of the network).
func (s *Server) handleOpenDataFolder(w http.ResponseWriter, r *http.Request) {
	if s.openPath == nil {
		writeErr(w, http.StatusNotImplemented, errors.New("opening a folder is not available here"))
		return
	}
	s.openPath(s.lib.Dir())
	w.WriteHeader(http.StatusNoContent)
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

// GET /api/sim: which simulator this is and what the program can do with it.
func (s *Server) handleSim(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"id": s.sim.ID, "name": s.sim.Name, "capabilities": s.sim.Capabilities()})
}

// installView is an installation of the simulator as the manager reads it.
type installView struct {
	Edition      string       `json:"edition"`
	PackagesPath string       `json:"packagesPath"`
	Community    string       `json:"community"` // where the app goes
	ExeXML       string       `json:"exeXml"`    // what starts the program with the simulator
	EFB          sim.AppState `json:"efb"`
	AutoStart    bool         `json:"autoStart"`
}

// installations lists where the simulator is installed (none when it has no app to install).
func (s *Server) installations() []sim.AppInstallation {
	if s.sim.App == nil {
		return nil
	}
	return s.sim.App.Detect(s.exePath())
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	installs := []installView{}
	for _, in := range s.installations() {
		in.App.URLMismatch = in.App.Installed && in.App.ServerURL != "" && in.App.ServerURL != s.serverURL()
		installs = append(installs, installView{Edition: in.Edition, PackagesPath: in.PackagesPath, Community: in.Target,
			ExeXML: in.AutoStartFile, EFB: in.App, AutoStart: in.AutoStart})
	}
	p := s.currentPosition()
	writeJSON(w, 200, map[string]any{
		"exe": s.exePath(), "dataDir": s.lib.Dir(), "addr": s.listenAddr(), "serverURL": s.serverURL(),
		"efbVersion": buildinfo.Version, "program": buildinfo.Get(buildinfo.Version), "installs": installs,
		"sim": map[string]any{"source": p.Source, "connected": p.Connected || p.Valid, "receiving": p.Valid, "detail": p.Detail},
	})
}

var errNoApp = errors.New("this simulator has no app to install")

// targetFor accepts a folder reported by Detect, or any folder the installer accepts.
func (s *Server) targetFor(p string) (string, error) {
	if s.sim.App == nil {
		return "", errNoApp
	}
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("community folder is required")
	}
	p = filepath.Clean(p)
	return p, s.sim.App.ValidTarget(p)
}

func (s *Server) handleInstallEFB(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Community string `json:"community"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	c, err := s.targetFor(body.Community)
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.sim.App.Install(c, s.serverURL()); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, s.sim.App.State(c))
}

func (s *Server) handleRemoveEFB(w http.ResponseWriter, r *http.Request) {
	c, err := s.targetFor(r.URL.Query().Get("community"))
	if err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.sim.App.Remove(c); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, s.sim.App.State(c))
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
	for _, in := range s.installations() {
		if in.AutoStartFile == body.ExeXML {
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
	if err := s.sim.App.SetAutoStart(body.ExeXML, s.exePath(), args, body.Enabled); err != nil {
		writeErr(w, 400, err)
		return
	}
	writeJSON(w, 200, map[string]any{"autoStart": s.sim.App.AutoStart(body.ExeXML)})
}
