package importer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

// A chart too big to show whole is cut into a pyramid of tiles, so a viewer loads only what is on
// screen and at the sharpness the zoom needs.
//
//   - Level 0 is the picture at full resolution; every next level is half the size of the one
//     before, down to the overview that is chart.png itself.
//   - The picture is padded with white to a multiple of 2^levels, so that the overview is exactly
//     one 2^levels-th of it and the same coordinates serve every level.
//   - Level k is stored under the map zoom it is shown at: z = levels - k. Zoom 0 is the overview
//     (chart.png) resolution; zoom `levels` is the full resolution. Below zoom 0 there are coarser
//     levels (-1, -2...) until the whole chart fits in one tile, so a zoomed-out view is made of
//     filtered tiles and not of a picture shrunk by the browser.
//
// Tiles are TileSize x TileSize PNGs, in the picture's own palette when it has one: a chart has few
// colours, so they are small and exact.

// TileSize is the side of a tile, in pixels.
const TileSize = 512

// Tiles describes a pyramid in chart.json.
type Tiles struct {
	Size    int    `json:"size"`    // tile side in pixels
	Format  string `json:"format"`  // "png"
	MaxZoom int    `json:"maxZoom"` // the finest zoom (full resolution); zoom 0 is chart.png's resolution
	MinZoom int    `json:"minZoom"` // the coarsest zoom: the whole chart fits in one tile (0, -1, -2...)
	Width   int    `json:"width"`   // picture width at the finest zoom (padded)
	Height  int    `json:"height"`
	// Path is relative to the chart: tiles/{z}/{x}_{y}.png (x to the right, y downwards).
	Path string `json:"path"`
}

// pyramidLevels picks how many halvings bring the picture closest to the target long edge.
// 0 means the picture is small enough as it is.
func pyramidLevels(w, h, target int) int {
	long := max(w, h)
	if float64(long) <= float64(target)*1.2 {
		return 0
	}
	best, bestScore := 0, math.Inf(1)
	for m := 0; m <= 6; m++ {
		over := math.Ceil(float64(long) / float64(int(1)<<m))
		score := math.Abs(math.Log(over / float64(target)))
		if score < bestScore {
			best, bestScore = m, score
		}
	}
	return best
}

// buildPyramid cuts src into tiles for levels 0..m and returns the overview (level m) and the
// tile files by relative path. src may be a palette picture (the common case for charts).
func buildPyramid(src image.Image, m int) (*image.RGBA, map[string][]byte, Tiles, error) {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	p := 1 << m
	w0, h0 := (sw+p-1)/p*p, (sh+p-1)/p*p
	files := map[string][]byte{}
	info := Tiles{Size: TileSize, Format: "png", MaxZoom: m, Width: w0, Height: h0, Path: "tiles/{z}/{x}_{y}.png"}

	var q *quantizer
	if pal, ok := src.(*image.Paletted); ok {
		q = newQuantizer(pal.Palette)
	}
	encode := func(img *image.RGBA, x0, y0, tw, th int) ([]byte, error) {
		var buf bytes.Buffer
		enc := png.Encoder{CompressionLevel: png.DefaultCompression}
		if q != nil {
			// always a whole tile: a smaller one would be stretched to the tile size by the viewer
			white := q.index(255, 255, 255)
			t := image.NewPaletted(image.Rect(0, 0, TileSize, TileSize), q.palette)
			for y := 0; y < TileSize; y++ {
				for x := 0; x < TileSize; x++ {
					if x >= tw || y >= th {
						t.Pix[y*t.Stride+x] = white
						continue
					}
					o := (y0+y)*img.Stride + (x0+x)*4
					t.Pix[y*t.Stride+x] = q.index(img.Pix[o], img.Pix[o+1], img.Pix[o+2])
				}
			}
			if err := enc.Encode(&buf, t); err != nil {
				return nil, err
			}
		} else {
			t := image.NewRGBA(image.Rect(0, 0, TileSize, TileSize))
			for i := range t.Pix {
				t.Pix[i] = 255
			}
			for y := 0; y < th; y++ {
				copy(t.Pix[y*t.Stride:y*t.Stride+tw*4], img.Pix[(y0+y)*img.Stride+x0*4:])
			}
			if err := enc.Encode(&buf, t); err != nil {
				return nil, err
			}
		}
		return buf.Bytes(), nil
	}

	// level 0 is cut straight from the source, tile by tile: a 100-megapixel picture is never held a
	// second time as RGBA
	z0 := m
	for ty := 0; ty*TileSize < h0; ty++ {
		for tx := 0; tx*TileSize < w0; tx++ {
			data, err := sourceTile(src, q, tx*TileSize, ty*TileSize, TileSize, TileSize, w0, h0)
			if err != nil {
				return nil, nil, Tiles{}, err
			}
			files[fmt.Sprintf("tiles/%d/%d_%d.png", z0, tx, ty)] = data
		}
	}

	var overview *image.RGBA
	if m == 0 {
		overview = areaResampleV(src, w0, h0, w0, h0)
	}
	// the levels past the overview are made too, down to a single tile
	total := m
	for (w0>>total) > TileSize || (h0>>total) > TileSize {
		total++
	}
	info.MinZoom = m - total
	for k := 1; k <= total; k++ {
		lw, lh := max(1, w0>>k), max(1, h0>>k)
		// the level, reduced from the source by area averaging
		level := areaResampleV(src, w0, h0, lw, lh)
		z := m - k
		for ty := 0; ty*TileSize < lh; ty++ {
			for tx := 0; tx*TileSize < lw; tx++ {
				x0, y0 := tx*TileSize, ty*TileSize
				data, err := encode(level, x0, y0, min(TileSize, lw-x0), min(TileSize, lh-y0))
				if err != nil {
					return nil, nil, Tiles{}, err
				}
				files[fmt.Sprintf("tiles/%d/%d_%d.png", z, tx, ty)] = data
			}
		}
		if k == m {
			overview = level
		}
	}
	return overview, files, info, nil
}

// quantizer maps an RGB colour to the nearest entry of a palette (remembering the answers: a
// chart repeats a few thousand colours millions of times).
type quantizer struct {
	palette color.Palette
	memo    map[uint32]uint8
}

func newQuantizer(p color.Palette) *quantizer {
	return &quantizer{palette: p, memo: map[uint32]uint8{}}
}

func (q *quantizer) index(r, g, b uint8) uint8 {
	key := uint32(r)<<16 | uint32(g)<<8 | uint32(b)
	if v, ok := q.memo[key]; ok {
		return v
	}
	best, bestD := 0, math.MaxInt
	for i, c := range q.palette {
		pr, pg, pb, _ := c.RGBA()
		dr, dg, db := int(r)-int(pr>>8), int(g)-int(pg>>8), int(b)-int(pb>>8)
		if d := dr*dr + dg*dg + db*db; d < bestD {
			best, bestD = i, d
		}
	}
	q.memo[key] = uint8(best)
	return uint8(best)
}

// sourceTile encodes the tile at (x0, y0) of the source picture, padded with white to w0 x h0.
func sourceTile(src image.Image, q *quantizer, x0, y0, tw, th, w0, h0 int) ([]byte, error) {
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	// a whole tile, white beyond the picture (a short tile would be stretched by the viewer)
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if pal, ok := src.(*image.Paletted); ok && q != nil {
		white := q.index(255, 255, 255)
		t := image.NewPaletted(image.Rect(0, 0, tw, th), pal.Palette)
		for y := 0; y < th; y++ {
			row := t.Pix[y*t.Stride : y*t.Stride+tw]
			for x := range row {
				if sx, sy := x0+x, y0+y; sx < sw && sy < sh && x0+x < w0 && y0+y < h0 {
					row[x] = pal.Pix[sy*pal.Stride+sx]
				} else {
					row[x] = white
				}
			}
		}
		if err := enc.Encode(&buf, t); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	t := image.NewRGBA(image.Rect(0, 0, tw, th))
	for y := 0; y < th; y++ {
		for x := 0; x < tw; x++ {
			o := y*t.Stride + x*4
			if sx, sy := x0+x, y0+y; sx < sw && sy < sh && x0+x < w0 && y0+y < h0 {
				r, g, bl, _ := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
				t.Pix[o], t.Pix[o+1], t.Pix[o+2], t.Pix[o+3] = uint8(r>>8), uint8(g>>8), uint8(bl>>8), 255
			} else {
				t.Pix[o], t.Pix[o+1], t.Pix[o+2], t.Pix[o+3] = 255, 255, 255, 255
			}
		}
	}
	if err := enc.Encode(&buf, t); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
