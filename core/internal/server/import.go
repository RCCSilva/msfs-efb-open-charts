package server

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/pdfgeo"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sources"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// importForm carries the optional fields a user (or a source) can set for an import.
type importForm struct {
	Source, Name, ICAO, Type, Title, Validity string
	Package                                   string // archive the PDF was taken from
	// Stage, when set, is told what the import is doing and how far along it is.
	Stage func(stage string, percent int)
}

func (f importForm) stage(name string, percent int) {
	if f.Stage != nil {
		f.Stage(name, percent)
	}
}

// errAlready is returned when the same original file is already in the library.
type errAlready struct{ ID, Name string }

func (e errAlready) Error() string { return fmt.Sprintf("already in the library as %q", e.Name) }

// importPDF converts one PDF and adds it to the library. It is the single path used by uploads,
// package imports. Imports run one at a time (one PDFium instance).
func (s *Server) importPDF(data []byte, filename string, f importForm) (*store.Chart, error) {
	if !s.canImport() {
		return nil, errors.New("importer unavailable")
	}
	if len(data) < 8 || string(data[:5]) != "%PDF-" {
		return nil, errNotPDF
	}
	base := filepath.Base(filename)
	if c, ok := s.lib.FindByOriginal(base); ok {
		return c, errAlready{ID: c.ID, Name: c.Name}
	}

	stem := strings.TrimSuffix(base, filepath.Ext(base))
	pick := func(v, fallback string) string {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
		return fallback
	}
	source := pick(f.Source, "manual")
	id := sources.Identify(source, stem, base, pdfViewportName(data))
	opt := importer.Options{
		Source:   source,
		Name:     strings.TrimSpace(f.Name),
		ICAO:     strings.ToUpper(pick(f.ICAO, id.ICAO)),
		Type:     strings.ToUpper(pick(f.Type, id.Type)),
		Title:    pick(f.Title, id.Title),
		Validity: pick(f.Validity, id.Validity),
		Original: base,
		Package:  f.Package,
		// Detail level chosen in the manager (settings), for charts imported from now on.
		LongEdgePx: settings.LongEdgePx(s.settingsSnapshot().Detail),
	}
	opt.ID = s.lib.UniqueID(store.Slug(source + " " + stem))

	var res *importer.Result
	var began time.Time
	f.stage("waiting", 5)
	err := s.withRenderer(func() { f.stage("engine", 15) }, func(r *importer.Renderer) (e error) {
		f.stage("render", 40)
		began = time.Now()
		res, e = r.Import(data, opt)
		return e
	})
	if err != nil {
		return nil, err
	}
	f.stage("save", 85)
	c, err := s.lib.Add(opt.ID, res.PNG, res.JSON, data)
	if err != nil {
		return nil, err
	}
	s.impTime.record(time.Since(began))
	log.Printf("imported %s (%s) %dx%d fit %.2f m", c.ID, c.Name, c.Width, c.Height, c.ErrorMeters)
	return c, nil
}

var errNotPDF = errors.New("not a PDF file")

// importStatus maps an import error to an HTTP status.
func importStatus(err error) int {
	var already errAlready
	switch {
	case errors.As(err, &already):
		return 409
	case errors.Is(err, errNotPDF):
		return 400
	case errors.Is(err, importer.ErrNotGeoreferenced):
		return 422
	default:
		return 422
	}
}

// itemStatus classifies an import error for a batch report.
func itemStatus(err error) (status, msg string) {
	var already errAlready
	switch {
	case errors.As(err, &already):
		return "skipped", err.Error()
	case errors.Is(err, importer.ErrNotGeoreferenced):
		return "skipped", "no georeferencing in this PDF"
	default:
		return "error", err.Error()
	}
}

// pdfViewportName returns the name of the first georeferenced viewport of a PDF, or "".
func pdfViewportName(data []byte) string {
	if pg, err := pdfgeo.Read(data); err == nil && len(pg.Viewports) > 0 {
		return strings.TrimSpace(pg.Viewports[0].Name)
	}
	return ""
}
