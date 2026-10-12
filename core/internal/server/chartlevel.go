package server

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"

	"golang.org/x/image/draw"
)

// A chart image is drawn by the browser at whatever size the map is zoomed to. Shrinking a big
// picture to a fraction of its size in the browser is rough (thin lines and small text break up),
// and it also costs the simulator's browser a lot to decode a very large PNG just to show it small.
// So the server keeps reduced copies, "chart@2.png" (half size) and "chart@4.png" (quarter size),
// made with a proper resampling filter the first time one is asked for, and the viewer asks for the
// one that matches its zoom.
var levelFileRe = regexp.MustCompile(`^chart@([24])\.png$`)

var levelMu sync.Mutex // one reduction at a time: it is memory heavy and rarely needed twice

// chartLevelPath returns the path of a reduced copy, making (or refreshing) it when needed.
func chartLevelPath(chartDir, file string) (string, bool) {
	m := levelFileRe.FindStringSubmatch(file)
	if m == nil {
		return "", false
	}
	div, _ := strconv.Atoi(m[1])
	src := filepath.Join(chartDir, "chart.png")
	dst := filepath.Join(chartDir, file)
	si, err := os.Stat(src)
	if err != nil {
		return "", false
	}
	levelMu.Lock()
	defer levelMu.Unlock()
	if di, err := os.Stat(dst); err == nil && !di.ModTime().Before(si.ModTime()) {
		return dst, true
	}
	if err := reduceChart(src, dst, div); err != nil {
		return "", false
	}
	return dst, true
}

func reduceChart(src, dst string, div int) error {
	raw, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return err
	}
	b := img.Bounds()
	w, h := b.Dx()/div, b.Dy()/div
	if w < 1 || h < 1 {
		w, h = 1, 1
	}
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(out, out.Bounds(), img, b, draw.Src, nil)
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.DefaultCompression}
	if err := enc.Encode(&buf, out); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
