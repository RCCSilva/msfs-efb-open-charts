package importer

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"

	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

// A big chart (a WAC, an en-route chart of the whole country) is drawn at a resolution that makes
// it sharp when the map is zoomed in, which is far beyond what fits in memory as one picture: the
// full page of an en-route chart at 35 metres per pixel is almost two billion pixels. So no page is
// ever rendered whole here. Every tile of every level is drawn straight from the PDF (a region of
// the page at the level's scale), one at a time, and encoded at once; the vector drawing is
// anti-aliased by PDFium at each scale, nothing is shrunk from a bigger picture.

// MaxDetailPixels is the most pixels the finest level of a chart may have.
const MaxDetailPixels = 2_500_000_000

// MaxDetailLevels is how many halvings the pyramid may have.
const MaxDetailLevels = 6

// detailPlan sizes the pyramid for a page of boxW x boxH points drawn at s0 pixels per point at the
// finest level: how many halvings m bring it to about overview pixels on the long side, and the
// size of the finest level, padded to a multiple of 2^m so that every level is an exact half of the
// one before.
func detailPlan(boxW, boxH, s0 float64, overview int) (m, w0, h0 int) {
	w, h := int(math.Ceil(boxW*s0)), int(math.Ceil(boxH*s0))
	m = min(pyramidLevels(w, h, overview), MaxDetailLevels)
	p := 1 << m
	return m, (w + p - 1) / p * p, (h + p - 1) / p * p
}

// detailScale limits s0 (pixels per point) so that the finest level stays within MaxDetailPixels.
func detailScale(boxW, boxH, s0 float64) float64 {
	if px := boxW * s0 * boxH * s0; px > MaxDetailPixels {
		s0 *= math.Sqrt(MaxDetailPixels / px)
	}
	return s0
}

// renderDetail draws page 0 of the document as a pyramid with the finest level at s0 pixels per
// point. It returns the overview (the level that becomes chart.png), the tile files and the
// description of the pyramid.
func (r *Renderer) renderDetail(doc references.FPDF_DOCUMENT, boxW, boxH, s0 float64, overviewPx int) (*image.RGBA, map[string][]byte, Tiles, error) {
	loaded, err := r.instance.FPDF_LoadPage(&requests.FPDF_LoadPage{Document: doc, Index: 0})
	if err != nil {
		return nil, nil, Tiles{}, fmt.Errorf("load page: %w", err)
	}
	defer r.instance.FPDF_ClosePage(&requests.FPDF_ClosePage{Page: loaded.Page})
	page := requests.Page{ByReference: &loaded.Page}

	m, w0, h0 := detailPlan(boxW, boxH, s0, overviewPx)
	files := map[string][]byte{}
	info := Tiles{Size: TileSize, Format: "png", MaxZoom: m, Width: w0, Height: h0, Path: "tiles/{z}/{x}_{y}.png"}
	ow, oh := w0>>m, h0>>m
	overview := image.NewRGBA(image.Rect(0, 0, ow, oh))

	for k := 0; k <= m; k++ {
		scale := s0 / float64(int(1)<<k) // pixels per point at this level
		lw, lh := w0>>k, h0>>k
		span := float64(TileSize) / scale // the side of a tile, in points
		for ty := 0; ty*TileSize < lh; ty++ {
			for tx := 0; tx*TileSize < lw; tx++ {
				res, err := r.instance.RenderPageInPixels(&requests.RenderPageInPixels{
					Page: page, Width: TileSize, Height: TileSize,
					Crop: &requests.RenderPageCrop{X: float64(tx) * span, Y: float64(ty) * span, Width: span, Height: span},
				})
				if err != nil {
					return nil, nil, Tiles{}, fmt.Errorf("render tile %d/%d_%d: %w", m-k, tx, ty, err)
				}
				tile := whiteTile(res.Result.Image)
				res.Cleanup()
				data, err := encodeTile(tile, TileSize, TileSize)
				if err != nil {
					return nil, nil, Tiles{}, err
				}
				files[fmt.Sprintf("tiles/%d/%d_%d.png", m-k, tx, ty)] = data
				if k == m {
					copyInto(overview, tile, tx*TileSize, ty*TileSize)
				}
			}
		}
	}

	// the levels past the overview are made from it, down to a single tile
	total := m
	for (w0>>total) > TileSize || (h0>>total) > TileSize {
		total++
	}
	info.MinZoom = m - total
	prev := overview
	for k := m + 1; k <= total; k++ {
		lw, lh := max(1, w0>>k), max(1, h0>>k)
		pb := prev.Bounds()
		level := areaResampleV(prev, pb.Dx(), pb.Dy(), lw, lh)
		prev = level
		for ty := 0; ty*TileSize < lh; ty++ {
			for tx := 0; tx*TileSize < lw; tx++ {
				x0, y0 := tx*TileSize, ty*TileSize
				t := image.NewRGBA(image.Rect(0, 0, TileSize, TileSize))
				for i := range t.Pix {
					t.Pix[i] = 255
				}
				for y := 0; y < min(TileSize, lh-y0); y++ {
					copy(t.Pix[y*t.Stride:y*t.Stride+min(TileSize, lw-x0)*4], level.Pix[(y0+y)*level.Stride+x0*4:])
				}
				data, err := encodeTile(t, TileSize, TileSize)
				if err != nil {
					return nil, nil, Tiles{}, err
				}
				files[fmt.Sprintf("tiles/%d/%d_%d.png", m-k, tx, ty)] = data
			}
		}
	}
	return overview, files, info, nil
}

// whiteTile copies a rendered region into a whole TileSize x TileSize tile, white where the region
// came out smaller, with every pixel opaque. The renderer's buffer is only valid until Cleanup, so
// the copy is also what outlives it.
func whiteTile(src *image.RGBA) *image.RGBA {
	t := image.NewRGBA(image.Rect(0, 0, TileSize, TileSize))
	for i := range t.Pix {
		t.Pix[i] = 255
	}
	b := src.Bounds()
	for y := 0; y < min(TileSize, b.Dy()); y++ {
		copy(t.Pix[y*t.Stride:y*t.Stride+min(TileSize, b.Dx())*4], src.Pix[y*src.Stride:])
	}
	for i := 3; i < len(t.Pix); i += 4 {
		t.Pix[i] = 255
	}
	return t
}

// copyInto puts a tile in the picture at (x0, y0), cut at the picture's edge.
func copyInto(dst *image.RGBA, tile *image.RGBA, x0, y0 int) {
	db := dst.Bounds()
	w := min(TileSize, db.Dx()-x0)
	for y := 0; y < min(TileSize, db.Dy()-y0); y++ {
		copy(dst.Pix[(y0+y)*dst.Stride+x0*4:(y0+y)*dst.Stride+(x0+w)*4], tile.Pix[y*tile.Stride:y*tile.Stride+w*4])
	}
}

// encodeTile encodes a tile as PNG.
func encodeTile(t *image.RGBA, w, h int) ([]byte, error) {
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, t.SubImage(image.Rect(0, 0, w, h))); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
