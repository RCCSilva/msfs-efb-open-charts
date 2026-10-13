package importer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/geotiff"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
)

// Neatline is a geographic extent in degrees.
type Neatline struct{ West, East, South, North float64 }

// ParseFGDCBounds reads the bounding coordinates of the FGDC metadata (.htm) the FAA ships next
// to its rasters. They describe the whole file (margins and legends included), not the map area,
// so they are only a cross-check. ok is false when they are not all there.
func ParseFGDCBounds(htm []byte) (Neatline, bool) {
	text := regexp.MustCompile(`<[^>]+>`).ReplaceAll(htm, []byte(" "))
	get := func(name string) (float64, bool) {
		m := regexp.MustCompile(name + `_Bounding_Coordinate:\s*(-?\d+(?:\.\d+)?)`).FindSubmatch(text)
		if m == nil {
			return 0, false
		}
		v, err := strconv.ParseFloat(string(m[1]), 64)
		return v, err == nil
	}
	w, ok1 := get("West")
	e, ok2 := get("East")
	s, ok3 := get("South")
	n, ok4 := get("North")
	return Neatline{West: w, East: e, South: s, North: n}, ok1 && ok2 && ok3 && ok4
}

// ParseFGDCPublication reads Publication_Date (YYYYMMDD) of the FAA metadata as YYYY-MM-DD.
func ParseFGDCPublication(htm []byte) string {
	text := regexp.MustCompile(`<[^>]+>`).ReplaceAll(htm, []byte(" "))
	m := regexp.MustCompile(`Publication_Date:\s*(\d{8})`).FindSubmatch(text)
	if m == nil {
		return ""
	}
	return naming.ParseDate(string(m[1]))
}

// ImportGeoTIFF converts a georeferenced TIFF into chart.png + chart.json, the same pair a GeoPDF
// gives. The picture is reduced (by area averaging) so its longer side is opt.LongEdgePx (3200 by
// default) and never enlarged. It never guesses: a TIFF without usable georeferencing is an error.
func ImportGeoTIFF(tif []byte, opt Options) (*Result, error) {
	g, err := geotiff.ReadGeoref(tif)
	if err != nil {
		return nil, err
	}
	src, err := geotiff.Decode(tif)
	if err != nil {
		return nil, err
	}
	W, H := src.Bounds().Dx(), src.Bounds().Dy()
	if W != g.Width || H != g.Height {
		return nil, fmt.Errorf("the picture is %dx%d but the TIFF says %dx%d", W, H, g.Width, g.Height)
	}

	long := opt.LongEdgePx
	if long <= 0 {
		long = 3200
	}
	// A picture bigger than the overview size is also cut into a pyramid of tiles (see pyramid.go);
	// chart.png is then the overview, exactly one 2^levels-th of the picture.
	var out *image.RGBA
	var files map[string][]byte
	var tiles *Tiles
	var fx, fy float64
	if levels := pyramidLevels(W, H, long); levels > 0 {
		ov, tf, info, err := buildPyramid(src, levels)
		if err != nil {
			return nil, err
		}
		out, files, tiles = ov, tf, &info
		fx, fy = 1/float64(int(1)<<levels), 1/float64(int(1)<<levels)
	} else {
		out = areaResample(src, W, H)
		fx, fy = 1, 1
	}
	outW, outH := out.Bounds().Dx(), out.Bounds().Dy()

	var pngBuf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&pngBuf, out); err != nil {
		return nil, err
	}

	// projected metres -> pixels of chart.png
	var m Meta
	m.ToPixel.A = fx / g.DX
	m.ToPixel.C = -g.X0 * fx / g.DX
	m.ToPixel.E = fy / g.DY
	m.ToPixel.F = -g.Y0 * fy / g.DY

	// The map area. A chart raster carries margins and legends around the map, and only the map is
	// georeferenced: it is framed by a long continuous line, which is found in the picture itself.
	// When there is no such frame, the whole picture is the map.
	left, top, right, bottom := 0.0, 0.0, float64(outW), float64(outH)
	if l, t, r, bt, ok := detectMapFrame(out); ok {
		left, top, right, bottom = float64(l), float64(t), float64(r), float64(bt)
	}
	// corners of the map area, as latitude/longitude (needs the inverse projection)
	for _, c := range [][2]float64{{left, top}, {right, top}, {right, bottom}, {left, bottom}} {
		e, n := g.PixelToProjected(c[0]/fx, c[1]/fy)
		lat, lon, ok := g.Projection.Inverse(e, n)
		if !ok {
			return nil, fmt.Errorf("this projection cannot be turned back into latitude/longitude yet")
		}
		m.GeoCorners = append(m.GeoCorners, Corner{Lat: round(lat, 6), Lon: round(lon, 6), PX: round(c[0], 2), PY: round(c[1], 2)})
	}

	m.ID = opt.ID
	m.ICAO, m.Type, m.Title, m.Validity = opt.ICAO, opt.Type, opt.Title, opt.Validity
	m.Name = firstNonEmpty(opt.Name, naming.Display(opt.ICAO, opt.Type, opt.Title))
	m.Source = opt.Source
	m.AddedAt = time.Now().UTC().Format(time.RFC3339)
	m.OriginalFile = opt.Original
	m.Package = opt.Package
	m.Image.File, m.Image.Width, m.Image.Height = "chart.png", outW, outH
	m.Projection = g.Projection.JSON()
	m.MapFramePx.Left, m.MapFramePx.Top, m.MapFramePx.Right, m.MapFramePx.Bottom = round(left, 2), round(top, 2), round(right, 2), round(bottom, 2)
	m.Scale.X, m.Scale.Y = fx, fy
	m.Georef.MetersPerPixel = round(g.MetersPerPixel()/fx, 3)
	m.Generator = "open-charts/importer (geotiff, no cgo)"
	if g.Datum != "" {
		m.Georef.Viewport = g.Datum // the field is free text for the record
	}

	m.Tiles = tiles
	js, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return &Result{Meta: m, PNG: pngBuf.Bytes(), JSON: js, Files: files}, nil
}

// areaResample reduces src to w x h by averaging the area each new pixel covers: the right way to
// shrink a chart (thin lines and small text stay legible, unlike nearest-neighbour). It walks the
// source rows once, so a 90-megapixel palette image costs about a second.
func areaResample(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	return areaResampleV(src, b.Dx(), b.Dy(), w, h)
}

// areaResampleV is areaResample for a source taken as vw x vh pixels: what lies beyond the real
// picture (when it was padded to a multiple of a power of two) counts as white.
func areaResampleV(src image.Image, vw, vh, w, h int) *image.RGBA {
	b := src.Bounds()
	realW, realH := b.Dx(), b.Dy()
	sw, sh := vw, vh
	out := image.NewRGBA(image.Rect(0, 0, w, h))

	// horizontal weights: for each new column, the source columns it covers and by how much
	type span struct {
		first int
		wt    []float32
	}
	cols := make([]span, w)
	for x := 0; x < w; x++ {
		x0, x1 := float64(x)*float64(sw)/float64(w), float64(x+1)*float64(sw)/float64(w)
		first := int(math.Floor(x0))
		last := min(sw-1, int(math.Ceil(x1))-1)
		wt := make([]float32, 0, last-first+1)
		for j := first; j <= last; j++ {
			lo, hi := math.Max(x0, float64(j)), math.Min(x1, float64(j+1))
			wt = append(wt, float32((hi-lo)/(x1-x0)))
		}
		cols[x] = span{first, wt}
	}

	// how a source pixel becomes RGB
	var pal [256][3]float32
	pix := func(img image.Image, x, y int) (float32, float32, float32) {
		r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
		return float32(r >> 8), float32(g >> 8), float32(bl >> 8)
	}
	pimg, isPal := src.(*image.Paletted)
	if isPal {
		for i, c := range pimg.Palette {
			if i >= 256 {
				break
			}
			r, g, bl, _ := c.RGBA()
			pal[i] = [3]float32{float32(r >> 8), float32(g >> 8), float32(bl >> 8)}
		}
	}

	row := make([]float32, w*3) // one source row, reduced horizontally
	accA := make([]float32, w*3)
	// each source row feeds the new row(s) it overlaps
	cur := 0 // the new row being filled (accA)
	curEnd := float64(cur+1) * float64(sh) / float64(h)
	flush := func(acc []float32, y int, div float64) {
		o := out.Pix[y*out.Stride:]
		for x := 0; x < w; x++ {
			for c := 0; c < 3; c++ {
				v := float64(acc[x*3+c]) / div
				o[x*4+c] = uint8(math.Min(255, math.Max(0, v+0.5)))
			}
			o[x*4+3] = 255
		}
	}
	rowSpan := float64(sh) / float64(h)
	for y := 0; y < sh; y++ {
		// horizontal reduction of this source row
		for x := 0; x < w; x++ {
			var r, g, bl float32
			sp := cols[x]
			for k, wt := range sp.wt {
				xx := sp.first + k
				var pr, pg, pb float32
				if y >= realH || xx >= realW {
					pr, pg, pb = 255, 255, 255 // the padding
				} else if isPal {
					c := pal[pimg.Pix[y*pimg.Stride+xx]]
					pr, pg, pb = c[0], c[1], c[2]
				} else {
					pr, pg, pb = pix(src, xx, y)
				}
				r += pr * wt
				g += pg * wt
				bl += pb * wt
			}
			row[x*3], row[x*3+1], row[x*3+2] = r, g, bl
		}
		// vertical: this row covers [y, y+1) of the source; split between the new rows it touches
		top, bot := float64(y), float64(y+1)
		for top < bot-1e-9 {
			end := math.Min(bot, curEnd)
			wt := float32(end - top)
			for i := range row {
				accA[i] += row[i] * wt
			}
			top = end
			if end >= curEnd-1e-9 {
				flush(accA, cur, rowSpan)
				for i := range accA {
					accA[i] = 0
				}
				cur++
				curEnd = float64(cur+1) * rowSpan
				if cur >= h {
					return out
				}
			}
		}
	}
	return out
}

// detectMapFrame finds the rectangle drawn around the map. The frame's left and right sides are the
// outermost columns with a long unbroken dark line (at least half of the picture high); where those
// lines start and end gives the top and the bottom. (The horizontal lines below the map, the scale
// bar for example, are not the frame: the vertical sides say where the frame stops.) Gaps of a few
// pixels (a label or a tick crossing the line, up to gapTolerance) do not break a line.
func detectMapFrame(img *image.RGBA) (left, top, right, bottom int, ok bool) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	// a one-pixel line shrunk three times is no longer black: the threshold is generous
	const threshold, gapTolerance = 170, 40
	dark := func(x, y int) bool {
		p := img.Pix[y*img.Stride+x*4:]
		return (int(p[0])*299+int(p[1])*587+int(p[2])*114)/1000 < threshold
	}
	// longest dark run down column x: its length and where it starts
	longest := func(x int) (n, start int) {
		cur, curStart, gap := 0, 0, 0
		for y := 0; y < h; y++ {
			if dark(x, y) {
				if cur == 0 {
					curStart = y
				}
				cur += 1 + gap
				gap = 0
				if cur > n {
					n, start = cur, curStart
				}
			} else if cur > 0 {
				gap++
				if gap > gapTolerance {
					cur, gap = 0, 0
				}
			}
		}
		return n, start
	}
	// the left side is the first long line; the right side is the last line that spans the same
	// height (a box border of a legend panel is long enough but does not span it)
	left, right = -1, -1
	var s0, e0 int
	for x := 0; x < w; x++ {
		n, st := longest(x)
		if n < h/2 {
			continue
		}
		if left < 0 {
			left, s0, e0 = x, st, st+n
			right = x
			continue
		}
		span := e0 - s0
		tol := h / 33
		if n >= span*9/10 && abs(st-s0) <= tol && abs(st+n-e0) <= tol {
			right = x
		}
	}
	if left < 0 || right-left < w/4 {
		return 0, 0, 0, 0, false
	}
	top, bottom = s0, e0
	if bottom-top < h/4 {
		return 0, 0, 0, 0, false
	}
	return left, top, right + 1, bottom, true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
