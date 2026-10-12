// Package pdfgeo reads the georeferencing of a geospatial PDF (ISO 32000 /VP viewports with
// a /Measure of subtype /GEO) using pdfcpu. It does not render anything.
package pdfgeo

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// Viewport is one georeferenced region of the page.
type Viewport struct {
	Name string
	BBox [4]float64 // x0, y0, x1, y1 in default user space (points, origin bottom-left)
	GPTS []float64  // lat, lon pairs
	LPTS []float64  // x, y pairs, normalised (0..1) inside BBox
	WKT  string     // /GCS /WKT
}

// Page is what the importer needs from the first page.
type Page struct {
	Box       [4]float64 // visible box (CropBox if present, else MediaBox): x0, y0, x1, y1
	Rotate    int
	Viewports []Viewport
}

// Read parses page 1.
func Read(data []byte) (*Page, error) {
	c := context.Background()
	ctx, err := api.ReadContext(c, bytes.NewReader(data), model.NewDefaultConfiguration())
	if err != nil {
		return nil, fmt.Errorf("read pdf: %w", err)
	}
	if err := ctx.EnsurePageCount(); err != nil {
		return nil, fmt.Errorf("page tree: %w", err)
	}
	d, _, inh, err := ctx.PageDict(c, 1, false)
	if err != nil {
		return nil, fmt.Errorf("page 1: %w", err)
	}

	pg := &Page{}
	box := inh.MediaBox
	if inh.CropBox != nil {
		box = inh.CropBox
	}
	if box == nil {
		return nil, fmt.Errorf("page has no MediaBox")
	}
	pg.Box = [4]float64{box.LL.X, box.LL.Y, box.UR.X, box.UR.Y}
	pg.Rotate = inh.Rotate

	vpObj, found := d.Find("VP")
	if !found {
		return pg, nil
	}
	arr, err := ctx.DereferenceArray(vpObj)
	if err != nil {
		return nil, fmt.Errorf("/VP: %w", err)
	}
	for _, o := range arr {
		vd, err := ctx.DereferenceDict(o)
		if err != nil || vd == nil {
			continue
		}
		v, ok := readViewport(ctx, vd)
		if ok {
			pg.Viewports = append(pg.Viewports, v)
		}
	}
	return pg, nil
}

func numbers(ctx *model.Context, o types.Object) ([]float64, error) {
	a, err := ctx.DereferenceArray(o)
	if err != nil {
		return nil, err
	}
	out := make([]float64, 0, len(a))
	for _, e := range a {
		f, err := ctx.DereferenceNumber(e)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func readViewport(ctx *model.Context, vd types.Dict) (Viewport, bool) {
	var v Viewport
	bo, ok := vd.Find("BBox")
	if !ok {
		return v, false
	}
	bb, err := numbers(ctx, bo)
	if err != nil || len(bb) != 4 {
		return v, false
	}
	copy(v.BBox[:], bb)

	mo, ok := vd.Find("Measure")
	if !ok {
		return v, false
	}
	md, err := ctx.DereferenceDict(mo)
	if err != nil || md == nil {
		return v, false
	}
	if g, ok := md.Find("GPTS"); ok {
		v.GPTS, _ = numbers(ctx, g)
	}
	if l, ok := md.Find("LPTS"); ok {
		v.LPTS, _ = numbers(ctx, l)
	}
	if len(v.GPTS) < 6 || len(v.GPTS)%2 != 0 || len(v.LPTS) != len(v.GPTS) {
		return v, false
	}
	if gcs, ok := md.Find("GCS"); ok {
		if gd, err := ctx.DereferenceDict(gcs); err == nil && gd != nil {
			if w, ok := gd.Find("WKT"); ok {
				s, _ := ctx.DereferenceStringOrHexLiteral(w, model.V10, nil)
				v.WKT = strings.ReplaceAll(s, `\"`, `"`)
			}
		}
	}
	if no, ok := vd.Find("Name"); ok {
		v.Name, _ = ctx.DereferenceStringOrHexLiteral(no, model.V10, nil)
	}
	return v, true
}
