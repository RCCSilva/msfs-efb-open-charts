package geotiff

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"

	"golang.org/x/image/tiff/lzw"
)

// Tags of the picture itself.
const (
	tagBitsPerSample = 258
	tagCompression   = 259
	tagPhotometric   = 262
	tagStripOffsets  = 273
	tagSamplesPerPx  = 277
	tagRowsPerStrip  = 278
	tagStripCounts   = 279
	tagPlanar        = 284
	tagPredictor     = 317
	tagColorMap      = 320
	tagTileWidth     = 322
	tagTileLength    = 323
	tagTileOffsets   = 324
	tagTileCounts    = 325

	// a chart never needs more than this many pixels; it stops a hostile header from asking for
	// gigabytes
	maxPixels = 400 << 20
)

// uints reads a SHORT or LONG array.
func (rd *reader) uints(e entry) ([]uint32, bool) {
	if e.count == 0 || (e.typ != 3 && e.typ != 4) {
		return nil, false
	}
	b, ok := rd.data(e)
	if !ok {
		return nil, false
	}
	out := make([]uint32, e.count)
	for i := range out {
		if e.typ == 3 {
			out[i] = uint32(rd.bo.Uint16(b[i*2:]))
		} else {
			out[i] = rd.bo.Uint32(b[i*4:])
		}
	}
	return out, true
}

// Decode reads the picture of a baseline-or-LZW TIFF: 8 bits per sample, one band with a palette
// or grey levels, or RGB; strips or tiles; no compression, LZW, Deflate or PackBits; optional
// horizontal predictor. That covers the FAA's rasters; anything else is refused by name.
func Decode(data []byte) (image.Image, error) {
	if !IsTIFF(data) {
		return nil, ErrNotTIFF
	}
	rd := &reader{r: bytes.NewReader(data)}
	if data[0] == 'I' {
		rd.bo = binary.LittleEndian
	} else {
		rd.bo = binary.BigEndian
	}
	tags, err := rd.readIFD(int64(rd.bo.Uint32(data[4:8])))
	if err != nil {
		return nil, err
	}
	one := func(tag uint16, def uint32) uint32 {
		if v, ok := rd.uint(tags[tag]); ok {
			return v
		}
		return def
	}
	w, h := int(one(tagImageWidth, 0)), int(one(tagImageLength, 0))
	if w <= 0 || h <= 0 || w*h > maxPixels {
		return nil, fmt.Errorf("TIFF picture size %dx%d is not usable", w, h)
	}
	if bits, ok := rd.uints(tags[tagBitsPerSample]); !ok || bits[0] != 8 {
		return nil, errors.New("only 8-bit TIFFs are supported")
	}
	spp := int(one(tagSamplesPerPx, 1))
	photo := one(tagPhotometric, 1)
	comp := one(tagCompression, 1)
	predictor := one(tagPredictor, 1)
	if one(tagPlanar, 1) != 1 {
		return nil, errors.New("planar TIFFs are not supported")
	}
	switch comp {
	case 1, 5, 8, 32946, 32773:
	default:
		return nil, fmt.Errorf("TIFF compression %d is not supported (supported: none, LZW, Deflate, PackBits)", comp)
	}
	if predictor != 1 && predictor != 2 {
		return nil, fmt.Errorf("TIFF predictor %d is not supported", predictor)
	}

	// destination
	var pal *image.Paletted
	var gray *image.Gray
	var rgba *image.RGBA
	bytesPerPx := spp
	switch {
	case photo == 3 && spp == 1:
		cm, ok := rd.shorts(tags[tagColorMap])
		if !ok || len(cm) < 768 {
			return nil, errors.New("palette TIFF without a (complete) colour map")
		}
		p := make(color.Palette, 256)
		for i := range p {
			p[i] = color.RGBA{uint8(cm[i] >> 8), uint8(cm[256+i] >> 8), uint8(cm[512+i] >> 8), 255}
		}
		pal = image.NewPaletted(image.Rect(0, 0, w, h), p)
	case (photo == 0 || photo == 1) && spp == 1:
		gray = image.NewGray(image.Rect(0, 0, w, h))
	case photo == 2 && spp >= 3:
		rgba = image.NewRGBA(image.Rect(0, 0, w, h))
	default:
		return nil, fmt.Errorf("TIFF colour layout (photometric %d, %d samples) is not supported", photo, spp)
	}

	put := func(x0, y0, bw, bh int, px []byte, rowBytes int) {
		for y := 0; y < bh && y0+y < h; y++ {
			row := px[y*rowBytes:]
			cw := min(bw, w-x0)
			switch {
			case pal != nil:
				copy(pal.Pix[(y0+y)*pal.Stride+x0:], row[:cw])
			case gray != nil:
				if photo == 0 { // WhiteIsZero
					for i := 0; i < cw; i++ {
						gray.Pix[(y0+y)*gray.Stride+x0+i] = 255 - row[i]
					}
				} else {
					copy(gray.Pix[(y0+y)*gray.Stride+x0:], row[:cw])
				}
			default:
				o := (y0+y)*rgba.Stride + x0*4
				for i := 0; i < cw; i++ {
					rgba.Pix[o+i*4], rgba.Pix[o+i*4+1], rgba.Pix[o+i*4+2], rgba.Pix[o+i*4+3] = row[i*spp], row[i*spp+1], row[i*spp+2], 255
				}
			}
		}
	}

	// unpack one block of compressed bytes into rows of bw pixels
	block := func(src []byte, bw, bh int) ([]byte, error) {
		want := bw * bh * bytesPerPx
		var out []byte
		switch comp {
		case 1:
			out = src
		case 5:
			r := lzw.NewReader(bytes.NewReader(src), lzw.MSB, 8)
			out = make([]byte, want)
			n, err := io.ReadFull(r, out)
			_ = r.Close()
			if err != nil && n < want {
				return nil, fmt.Errorf("LZW data ended early (%d of %d bytes)", n, want)
			}
		case 8, 32946:
			zr, err := zlib.NewReader(bytes.NewReader(src))
			if err != nil {
				return nil, err
			}
			out = make([]byte, want)
			if n, err := io.ReadFull(zr, out); err != nil && n < want {
				return nil, fmt.Errorf("Deflate data ended early (%d of %d bytes)", n, want)
			}
		case 32773:
			out = unpackBits(src, want)
		}
		if len(out) < want {
			return nil, fmt.Errorf("picture data is short (%d of %d bytes)", len(out), want)
		}
		if predictor == 2 {
			for y := 0; y < bh; y++ {
				row := out[y*bw*bytesPerPx:]
				for i := bytesPerPx; i < bw*bytesPerPx; i++ {
					row[i] += row[i-bytesPerPx]
				}
			}
		}
		return out, nil
	}

	slice := func(off, n uint32) ([]byte, error) {
		if uint64(off)+uint64(n) > uint64(len(data)) {
			return nil, errors.New("picture data is beyond the end of the file")
		}
		return data[off : off+n], nil
	}

	if tw, ok := rd.uint(tags[tagTileWidth]); ok && tw > 0 {
		th := one(tagTileLength, 0)
		offs, ok1 := rd.uints(tags[tagTileOffsets])
		cnts, ok2 := rd.uints(tags[tagTileCounts])
		if !ok1 || !ok2 || th == 0 {
			return nil, errors.New("tiled TIFF without tile tables")
		}
		across := (w + int(tw) - 1) / int(tw)
		for i := range offs {
			if i >= len(cnts) {
				break
			}
			src, err := slice(offs[i], cnts[i])
			if err != nil {
				return nil, err
			}
			px, err := block(src, int(tw), int(th))
			if err != nil {
				return nil, err
			}
			put((i%across)*int(tw), (i/across)*int(th), int(tw), int(th), px, int(tw)*bytesPerPx)
		}
		return finish(pal, gray, rgba), nil
	}

	offs, ok1 := rd.uints(tags[tagStripOffsets])
	cnts, ok2 := rd.uints(tags[tagStripCounts])
	if !ok1 {
		return nil, errors.New("TIFF without strip offsets")
	}
	rps := int(one(tagRowsPerStrip, uint32(h)))
	if rps <= 0 || rps > h {
		rps = h
	}
	for i := range offs {
		y0 := i * rps
		if y0 >= h {
			break
		}
		bh := min(rps, h-y0)
		var count uint32
		if ok2 && i < len(cnts) {
			count = cnts[i]
		} else if comp == 1 {
			count = uint32(w * bh * bytesPerPx)
		} else {
			return nil, errors.New("TIFF without strip byte counts")
		}
		src, err := slice(offs[i], count)
		if err != nil {
			return nil, err
		}
		px, err := block(src, w, bh)
		if err != nil {
			return nil, err
		}
		put(0, y0, w, bh, px, w*bytesPerPx)
	}
	return finish(pal, gray, rgba), nil
}

func finish(pal *image.Paletted, gray *image.Gray, rgba *image.RGBA) image.Image {
	switch {
	case pal != nil:
		return pal
	case gray != nil:
		return gray
	}
	return rgba
}

// unpackBits decodes PackBits runs until want bytes are produced.
func unpackBits(src []byte, want int) []byte {
	out := make([]byte, 0, want)
	for i := 0; i < len(src) && len(out) < want; {
		n := int(int8(src[i]))
		i++
		switch {
		case n >= 0:
			end := min(len(src), i+n+1)
			out = append(out, src[i:end]...)
			i = end
		case n != -128:
			if i < len(src) {
				for k := 0; k < 1-n; k++ {
					out = append(out, src[i])
				}
				i++
			}
		}
	}
	return out
}
