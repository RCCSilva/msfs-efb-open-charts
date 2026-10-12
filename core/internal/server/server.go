// Package server is the local HTTP server: the viewer pages for the simulator, the chart API,
// flights/pins, PDF import and an OpenStreetMap tile cache.
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

	"localmovingmap/internal/importer"
	"localmovingmap/internal/jobs"
	"localmovingmap/internal/naming"
	"localmovingmap/internal/store"
	"localmovingmap/web"
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
	tiles       *tileCache
	jobs        *jobs.Manager
	staged      stagedStore
	addr        string
}

// New builds a server. newRenderer may be nil (import then returns 503).
func New(lib *store.Library, newRenderer RendererFactory, pos PositionSource, addr string) *Server {
	return &Server{lib: lib, newRenderer: newRenderer, position: pos, addr: addr,
		tiles: newTileCache(filepath.Join(lib.Dir(), "tiles")), jobs: jobs.NewManager(50)}
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		// The EFB app (another origin) polls this to show its status bar; it carries no data
		// worth protecting, so it is the only endpoint that allows cross-origin reads.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		p := s.position()
		writeJSON(w, 200, map[string]any{"ok": true, "charts": s.lib.Count(), "importer": s.canImport(),
			"simConnected": p.Connected || p.Valid, "simulated": p.Source == "fixed"})
	})
	mux.HandleFunc("GET /api/position", s.handlePosition)
	mux.HandleFunc("GET /api/viewer-state", s.handleViewerState)
	mux.HandleFunc("PUT /api/viewer-state", s.handleSetViewerState)

	// charts
	mux.HandleFunc("GET /api/charts", s.handleSearch)
	mux.HandleFunc("GET /api/charts/{id}", s.handleChart)
	mux.HandleFunc("PATCH /api/charts/{id}", s.handleRenameChart)
	mux.HandleFunc("DELETE /api/charts/{id}", s.handleDeleteChart)
	mux.HandleFunc("POST /api/import/inspect", s.handleInspect)
	mux.HandleFunc("POST /api/import", s.handleImport)
	mux.HandleFunc("GET /charts/{id}/{file}", s.handleChartFile)

	// flights and pins
	mux.HandleFunc("GET /api/flights", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, s.lib.Flights()) })
	mux.HandleFunc("POST /api/flights", s.handleCreateFlight)
	mux.HandleFunc("PUT /api/flights/current", s.handleSetCurrent)
	mux.HandleFunc("PATCH /api/flights/{id}", s.handleRenameFlight)
	mux.HandleFunc("DELETE /api/flights/{id}", s.handleDeleteFlight)
	mux.HandleFunc("PUT /api/flights/{id}/pins/{chart}", s.handlePin)
	mux.HandleFunc("DELETE /api/flights/{id}/pins/{chart}", s.handleUnpin)

	s.registerSourceRoutes(mux)
	s.registerSetupRoutes(mux)

	// OpenStreetMap tiles (cached; the sim only talks to localhost over plain http)
	mux.HandleFunc("GET /tiles/{z}/{x}/{file}", s.tiles.handle)

	// viewer pages
	static := http.FileServer(http.FS(web.Sim()))
	mux.Handle("GET /", noCache(static))

	// manager UI (the app's own window; "/" opens it)
	mux.Handle("GET /manager/", noCache(http.StripPrefix("/manager/", http.FileServer(http.FS(web.Manager())))))
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/manager/", http.StatusFound) })

	return guard(mux, s.addr)
}

// ------------------------------------------------------------------ middleware

// guard rejects requests that do not look like they come from our own pages: a web page
// open in the user's browser must not be able to drive this local API (DNS rebinding, CSRF).
func guard(next http.Handler, addr string) http.Handler {
	_, port, _ := strings.Cut(addr, ":")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := strings.ToLower(r.Host)
		if h, p, ok := strings.Cut(host, ":"); ok {
			host = h
			if port != "" && p != port {
				http.Error(w, "forbidden host", http.StatusForbidden)
				return
			}
		}
		if host != "127.0.0.1" && host != "localhost" && host != "[::1]" {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" {
				if !strings.HasPrefix(o, "http://127.0.0.1") && !strings.HasPrefix(o, "http://localhost") {
					http.Error(w, "forbidden origin", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

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
	if f := q.Get("flight"); f != "" {
		for _, fl := range s.lib.Flights().Flights {
			if fl.ID == f || (f == "current" && fl.ID == s.lib.Flights().Current) {
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

func (s *Server) handleRenameChart(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.lib.Rename(r.PathValue("id"), body.Name); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	c, _ := s.lib.Get(r.PathValue("id"))
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
	if !store.ValidID(id) || (file != "chart.png" && file != "chart.json") {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.lib.ChartDir(id), file)
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
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
	if len(data) < 8 || string(data[:5]) != "%PDF-" {
		writeErr(w, 400, errors.New("not a PDF file"))
		return
	}

	c, err := s.importPDF(data, filename, importForm{
		Source:   r.FormValue("source"),
		Name:     r.FormValue("name"),
		ICAO:     r.FormValue("icao"),
		Type:     r.FormValue("type"),
		Title:    r.FormValue("title"),
		Validity: r.FormValue("validity"),
	})
	if err != nil {
		writeErr(w, importStatus(err), err)
		return
	}
	if token != "" {
		s.staged.drop(token)
	}
	writeJSON(w, 201, c)
}

func (s *Server) handleCreateFlight(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	_ = readJSON(r, &body)
	f, err := s.lib.CreateFlight(body.Name)
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
	writeJSON(w, 200, s.lib.Flights())
}

func (s *Server) handleRenameFlight(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	if err := s.lib.RenameFlight(r.PathValue("id"), body.Name); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, s.lib.Flights())
}

func (s *Server) handleDeleteFlight(w http.ResponseWriter, r *http.Request) {
	if err := s.lib.DeleteFlight(r.PathValue("id")); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, s.lib.Flights())
}

func (s *Server) handlePin(w http.ResponseWriter, r *http.Request) {
	if err := s.lib.Pin(r.PathValue("id"), r.PathValue("chart")); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, s.lib.Flights())
}

func (s *Server) handleUnpin(w http.ResponseWriter, r *http.Request) {
	if err := s.lib.Unpin(r.PathValue("id"), r.PathValue("chart")); err != nil {
		writeErr(w, statusFor(err), err)
		return
	}
	writeJSON(w, 200, s.lib.Flights())
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
