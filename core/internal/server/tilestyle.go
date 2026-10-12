package server

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Base map styles. The data is always OpenStreetMap's; the server only re-draws the standard
// tile as a quiet grey-scale JPEG so the chart on top is what stands out, and the tile is a
// fraction of the size of the coloured original (about 36 KB -> 6-9 KB).
//
//	standard  the OpenStreetMap tile as it is (coloured, PNG)
//	light     soft grey, for use under a chart
//	dark      inverted and dimmed, for night flying
//	none      no map at all (the viewer draws a plain background; nothing is requested)
const (
	BasemapStandard = "standard"
	BasemapLight    = "light"
	BasemapDark     = "dark"
	BasemapNone     = "none"
)

// restyle redraws an OSM PNG tile in the given style ("light" or "dark") as a JPEG. The picture is
// grey but stored as colour (3 channels): the simulator's browser handles plain RGB images best.
func restyle(pngData []byte, style string) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	dst := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := src.At(x, y).RGBA() // 16-bit
			lum := (299*r + 587*g + 114*bl) / 1000 >> 8
			var v float64
			switch style {
			case BasemapDark:
				// white ground -> near black, dark labels -> mid grey
				v = 22 + (255-float64(lum))*0.40
			default: // light
				// white ground stays white, black labels become a readable dark grey
				v = 84 + float64(lum)*0.67
			}
			gv := uint8(v)
			dst.SetRGBA(x, y, color.RGBA{R: gv, G: gv, B: gv, A: 255})
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 72}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// handleStyled serves /tiles/{style}/{z}/{x}/{y}.jpg for the derived styles. The derived tile is
// cached on disk next to the original, so the work is done once per tile.
func (t *tileCache) handleStyled(w http.ResponseWriter, r *http.Request) {
	style := r.PathValue("style")
	if style != BasemapLight && style != BasemapDark {
		http.NotFound(w, r)
		return
	}
	z, err1 := strconv.Atoi(r.PathValue("z"))
	x, err2 := strconv.Atoi(r.PathValue("x"))
	y, err3 := strconv.Atoi(strings.TrimSuffix(r.PathValue("file"), ".jpg"))
	if err1 != nil || err2 != nil || err3 != nil || z < 0 || z > tileMaxZoom || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
		http.NotFound(w, r)
		return
	}
	derived := filepath.Join(t.dir, "_"+style+"-rgb", strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".jpg")
	if data, err := os.ReadFile(derived); err == nil {
		sendJPEG(w, data)
		return
	}
	raw, err := t.original(z, x, y)
	if err != nil {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "2")
		http.Error(w, "tile fetch failed", http.StatusServiceUnavailable)
		return
	}
	out, err := restyle(raw, style)
	if err != nil {
		http.Error(w, errors.New("tile could not be drawn").Error(), http.StatusBadGateway)
		return
	}
	if err := os.MkdirAll(filepath.Dir(derived), 0o755); err == nil {
		tmp := derived + ".tmp"
		if os.WriteFile(tmp, out, 0o644) == nil {
			_ = os.Rename(tmp, derived)
		}
	}
	sendJPEG(w, out)
}

func sendJPEG(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = w.Write(data)
}

// truecolor returns the tile as a plain 24-bit RGB PNG. OpenStreetMap serves 8-bit palette PNGs,
// an unusual pixel format; the simulator's browser turned those tiles black. The converted tile is
// kept on disk next to the original.
func (t *tileCache) truecolor(z, x, y int) ([]byte, error) {
	file := filepath.Join(t.dir, "_rgb", strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".png")
	if data, err := os.ReadFile(file); err == nil {
		return data, nil
	}
	raw, err := t.original(z, x, y)
	if err != nil {
		return nil, err
	}
	out, err := toTruecolor(raw)
	if err != nil {
		return raw, nil // an odd tile is still better than none
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err == nil {
		tmp := file + ".tmp"
		if os.WriteFile(tmp, out, 0o644) == nil {
			_ = os.Rename(tmp, file)
		}
	}
	return out, nil
}

func toTruecolor(pngData []byte) ([]byte, error) {
	src, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, image.NewUniform(color.White), image.Point{}, draw.Src)
	draw.Draw(dst, b, src, b.Min, draw.Over)
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
