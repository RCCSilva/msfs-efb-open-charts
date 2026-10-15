package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sources"
)

// userPatterns are the user's file-name patterns, compiled, in order. A pattern that no longer
// compiles is skipped (it cannot have been saved, but the file may have been edited by hand).
func (s *Server) userPatterns() []*naming.Pattern {
	var out []*naming.Pattern
	for _, text := range s.settingsSnapshot().NamePatterns {
		if p, err := naming.ParsePattern(text); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// identify is sources.Identify, with the user's own file-name patterns tried first for the "My
// PDFs" source: a provider's files are recognised by the provider's rules, the user's by theirs.
func (s *Server) identify(source, stem, file, viewport string) sources.Identity {
	if source == "manual" {
		for _, p := range s.userPatterns() {
			if info, ok := p.Match(stem); ok {
				return sources.IdentifyWith(source, stem, file, viewport, &info)
			}
		}
	}
	return sources.Identify(source, stem, file, viewport)
}

// identifyContent completes an identity from the text of the first page of the PDF, when the name
// of the file recognised nothing (the charts of the UK AIP are called 475382.pdf, but each carries
// its reference "AD 2-EGLL-6-1" in the footer). It only opens the PDF's text then.
func (s *Server) identifyContent(id sources.Identity, source string, data []byte) sources.Identity {
	if id.Recognizer != "" || id.ICAO != "" || !sources.NeedsContent() || !s.canImport() {
		return id
	}
	var text string
	err := s.withRenderer(nil, func(r *importer.Renderer) error {
		t, err := r.PageText(data)
		text = t
		return err
	})
	if err != nil || text == "" {
		return id
	}
	info, by, ok := sources.RecognizeContent(source, text)
	if !ok {
		return id
	}
	id.ICAO, id.Type, id.Title, id.Recognizer = info.ICAO, info.Type, info.Title, by
	id.Confident = info.ICAO != "" && info.Type != ""
	return id
}

func (s *Server) registerNamingRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/naming/patterns", s.handleGetPatterns)
	mux.HandleFunc("PUT /api/naming/patterns", s.handlePutPatterns)
	mux.HandleFunc("POST /api/naming/test", s.handleTestPattern)
}

func (s *Server) handleGetPatterns(w http.ResponseWriter, r *http.Request) {
	list := s.settingsSnapshot().NamePatterns
	if list == nil {
		list = []string{}
	}
	writeJSON(w, 200, map[string]any{"patterns": list, "max": settings.MaxNamePatterns, "placeholders": naming.PlaceholderNames})
}

// handlePutPatterns replaces the list. Every pattern is checked first: nothing is saved if one is
// wrong, and the answer says which one and why.
func (s *Server) handlePutPatterns(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Patterns []string `json:"patterns"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	var clean []string
	for _, text := range body.Patterns {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		if _, err := naming.ParsePattern(text); err != nil {
			writeErr(w, 400, errors.New(text+": "+err.Error()))
			return
		}
		clean = append(clean, text)
	}
	if len(clean) > settings.MaxNamePatterns {
		writeErr(w, 400, errors.New("too many patterns"))
		return
	}
	s.net.mu.Lock()
	s.net.cfg.NamePatterns = clean
	cfg, dir := s.net.cfg, s.net.dataDir
	s.net.mu.Unlock()
	if err := cfg.Save(dir); err != nil {
		writeErr(w, 500, err)
		return
	}
	s.handleGetPatterns(w, r)
}

// handleTestPattern tells what a pattern reads from a sample file name, so the user can check it
// before importing a whole folder.
func (s *Server) handleTestPattern(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Pattern string `json:"pattern"`
		Sample  string `json:"sample"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	p, err := naming.ParsePattern(body.Pattern)
	if err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	info, ok := p.Match(body.Sample)
	if !ok {
		writeJSON(w, 200, map[string]any{"ok": false, "error": "no match"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "icao": info.ICAO, "type": info.Type, "title": info.Title, "validity": info.Validity})
}
