// Package importer turns a georeferenced chart PDF into the two files the viewer needs:
// chart.png (the page, rasterised) and chart.json (how latitude/longitude maps to its pixels).
package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/geo"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/pdfgeo"
)

// MaxGroundErrorMeters is the largest corner-fit residual we accept. GPTS values are often
// written with 4 decimals (~11 m), so a few metres is the precision floor of the chart itself.
const MaxGroundErrorMeters = 15.0

// Renderer owns the PDFium (WebAssembly) pool. Create once, reuse for every import.
type Renderer struct {
	pool     pdfium.Pool
	instance pdfium.Pdfium
}

// NewRenderer starts PDFium. The first call compiles the WASM module (~3 s).
func NewRenderer() (*Renderer, error) {
	pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		return nil, err
	}
	inst, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return &Renderer{pool: pool, instance: inst}, nil
}

// Close releases PDFium.
func (r *Renderer) Close() {
	if r.instance != nil {
		r.instance.Close()
	}
	if r.pool != nil {
		r.pool.Close()
	}
}

// Options for one import.
type Options struct {
	ID     string // library id
	Name   string // display name; default: "ICAO Â· TYPE Â· Title"
	Source string // e.g. "decea-br" or "manual"
	DPI    int    // 0 = automatic
	// LongEdgePx is the size the longer side of the image should have (0 = 3200). It only
	// matters when DPI is automatic.
	LongEdgePx int
	ICAO       string // "SBSP" (optional)
	Type       string // "IAC", "ADC", ... (optional)
	Title      string // "ILS X RWY 17R"; default: the PDF's viewport name when there is no ICAO
	Validity   string // "2026-08-06" (optional)
	Original   string // original file name, kept to recognise a chart that is already imported
	// ViewOnly lets a PDF without georeferencing in: it is shown as a plain picture, with no
	// aircraft position (an airport diagram, for example). Without it such a PDF is an error.
	ViewOnly bool
	Package  string // archive the PDF came from (provenance), if any
}

// Meta is chart.json. Field names match what the existing viewer pages read.
type Meta struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Source       string `json:"source"`
	ICAO         string `json:"icao"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	Validity     string `json:"validity"`
	AddedAt      string `json:"addedAt"`
	OriginalFile string `json:"originalFile,omitempty"`
	Package      string `json:"package,omitempty"`
	Image        struct {
		File   string `json:"file"`
		Width  int    `json:"width"`
		Height int    `json:"height"`
		DPI    int    `json:"dpi"`
	} `json:"image"`
	Projection map[string]any `json:"projection"`
	ToPixel    struct {
		A float64 `json:"a"`
		B float64 `json:"b"`
		C float64 `json:"c"`
		D float64 `json:"d"`
		E float64 `json:"e"`
		F float64 `json:"f"`
	} `json:"toPixel"`
	MapFramePx struct {
		Left   float64 `json:"left"`
		Top    float64 `json:"top"`
		Right  float64 `json:"right"`
		Bottom float64 `json:"bottom"`
	} `json:"mapFramePx"`
	GeoCorners []Corner `json:"geoCorners"`
	// Effective pixels-per-point actually produced by the renderer (per axis). PDFium stretches
	// the page to an integer pixel size, so this is NOT exactly dpi/72.
	Scale struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"scale"`
	Georef struct {
		Viewport       string  `json:"viewport"`
		ErrorMeters    float64 `json:"errorMeters"`
		MetersPerPixel float64 `json:"metersPerPixel"`
	} `json:"georef"`
	Generator string `json:"generator"`
	// Unreferenced is true for a chart with no georeferencing: a picture to look at, with no
	// aircraft on it and no overlay on the map.
	Unreferenced bool `json:"unreferenced,omitempty"`
	// Tiles describes the pyramid of tiles of a picture too big to show whole (see pyramid.go).
	Tiles *Tiles `json:"tiles,omitempty"`
}

// Corner is a control point: geographic position and where it lands in the image.
type Corner struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
	PX  float64 `json:"px"`
	PY  float64 `json:"py"`
}

// Result of an import.
type Result struct {
	Meta Meta
	PNG  []byte
	JSON []byte
	// Files are extra files of the chart by relative path (the tiles of the pyramid).
	Files map[string][]byte
}

// ErrNotGeoreferenced is returned for PDFs without a usable /VP georeference.
var ErrNotGeoreferenced = fmt.Errorf("this PDF has no georeferencing (/VP)")

// Import converts one PDF. It never guesses: unsupported projections or poor fits are errors.
func (r *Renderer) Import(pdf []byte, opt Options) (*Result, error) {
	pg, err := pdfgeo.Read(pdf)
	if err != nil {
		return nil, err
	}
	unref := len(pg.Viewports) == 0
	if unref && !opt.ViewOnly {
		return nil, ErrNotGeoreferenced
	}
	if pg.Rotate != 0 && !unref {
		return nil, fmt.Errorf("rotated pages (/Rotate %d) are not supported yet", pg.Rotate)
	}
	var (
		vp   pdfgeo.Viewport
		proj *geo.Projection
		aff  geo.Affine
		xy   [][2]float64
		uv   [][2]float64
		n    int
	)
	if !unref {
		vp = pickViewport(pg.Viewports)
		proj, err = geo.FromWKT(vp.WKT)
		if err != nil {
			return nil, err
		}

		// --- control points: lat/lon -> projected metres, and the matching normalised viewport coords
		n = len(vp.GPTS) / 2
		xy = make([][2]float64, n)
		uv = make([][2]float64, n)
		for i := 0; i < n; i++ {
			x, y := proj.Forward(vp.GPTS[2*i], vp.GPTS[2*i+1])
			xy[i] = [2]float64{x, y}
			uv[i] = [2]float64{vp.LPTS[2*i], vp.LPTS[2*i+1]}
		}
		aff, err = geo.FitAffine(xy, uv)
		if err != nil {
			return nil, err
		}
	}

	// --- render
	dpi := opt.DPI
	boxW, boxH := pg.Box[2]-pg.Box[0], pg.Box[3]-pg.Box[1]
	if dpi <= 0 {
		long := math.Max(boxW, boxH)
		target := float64(opt.LongEdgePx)
		if target <= 0 {
			target = 3200
		}
		dpi = int(math.Max(150, math.Min(600, math.Round(target/long*72))))
	}
	doc, err := r.instance.OpenDocument(&requests.OpenDocument{File: &pdf})
	if err != nil {
		return nil, fmt.Errorf("open: %w", err)
	}
	defer r.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	rendered, err := r.instance.RenderPageInDPI(&requests.RenderPageInDPI{
		DPI:  dpi,
		Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 0}},
	})
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	defer rendered.Cleanup()

	W, H := rendered.Result.Width, rendered.Result.Height
	// Effective scale: pixels actually produced per PDF point, per axis (of the full picture).
	sx, sy := float64(W)/boxW, float64(H)/boxH

	// A page too big for the overview size (a big visual chart is drawn at 150 dpi at least) is also
	// cut into a pyramid of tiles (see pyramid.go); chart.png is then the overview, exactly one
	// 2^levels-th of the picture, and every coordinate below is in the overview's pixels.
	long := opt.LongEdgePx
	if long <= 0 {
		long = 3200
	}
	var overview image.Image = rendered.Result.Image
	var files map[string][]byte
	var tiles *Tiles
	if levels := pyramidLevels(W, H, long); levels > 0 {
		ov, tf, info, err := buildPyramid(rendered.Result.Image, levels)
		if err != nil {
			return nil, err
		}
		overview, files, tiles = ov, tf, &info
		f := 1 / float64(int(1)<<levels)
		sx, sy = sx*f, sy*f
		dpi = int(math.Round(float64(dpi) * f))
	}
	ob := overview.Bounds()
	W, H = ob.Dx(), ob.Dy()

	var pngBuf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression} // BestCompression doubled the import time for ~2% smaller files
	if err := enc.Encode(&pngBuf, overview); err != nil {
		return nil, err
	}

	// --- compose: (E,N) -> (u,v) -> PDF points -> image pixels
	var m Meta
	var errM, mPerPx float64
	bx0, by0, bx1, by1 := vp.BBox[0], vp.BBox[1], vp.BBox[2], vp.BBox[3]
	if !unref {
		bw, bh := bx1-bx0, by1-by0
		m.ToPixel.A = bw * sx * aff.A
		m.ToPixel.B = bw * sx * aff.B
		m.ToPixel.C = (bx0-pg.Box[0])*sx + bw*sx*aff.C
		m.ToPixel.D = -bh * sy * aff.D
		m.ToPixel.E = -bh * sy * aff.E
		m.ToPixel.F = (pg.Box[3]-by0)*sy - bh*sy*aff.F

		toPx := func(e, n float64) (float64, float64) {
			return m.ToPixel.A*e + m.ToPixel.B*n + m.ToPixel.C, m.ToPixel.D*e + m.ToPixel.E*n + m.ToPixel.F
		}

		// Residual of the 4 control points, judged on the ground.
		mPerPx = 1 / math.Hypot(m.ToPixel.A, m.ToPixel.B)
		var worst float64
		for i := 0; i < n; i++ {
			px, py := toPx(xy[i][0], xy[i][1])
			wantX := (bx0 + uv[i][0]*bw - pg.Box[0]) * sx
			wantY := (pg.Box[3] - (by0 + uv[i][1]*bh)) * sy
			worst = math.Max(worst, math.Hypot(px-wantX, py-wantY))
			m.GeoCorners = append(m.GeoCorners, Corner{
				Lat: vp.GPTS[2*i], Lon: vp.GPTS[2*i+1],
				PX: round(px, 2), PY: round(py, 2),
			})
		}
		errM = worst * mPerPx
		if errM > MaxGroundErrorMeters {
			return nil, fmt.Errorf("georeference does not fit an affine map (%.1f m): refusing to guess", errM)
		}
	}

	title := opt.Title
	if title == "" && opt.ICAO == "" {
		title = strings.TrimSpace(vp.Name) // visual charts have no ICAO: the viewport name is the title
	}
	m.ID = opt.ID
	m.ICAO, m.Type, m.Title, m.Validity = opt.ICAO, opt.Type, title, opt.Validity
	m.Name = firstNonEmpty(opt.Name, naming.Display(opt.ICAO, opt.Type, title))
	m.Source = opt.Source
	m.AddedAt = time.Now().UTC().Format(time.RFC3339)
	m.OriginalFile = opt.Original
	m.Package = opt.Package
	m.Image.File, m.Image.Width, m.Image.Height, m.Image.DPI = "chart.png", W, H, dpi
	if unref {
		// the whole picture is the "map": there is nothing to place on it
		m.Unreferenced = true
		m.MapFramePx.Right, m.MapFramePx.Bottom = float64(W), float64(H)
		title = firstNonEmpty(title, strings.TrimSuffix(opt.Original, filepath.Ext(opt.Original)))
		m.Title = title
		m.Name = firstNonEmpty(opt.Name, naming.Display(opt.ICAO, opt.Type, title))
	} else {
		m.Projection = proj.JSON()
		m.MapFramePx.Left = (bx0 - pg.Box[0]) * sx
		m.MapFramePx.Right = (bx1 - pg.Box[0]) * sx
		m.MapFramePx.Top = (pg.Box[3] - by1) * sy
		m.MapFramePx.Bottom = (pg.Box[3] - by0) * sy
	}
	m.Scale.X, m.Scale.Y = sx, sy
	if !unref {
		m.Georef.Viewport = vp.Name
		m.Georef.ErrorMeters = round(errM, 3)
		m.Georef.MetersPerPixel = round(mPerPx, 3)
	}
	m.Generator = "open-charts/importer (pdfium-wasm, no cgo)"
	m.Tiles = tiles

	js, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Meta: m, PNG: pngBuf.Bytes(), JSON: js, Files: files}, nil
}

// pickViewport prefers a named chart viewport over a generic "BaseMap", then the largest.
func pickViewport(vps []pdfgeo.Viewport) pdfgeo.Viewport {
	best := -1
	var bestArea float64
	for i, v := range vps {
		if strings.HasPrefix(strings.ToLower(v.Name), "basemap") && len(vps) > 1 {
			continue
		}
		a := math.Abs((v.BBox[2] - v.BBox[0]) * (v.BBox[3] - v.BBox[1]))
		if best < 0 || a > bestArea {
			best, bestArea = i, a
		}
	}
	if best < 0 {
		best = 0
	}
	return vps[best]
}

// StemID builds a library id from a file path.
func StemID(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}
