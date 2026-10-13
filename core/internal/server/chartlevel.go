package server

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/image/draw"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
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

// handleChartTile serves one tile of a chart's pyramid: /charts/{id}/tiles/{z}/{x}/{y}.png (the
// file on disk is tiles/{z}/{x}_{y}.png). The coordinates are numbers, never a path.
func (s *Server) handleChartTile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	z, err1 := strconv.Atoi(r.PathValue("z"))
	x, err2 := strconv.Atoi(r.PathValue("x"))
	y, err3 := strconv.Atoi(strings.TrimSuffix(r.PathValue("y"), ".png"))
	if !store.ValidID(id) || err1 != nil || err2 != nil || err3 != nil || z < -12 || z > 12 || x < 0 || y < 0 {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(s.lib.ChartDir(id), "tiles", strconv.Itoa(z), strconv.Itoa(x)+"_"+strconv.Itoa(y)+".png")
	if _, err := os.Stat(path); err != nil {
		http.NotFound(w, r)
		return
	}
	// a chart can be imported again under the same id: revalidate (a cheap 304 on localhost)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, r, path)
}
