package importer

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/geo"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/pdfgeo"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

// Proof of concept: the paths of a chart as vectors in geographic space, to be drawn by the browser
// at any zoom instead of as pictures. The file has a small JSON header and then the arrays, all
// little endian. Points are in "world pixels" of map zoom WorldZoom (Web Mercator, 256 px tiles)
// relative to the origin in the header; line widths are in the same unit.

// WorldZoom is the map zoom of the coordinates in the vector file.
const WorldZoom = 8

// VectorHeader describes the vector file that follows it.
type VectorHeader struct {
	Version int        `json:"version"`
	Zoom    int        `json:"zoom"`
	Origin  [2]float64 `json:"origin"` // world pixels of (0,0)
	Paths   int        `json:"paths"`
	Ops     int        `json:"ops"`
	Points  int        `json:"points"`
	// Layout after the header: path styles (Paths x 13 bytes: fill RGBA, stroke RGBA, flags, width
	// float32), ops per path (Paths x uint32), ops (Ops x uint8: 0 move, 1 line, 2 curve of 3 points,
	// 3 close), points (Points x 2 float32).
	Bounds [4]float64 `json:"bounds"` // west, south, east, north
	// Frame is the map frame of the chart (its four corners, in the same coordinates as the points):
	// the page also holds legends and margins that are not on the map and must be cut off.
	Frame [4][2]float64 `json:"frame"`
}

// VectorStats is the size of what was extracted.
type VectorStats struct {
	Objects  map[string]int
	Paths    int
	Segments int
	Skipped  int
	MaxDepth int
}

const (
	flagStroke  = 1
	flagFill    = 2
	flagEvenOdd = 4
)

// ExtractVectors reads the filled and stroked paths of page 0 of a georeferenced chart and returns
// them as a vector file.
func (r *Renderer) ExtractVectors(pdf []byte) ([]byte, VectorStats, error) {
	st := VectorStats{Objects: map[string]int{}}
	pg, err := pdfgeo.Read(pdf)
	if err != nil {
		return nil, st, err
	}
	if len(pg.Viewports) == 0 {
		return nil, st, ErrNotGeoreferenced
	}
	view, err := newPageView(pg.Box, pg.Rotate)
	if err != nil {
		return nil, st, err
	}
	vp := pickViewport(pg.Viewports)
	proj, err := geo.FromWKT(vp.WKT)
	if err != nil {
		return nil, st, err
	}
	n := len(vp.GPTS) / 2
	xy := make([][2]float64, n)
	uv := make([][2]float64, n)
	for i := 0; i < n; i++ {
		x, y := proj.Forward(vp.GPTS[2*i], vp.GPTS[2*i+1])
		xy[i] = [2]float64{x, y}
		uv[i] = [2]float64{vp.LPTS[2*i], vp.LPTS[2*i+1]}
	}
	aff, err := geo.FitAffine(xy, uv)
	if err != nil {
		return nil, st, err
	}
	bw, bh := vp.BBox[2]-vp.BBox[0], vp.BBox[3]-vp.BBox[1]
	t := view.compose(aff, [4]float64{vp.BBox[0], vp.BBox[1], bw, bh}, 1, 1) // (E,N) -> shown page points
	det := t.A*t.E - t.B*t.D
	if det == 0 {
		return nil, st, fmt.Errorf("degenerate georeference")
	}
	world := float64(256 * (int(1) << WorldZoom))
	// shown page point (u,v) -> world pixels
	toWorld := func(u, v float64) (float64, float64) {
		du, dv := u-t.C, v-t.F
		e := (t.E*du - t.B*dv) / det
		nn := (-t.D*du + t.A*dv) / det
		lat, lon, ok := proj.Inverse(e, nn)
		if !ok {
			return math.NaN(), math.NaN()
		}
		x := (lon + 180) / 360 * world
		s := math.Sin(lat * math.Pi / 180)
		y := (0.5 - math.Log((1+s)/(1-s))/(4*math.Pi)) * world
		return x, y
	}
	frameBox := [4]float64{math.Min(vp.BBox[0], vp.BBox[2]), math.Min(vp.BBox[1], vp.BBox[3]), math.Max(vp.BBox[0], vp.BBox[2]), math.Max(vp.BBox[1], vp.BBox[3])}
	cu, cv := view.W/2, view.H/2
	ox, oy := toWorld(cu, cv)
	px1, py1 := toWorld(cu+1, cv)
	ptWorld := math.Hypot(px1-ox, py1-oy) // world pixels in one point of the page

	doc, err := r.instance.OpenDocument(&requests.OpenDocument{File: &pdf})
	if err != nil {
		return nil, st, fmt.Errorf("open: %w", err)
	}
	defer r.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
	loaded, err := r.instance.FPDF_LoadPage(&requests.FPDF_LoadPage{Document: doc.Document, Index: 0})
	if err != nil {
		return nil, st, fmt.Errorf("load page: %w", err)
	}
	defer r.instance.FPDF_ClosePage(&requests.FPDF_ClosePage{Page: loaded.Page})
	page := requests.Page{ByReference: &loaded.Page}

	var styles, opsPer, ops, pts bytes.Buffer
	var nOps, nPts int
	west, north, east, south := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1) // north is the smallest world y
	putF := func(b *bytes.Buffer, f float32) { _ = binary.Write(b, binary.LittleEndian, f) }
	col := func(c uint) byte { return byte(min(c, 255)) }

	var walk func(o references.FPDF_PAGEOBJECT, parent [6]float64, depth int) error
	walk = func(o references.FPDF_PAGEOBJECT, parent [6]float64, depth int) error {
		st.MaxDepth = max(st.MaxDepth, depth)
		ty, err := r.instance.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{PageObject: o})
		if err != nil {
			return err
		}
		mm, err := r.instance.FPDFPageObj_GetMatrix(&requests.FPDFPageObj_GetMatrix{PageObject: o})
		if err != nil {
			return err
		}
		own := [6]float64{float64(mm.Matrix.A), float64(mm.Matrix.B), float64(mm.Matrix.C), float64(mm.Matrix.D), float64(mm.Matrix.E), float64(mm.Matrix.F)}
		m := mulMatrix(own, parent)
		switch ty.Type {
		case enums.FPDF_PAGEOBJ_TEXT:
			st.Objects["text"]++
		case enums.FPDF_PAGEOBJ_IMAGE:
			st.Objects["image"]++
		case enums.FPDF_PAGEOBJ_FORM:
			st.Objects["form"]++
			c, err := r.instance.FPDFFormObj_CountObjects(&requests.FPDFFormObj_CountObjects{PageObject: o})
			if err != nil {
				return err
			}
			for i := 0; i < c.Count; i++ {
				sub, err := r.instance.FPDFFormObj_GetObject(&requests.FPDFFormObj_GetObject{PageObject: o, Index: uint64(i)})
				if err != nil {
					return err
				}
				if err := walk(sub.PageObject, m, depth+1); err != nil {
					return err
				}
			}
		case enums.FPDF_PAGEOBJ_PATH:
			st.Objects["path"]++
			dm, err := r.instance.FPDFPath_GetDrawMode(&requests.FPDFPath_GetDrawMode{PageObject: o})
			if err != nil {
				return err
			}
			fill := dm.FillMode != enums.FPDF_FILLMODE_NONE
			if !fill && !dm.Stroke {
				st.Skipped++ // a clipping path or an invisible one
				return nil
			}
			var flags byte
			if fill {
				flags |= flagFill
				if dm.FillMode == enums.FPDF_FILLMODE_ALTERNATE {
					flags |= flagEvenOdd
				}
			}
			if dm.Stroke {
				flags |= flagStroke
			}
			fc, _ := r.instance.FPDFPageObj_GetFillColor(&requests.FPDFPageObj_GetFillColor{PageObject: o})
			sc, _ := r.instance.FPDFPageObj_GetStrokeColor(&requests.FPDFPageObj_GetStrokeColor{PageObject: o})
			sw, _ := r.instance.FPDFPageObj_GetStrokeWidth(&requests.FPDFPageObj_GetStrokeWidth{PageObject: o})
			var fr, fg, fb, fa, sr, sg, sb, sa byte
			if fc != nil {
				fr, fg, fb, fa = col(fc.FillColor.R), col(fc.FillColor.G), col(fc.FillColor.B), col(fc.FillColor.A)
			}
			if sc != nil {
				sr, sg, sb, sa = col(sc.StrokeColor.R), col(sc.StrokeColor.G), col(sc.StrokeColor.B), col(sc.StrokeColor.A)
			}
			width := 0.0
			if sw != nil {
				width = float64(sw.StrokeWidth) * math.Sqrt(math.Abs(m[0]*m[3]-m[1]*m[2]))
			}

			c, err := r.instance.FPDFPath_CountSegments(&requests.FPDFPath_CountSegments{PageObject: o})
			if err != nil {
				return err
			}
			var pathOps int
			var lops bytes.Buffer
			var lpts []float32
			ux0, uy0, ux1, uy1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
			point := func(seg references.FPDF_PATHSEGMENT) error {
				p, err := r.instance.FPDFPathSegment_GetPoint(&requests.FPDFPathSegment_GetPoint{PathSegment: seg})
				if err != nil {
					return err
				}
				x, y := float64(p.X), float64(p.Y)
				ux, uy := m[0]*x+m[2]*y+m[4], m[1]*x+m[3]*y+m[5]
				ux0, ux1 = math.Min(ux0, ux), math.Max(ux1, ux)
				uy0, uy1 = math.Min(uy0, uy), math.Max(uy1, uy)
				u, v := view.point(ux, uy)
				wx, wy := toWorld(u, v)
				lpts = append(lpts, float32(wx-ox), float32(wy-oy))
				return nil
			}
			for i := 0; i < c.Count; i++ {
				seg, err := r.instance.FPDFPath_GetPathSegment(&requests.FPDFPath_GetPathSegment{PageObject: o, Index: i})
				if err != nil {
					return err
				}
				k, err := r.instance.FPDFPathSegment_GetType(&requests.FPDFPathSegment_GetType{PathSegment: seg.PathSegment})
				if err != nil {
					return err
				}
				st.Segments++
				switch k.Type {
				case enums.FPDF_SEGMENT_MOVETO:
					lops.WriteByte(0)
					pathOps++
					if err := point(seg.PathSegment); err != nil {
						return err
					}
				case enums.FPDF_SEGMENT_LINETO:
					lops.WriteByte(1)
					pathOps++
					if err := point(seg.PathSegment); err != nil {
						return err
					}
				case enums.FPDF_SEGMENT_BEZIERTO:
					// PDFium lists a cubic curve as three BEZIERTO segments in a row: control, control, end
					if i+2 >= c.Count {
						return fmt.Errorf("a curve cut short")
					}
					lops.WriteByte(2)
					pathOps++
					for j := 0; j < 3; j++ {
						s2 := seg
						if j > 0 {
							s2, err = r.instance.FPDFPath_GetPathSegment(&requests.FPDFPath_GetPathSegment{PageObject: o, Index: i + j})
							if err != nil {
								return err
							}
						}
						if err := point(s2.PathSegment); err != nil {
							return err
						}
					}
					st.Segments += 2
					i += 2
					seg, _ = r.instance.FPDFPath_GetPathSegment(&requests.FPDFPath_GetPathSegment{PageObject: o, Index: i})
				}
				if cl, err := r.instance.FPDFPathSegment_GetClose(&requests.FPDFPathSegment_GetClose{PathSegment: seg.PathSegment}); err == nil && cl.IsClose {
					lops.WriteByte(3)
					pathOps++
				}
			}
			if ux1 < frameBox[0] || ux0 > frameBox[2] || uy1 < frameBox[1] || uy0 > frameBox[3] {
				st.Skipped++ // wholly outside the map frame: a legend, a margin
				return nil
			}
			nOps += pathOps
			lops.WriteTo(&ops)
			for _, f := range lpts {
				putF(&pts, f)
			}
			nPts += len(lpts) / 2
			st.Paths++
			styles.Write([]byte{fr, fg, fb, fa, sr, sg, sb, sa, flags})
			putF(&styles, float32(width*ptWorld))
			_ = binary.Write(&opsPer, binary.LittleEndian, uint32(pathOps))
		}
		return nil
	}

	cnt, err := r.instance.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{Page: page})
	if err != nil {
		return nil, st, err
	}
	ident := [6]float64{1, 0, 0, 1, 0, 0}
	for i := 0; i < cnt.Count; i++ {
		o, err := r.instance.FPDFPage_GetObject(&requests.FPDFPage_GetObject{Page: page, Index: i})
		if err != nil {
			return nil, st, err
		}
		if err := walk(o.PageObject, ident, 1); err != nil {
			return nil, st, err
		}
	}

	unproject := func(x, y float64) (float64, float64) {
		lon := x/world*360 - 180
		lat := math.Atan(math.Sinh(math.Pi*(1-2*y/world))) * 180 / math.Pi
		return lon, lat
	}
	var frame [4][2]float64
	for i, c := range [4][2]float64{{frameBox[0], frameBox[1]}, {frameBox[2], frameBox[1]}, {frameBox[2], frameBox[3]}, {frameBox[0], frameBox[3]}} {
		u, v := view.point(c[0], c[1])
		wx, wy := toWorld(u, v)
		frame[i] = [2]float64{wx - ox, wy - oy}
		west, east = math.Min(west, wx), math.Max(east, wx)
		north, south = math.Min(north, wy), math.Max(south, wy)
	}
	w, n2 := unproject(west, north)
	e, s2 := unproject(east, south)
	h := VectorHeader{Version: 1, Zoom: WorldZoom, Origin: [2]float64{ox, oy}, Paths: st.Paths, Ops: nOps, Points: nPts,
		Bounds: [4]float64{w, s2, e, n2}, Frame: frame}
	hj, err := json.Marshal(h)
	if err != nil {
		return nil, st, err
	}
	var out bytes.Buffer
	_ = binary.Write(&out, binary.LittleEndian, uint32(len(hj)))
	out.Write(hj)
	for len(out.Bytes())%4 != 0 {
		out.WriteByte(' ')
	}
	out.Write(styles.Bytes())
	out.Write(opsPer.Bytes())
	out.Write(ops.Bytes())
	for out.Len()%4 != 0 {
		out.WriteByte(0)
	}
	out.Write(pts.Bytes())
	return out.Bytes(), st, nil
}

// mulMatrix returns the matrix that applies a and then b (PDF order: [a b c d e f]).
func mulMatrix(a, b [6]float64) [6]float64 {
	return [6]float64{
		a[0]*b[0] + a[1]*b[2], a[0]*b[1] + a[1]*b[3],
		a[2]*b[0] + a[3]*b[2], a[2]*b[1] + a[3]*b[3],
		a[4]*b[0] + a[5]*b[2] + b[4], a[4]*b[1] + a[5]*b[3] + b[5],
	}
}
