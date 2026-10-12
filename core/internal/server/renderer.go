package server

import (
	"errors"
	"runtime/debug"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
)

// The PDF renderer (PDFium compiled to WebAssembly) takes ~300 MB of memory once loaded and
// grows while rendering. It is only needed while importing, so it is created on first use and
// released after a quiet period: the resident program stays small (~20 MB).

// RendererFactory creates the PDF renderer. A nil factory disables importing.
type RendererFactory func() (*importer.Renderer, error)

// rendererIdle is how long the renderer is kept after the last import.
const rendererIdle = 60 * time.Second

func (s *Server) canImport() bool { return s.newRenderer != nil }

// withRenderer runs fn with the renderer, creating it if needed. Imports run one at a time
// (one PDFium instance).
// onLoad (may be nil) is called when the renderer has to be created first.
func (s *Server) withRenderer(onLoad func(), fn func(r *importer.Renderer) error) error {
	if s.newRenderer == nil {
		return errors.New("importer unavailable")
	}
	s.importMu.Lock()
	defer s.importMu.Unlock()
	if s.rend == nil {
		if onLoad != nil {
			onLoad()
		}
		r, err := s.newRenderer()
		if err != nil {
			return err
		}
		s.rend = r
	}
	if s.rendIdle != nil {
		s.rendIdle.Stop()
	}
	err := fn(s.rend)
	s.rendIdle = time.AfterFunc(rendererIdle, s.releaseRenderer)
	return err
}

func (s *Server) releaseRenderer() {
	s.importMu.Lock()
	defer s.importMu.Unlock()
	if s.rend != nil {
		s.rend.Close()
		s.rend = nil
		debug.FreeOSMemory()
	}
}

// Close releases the renderer; call it on shutdown.
func (s *Server) Close() {
	s.importMu.Lock()
	if s.rendIdle != nil {
		s.rendIdle.Stop()
	}
	s.importMu.Unlock()
	s.releaseRenderer()
}
