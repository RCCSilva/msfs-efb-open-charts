// Package server is the local HTTP server: the viewer pages for the simulator, the chart API,
// groups/pins, PDF import and an OpenStreetMap tile cache.
package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/buildinfo"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/geotiff"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/jobs"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/setup"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
	"github.com/RCCSilva/msfs-efb-open-charts/core/web"
)

// Position is the aircraft state published to the viewers.
type Position struct {
	Valid       bool
	Lat, Lon    float64
	HeadingTrue float64
	GroundSpeed float64
	AltitudeFt  float64
	Connected   bool   // the position source is reachable (always true for "fixed")
	Detail      string // why it is not connected, for the viewer
	Source      string // "fixed", "simconnect"
	UpdatedAt   time.Time
}

// PositionSource provides the current aircraft position.
type PositionSource func() Position

// Server wires everything together.
type Server struct {
	lib         *store.Library
	newRenderer RendererFactory    // nil: importing disabled
	rend        *importer.Renderer // created on demand, see renderer.go
	rendIdle    *time.Timer
	position    PositionSource
	importMu    sync.Mutex // PDFium instance is single-threaded
	impTime     importTiming
	tiles       *tileCache
	jobs        *jobs.Manager
	staged      stagedStore
	net         netState // listening, port and network access, see access.go
	nav         navState // navigation data of the simulator, see nav_api.go
	addr        string   // the address being listened on; guarded by net.mu
}

// New builds a server. newRenderer may be nil (import then returns 503).
func New(lib *store.Library, newRenderer RendererFactory, pos PositionSource, addr string) *Server {
	s := &Server{lib: lib, newRenderer: newRenderer, position: pos, addr: addr,
		tiles: newTileCache(filepath.Join(lib.Dir(), "tiles")), jobs: jobs.NewManager(50)}
	s.loadFAAMetadata()
	return s
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		// The EFB app (another origin) polls this to show its status bar; it carries no data
		// worth protecting, so it is the only endpoint that allows cross-origin reads.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		p := s.position()
		writeJSON(w, 200, map[string]any{"ok": true, "app": "open-charts", "local": isLocalRequest(r),
			"version": buildinfo.Display(setup.EFBVersion),
			"charts":  s.lib.Count(), "importer": s.canImport(),
			"simConnected": p.Connected || p.Valid, "simulated": p.Source == "fixed"})
	})
	mux.HandleFunc("GET /api/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, buildinfo.Get(setup.EFBVersion))
	})
	mux.HandleFunc("GET /api/position", s.handlePosition)
	mux.HandleFunc("GET /api/nav/airport/{icao}", s.handleNavAirport)
	mux.HandleFunc("GET /api/nav/airports", s.handleNavAirports)
	mux.HandleFunc("GET /api/nav/fix/{region}/{ident}", s.handleNavFix)
	mux.HandleFunc("POST /api/route/resolve", s.handleRouteResolve)
	mux.HandleFunc("GET /api/db/status", s.handleDBStatus)
	mux.HandleFunc("POST /api/db/backup", s.handleDBBackup)
	mux.HandleFunc("POST /api/db/reset", s.handleDBReset)
	mux.HandleFunc("POST /api/db/restore", s.handleDBRestore)
	mux.HandleFunc("POST /api/db/import", s.handleDBImport)
	mux.HandleFunc("DELETE /api/db/backups/{name}", s.handleDBBackupDelete)
	mux.HandleFunc("POST /api/route/ivao", s.handleRouteIVAO)
	mux.HandleFunc("POST /api/route/pln", s.handleRoutePLN)
	mux.HandleFunc("POST /api/route/pln/load", s.handleRoutePLNLoad)
	mux.HandleFunc("GET /api/routes", s.handleRoutes)
	mux.HandleFunc("GET /api/routes/name", s.handleRouteName)
	mux.HandleFunc("POST /api/routes", s.handleRouteSave)
	mux.HandleFunc("PUT /api/routes/{id}", s.handleRouteSave)
	mux.HandleFunc("DELETE /api/routes/{id}", s.handleRouteDelete)
	mux.HandleFunc("GET /api/nav/summary", s.handleNavSummary)
	mux.HandleFunc("GET /api/nav/area", s.handleNavArea)
	mux.HandleFunc("GET /api/nav/search", s.handleNavSearch)
	mux.HandleFunc("POST /api/route/distance", s.handleRouteDistance)
	mux.HandleFunc("GET /api/nav/fixes", s.handleNavFixes)
	mux.HandleFunc("DELETE /api/nav/fixes", s.handleNavFixDelete)
	mux.HandleFunc("DELETE /api/nav/airport/{icao}", s.handleNavAirportDelete)
	mux.HandleFunc("GET /api/nav/airways", s.handleNavAirways)
	mux.HandleFunc("GET /api/nav/airway/{name}", s.handleNavAirway)
	mux.HandleFunc("GET /api/nav/points", s.handleNavPoints)
	mux.HandleFunc("POST /api/nav/points", s.handleNavPointSave)
	mux.HandleFunc("POST /api/nav/points/import", s.handleNavPointsImport)
	mux.HandleFunc("PUT /api/nav/points/{id}", s.handleNavPointSave)
	mux.HandleFunc("DELETE /api/nav/points/{id}", s.handleNavPointDelete)
	mux.HandleFunc("GET /api/nav/load", s.handleNavLoadStatus)
	mux.HandleFunc("POST /api/nav/load", s.handleNavLoadStart)
	mux.HandleFunc("DELETE /api/nav/load", s.handleNavLoadStop)
	mux.HandleFunc("GET /api/route/resolve", s.handleRouteResolve)
	mux.HandleFunc("GET /api/viewer-state", s.handleViewerState)
	mux.HandleFunc("POST /api/client-info", s.handlePostClientInfo)
	mux.HandleFunc("GET /api/client-info", s.handleGetClientInfo)
	mux.HandleFunc("POST /api/client-perf", s.handlePostClientPerf)
	mux.HandleFunc("GET /api/client-perf", s.handleGetClientPerf)
	mux.HandleFunc("PUT /api/viewer-state", s.handleSetViewerState)

	// charts
	mux.HandleFunc("GET /api/charts", s.handleSearch)
	mux.HandleFunc("GET /api/charts/footprints", s.handleFootprints)
	mux.HandleFunc("GET /api/library", s.handleLibrary)
	mux.HandleFunc("GET /api/airports", s.handleAirports)
	mux.HandleFunc("POST /api/library/delete", s.handleDeleteMany)
	mux.HandleFunc("GET /api/charts/{id}", s.handleChart)
	mux.HandleFunc("PATCH /api/charts/{id}", s.handleUpdateChart)
	mux.HandleFunc("DELETE /api/charts/{id}", s.handleDeleteChart)
	mux.HandleFunc("POST /api/import/inspect", s.handleInspect)
	mux.HandleFunc("POST /api/import", s.handleImport)
	mux.HandleFunc("GET /charts/{id}/{file}", s.handleChartFile)
	mux.HandleFunc("GET /charts/{id}/tiles/{z}/{x}/{y}", s.handleChartTile)

	// groups and pins
	mux.HandleFunc("GET /api/groups", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.lib.Groups()) })
	mux.HandleFunc("POST /api/groups", s.handleCreateGroup)
	mux.HandleFunc("PUT /api/groups/current", s.handleSetCurrent)
	mux.HandleFunc("PATCH /api/groups/{id}", s.handleRenameGroup)
	mux.HandleFunc("DELETE /api/groups/{id}", s.handleDeleteGroup)
	mux.HandleFunc("PUT /api/groups/{id}/pins/{chart}", s.handlePin)
	mux.HandleFunc("DELETE /api/groups/{id}/pins/{chart}", s.handleUnpin)

	s.registerSourceRoutes(mux)
	s.registerNamingRoutes(mux)
	s.registerSetupRoutes(mux)
	s.registerAccessRoutes(mux)

	// OpenStreetMap tiles (cached; the sim only talks to localhost over plain http)
	mux.HandleFunc("GET /tiles/{z}/{x}/{file}", s.tiles.handle)
	mux.HandleFunc("GET /tiles/{style}/{z}/{x}/{file}", s.tiles.handleStyled) // quiet grey / dark versions

	// viewer pages
	static := http.FileServer(http.FS(web.Sim()))
	mux.Handle("GET /", noCache(static))

	// manager UI (the app's own window; "/" opens it)
	mux.Handle("GET /manager/", noCache(http.StripPrefix("/manager/", http.FileServer(http.FS(web.Manager())))))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/manager/", http.StatusFound) })

	return s.guard(mux)
}

// ------------------------------------------------------------------ middleware

func noCache(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		h.ServeHTTP(w, r)
	})
}

// ------------------------------------------------------------------ helpers

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(v)
}

// ------------------------------------------------------------------ handlers

func (s *Server) handlePosition(w http.ResponseWriter, r *http.Request) {
	p := s.position()
	age := any(nil)
	if !p.UpdatedAt.IsZero() {
		age = time.Since(p.UpdatedAt).Milliseconds()
	}
	// Same shape the viewer pages already read.
	writeJSON(w, 200, map[string]any{
		"valid": p.Valid, "simConnected": p.Connected || p.Valid, "simulated": p.Source == "fixed", "detail": p.Detail,
		"lat": p.Lat, "lon": p.Lon, "headingTrue": p.HeadingTrue,
		"groundSpeedKnots": p.GroundSpeed, "altitudeFeet": p.AltitudeFt,
		"source": p.Source, "ageMs": age,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := store.Query{Text: q.Get("q"), Type: q.Get("type"), ICAO: q.Get("icao"), Source: q.Get("source")}
	if f := q.Get("group"); f != "" {
		for _, fl := range s.lib.Groups().Groups {
			if fl.ID == f || (f == "current" && fl.ID == s.lib.Groups().Current) {
				query.IDs = append([]string{}, fl.Pins...)
				if query.IDs == nil {
					query.IDs = []string{}
				}
			}
		}
	}
	res := s.lib.Search(query)
	limit := 500
	if n, err := strconv.Atoi(q.Get("limit")); err == nil && n > 0 {
		limit = n
	}
	if len(res) > limit {
		res = res[:limit]
	}
	if res == nil {
		res = []*store.Chart{}
	}
	writeJSON(w, 200, res)
}

func (s *Server) handleChart(w http.ResponseWriter, r *http.Request) {
	c, ok := s.lib.Get(r.PathValue("id"))
	if !ok {
		writeErr(w, 404, errors.New("chart not found"))
		return
	}
	writeJSON(w, 200, c)
}

// handleUpdateChart changes the editable properties of a chart (icao, type, title, validity).
// The name is generated from ICAO, type and title; a name in the body overrides that.
// All validation lives in store.Library.Update.
func (s *Server) handleUpdateChart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ICAO     *string `json:"icao"`
		Type     *string `json:"type"`
		Title    *string `json:"title"`
		Validity *string `json:"validity"`
		Name     *string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	c, err := s.lib.Update(r.PathValue("id"), store.Patch{
		ICAO: body.ICAO, Type: body.Type, Title: body.Title, Validity: body.Validity, Name: body.Name,
	})
	if err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, c)
}

func (s *Server) handleDeleteChart(w http.ResponseWriter, r *http.Request) {
	if err := s.lib.Delete(r.PathValue("id")); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleChartFile(w http.ResponseWriter, r *http.Request) {
	id, file := r.PathValue("id"), r.PathValue("file")
	if !store.ValidID(id) {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.lib.ChartDir(id), file)
	switch {
	case file == "chart.png" || file == "chart.json":
	case file == "source.pdf": // the original, shown beside the edit form
		w.Header().Set("Content-Type", "application/pdf")
	case levelFileRe.MatchString(file):
		p, ok := chartLevelPath(s.lib.ChartDir(id), file)
		if !ok {
			http.NotFound(w, r)
			return
		}
		path = p
	default:
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	// A chart's name, ICAO, type, title and validity can be edited, and a chart can be imported
	// again under the same id: the browser must revalidate (a cheap 304 on localhost) instead of
	// showing a copy it kept for an hour.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, path)
}

// handleImport accepts multipart form: file (PDF) and optional name, icao, type, title, source.
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	if !s.canImport() {
		writeErr(w, 503, errors.New("importer unavailable"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeErr(w, 400, err)
		return
	}
	var data []byte
	var filename string
	token := strings.TrimSpace(r.FormValue("token"))
	if token != "" {
		st, ok := s.staged.get(token)
		if !ok {
			writeErr(w, 410, errors.New("the inspected file expired: add it again"))
			return
		}
		data, filename = st.data, st.filename
	} else {
		file, hdr, err := r.FormFile("file")
		if err != nil {
			writeErr(w, 400, errors.New("missing form field \"file\""))
			return
		}
		defer file.Close()
		if data, err = io.ReadAll(file); err != nil {
			writeErr(w, 400, err)
			return
		}
		filename = hdr.Filename
	}
	if !geotiff.IsTIFF(data) && (len(data) < 8 || string(data[:5]) != "%PDF-") {
		writeErr(w, 400, errors.New("not a PDF or GeoTIFF file"))
		return
	}

	form := importForm{
		Source:   r.FormValue("source"),
		Name:     r.FormValue("name"),
		ICAO:     r.FormValue("icao"),
		Type:     r.FormValue("type"),
		Title:    r.FormValue("title"),
		Validity: r.FormValue("validity"),
	}

	// async: answer at once with a job; the UI follows its stage and percentage.
	if r.FormValue("async") != "" {
		job := s.jobs.Start("import", 1, func(j *jobs.Job) {
			form.Stage = j.SetStage
			c, err := s.importPDF(data, filename, form)
			if err != nil {
				st, msg := itemStatus(err)
				it := jobs.Item{File: filename, Status: st, Message: msg}
				if c != nil {
					it.ChartID, it.Name = c.ID, c.Name
				}
				j.Add(it)
				return
			}
			if token != "" {
				s.staged.drop(token)
			}
			j.Add(jobs.Item{File: filename, ChartID: c.ID, Name: c.Name, Status: "ok"})
		})
		snap, _ := s.jobs.Get(job.ID())
		writeJSON(w, 202, snap)
		return
	}

	c, err := s.importPDF(data, filename, form)
	if err != nil {
		writeErr(w, importStatus(err), err)
		return
	}
	if token != "" {
		s.staged.drop(token)
	}
	writeJSON(w, 201, c)
}

func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = readJSON(r, &body)
	f, err := s.lib.CreateGroup(body.Name)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 201, f)
}

func (s *Server) handleSetCurrent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.lib.SetCurrent(body.ID); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, s.lib.Groups())
}

func (s *Server) handleRenameGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.lib.RenameGroup(r.PathValue("id"), body.Name); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, s.lib.Groups())
}

func (s *Server) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	if err := s.lib.DeleteGroup(r.PathValue("id")); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, s.lib.Groups())
}

func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	if err := s.lib.Pin(r.PathValue("id"), r.PathValue("chart")); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, s.lib.Groups())
}

func (s *Server) handleUnpin(w http.ResponseWriter, r *http.Request) {
	if err := s.lib.Unpin(r.PathValue("id"), r.PathValue("chart")); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, s.lib.Groups())
}

func statusFor(err error) int {
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, fs.ErrNotExist) {
		return http.StatusNotFound
	}
	return http.StatusBadRequest
}

var _ = fmt.Sprintf

// titleFallback: charts without an ICAO (visual charts) are better named by the PDF's own
// viewport name ("CCV REA XP2-SÃƒÆ’O PAULO") than by the file name, so leave it to the importer.
func titleFallback(info naming.Info) string {
	if info.ICAO == "" {
		return ""
	}
	return info.Title
}
