package server

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"localmovingmap/internal/importer"
	"localmovingmap/internal/naming"
	"localmovingmap/internal/pdfgeo"
	"localmovingmap/internal/sources"
	"localmovingmap/internal/store"
)

// importForm carries the optional fields a user (or a source) can set for an import.
type importForm struct {
	Source, Name, ICAO, Type, Title, Validity string
	Package                                   string // archive the PDF was taken from
}

// errAlready is returned when the same original file is already in the library.
type errAlready struct{ ID, Name string }

func (e errAlready) Error() string { return fmt.Sprintf("already in the library as %q", e.Name) }

// importPDF converts one PDF and adds it to the library. It is the single path used by uploads,
// package imports and the inbox. Imports run one at a time (one PDFium instance).
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
	info, by := sources.Recognize(source, stem)
	if by == "" {
		by = source
	}
	if it, ok := sources.MatchCatalog(by, base); ok { // exactly as the provider lists it
		info.Type, info.Title, info.Validity = it.Type, it.Name, it.Effective
	}
	opt := importer.Options{
		Source:   source,
		Name:     strings.TrimSpace(f.Name),
		ICAO:     strings.ToUpper(pick(f.ICAO, info.ICAO)),
		Type:     strings.ToUpper(pick(f.Type, info.Type)),
		Title:    pick(f.Title, s.autoTitle(data, info)),
		Validity: pick(f.Validity, info.Validity),
		Original: base,
		Package:  f.Package,
	}
	opt.ID = s.lib.UniqueID(store.Slug(source + " " + stem))

	var res *importer.Result
	err := s.withRenderer(func(r *importer.Renderer) (e error) {
		res, e = r.Import(data, opt)
		return e
	})
	if err != nil {
		return nil, err
	}
	c, err := s.lib.Add(opt.ID, res.PNG, res.JSON, data)
	if err != nil {
		return nil, err
	}
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

// autoTitle: ICAO charts use the title from the name; charts without ICAO use the title the PDF
// itself carries, then the tidied file name.
func (s *Server) autoTitle(data []byte, info naming.Info) string {
	if info.ICAO != "" {
		return info.Title
	}
	if pg, err := pdfgeo.Read(data); err == nil && len(pg.Viewports) > 0 {
		if n := strings.TrimSpace(pg.Viewports[0].Name); n != "" {
			return n
		}
	}
	return info.Title
}
