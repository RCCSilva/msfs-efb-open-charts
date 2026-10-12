package server

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// a small coloured "map": white ground, a green park, an orange road and a black label
func fakeOSMTile(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			c := color.RGBA{248, 244, 240, 255} // OSM ground
			switch {
			case x > 40 && x < 120 && y > 40 && y < 120:
				c = color.RGBA{173, 209, 158, 255} // park
			case y > 150 && y < 158:
				c = color.RGBA{252, 214, 164, 255} // road
			case x > 180 && x < 200 && y > 60 && y < 64:
				c = color.RGBA{0, 0, 0, 255} // label
			}
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func gray(t *testing.T, jpg []byte, x, y int) uint8 {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(jpg))
	if err != nil {
		t.Fatalf("not a JPEG: %v", err)
	}
	r, _, _, _ := img.At(x, y).RGBA()
	return uint8(r >> 8)
}

func TestRestyleLightIsQuietAndKeepsLabelsReadable(t *testing.T) {
	src := fakeOSMTile(t)
	out, err := restyle(src, BasemapLight)
	if err != nil {
		t.Fatal(err)
	}
	ground, park, label := gray(t, out, 5, 5), gray(t, out, 80, 80), gray(t, out, 190, 62)
	if ground < 225 {
		t.Fatalf("the ground must stay light, got %d", ground)
	}
	if label >= ground-80 {
		t.Fatalf("labels must still contrast with the ground: label %d vs ground %d", label, ground)
	}
	if ground-park > 70 {
		t.Fatalf("a park must be only slightly different from the ground (quiet map): ground %d park %d", ground, park)
	}
}

func TestRestyleDarkIsDark(t *testing.T) {
	out, err := restyle(fakeOSMTile(t), BasemapDark)
	if err != nil {
		t.Fatal(err)
	}
	if g := gray(t, out, 5, 5); g > 60 {
		t.Fatalf("dark ground = %d, want a dark value", g)
	}
	if l := gray(t, out, 190, 62); l <= gray(t, out, 5, 5) {
		t.Fatalf("dark map: labels must be lighter than the ground (label %d)", l)
	}
}

// Styled tiles are produced from the original, cached, and fetched from OSM only once.
func TestStyledTileIsServedAndCached(t *testing.T) {
	var calls int32
	tc, _ := newTestTiles(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		_, _ = w.Write(fakeOSMTile(t))
	}))
	tc.backoff = time.Millisecond
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tiles/{style}/{z}/{x}/{file}", tc.handleStyled)
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}
	first := get("/tiles/light/5/3/2.jpg")
	if first.Code != 200 || first.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("first: %d %s", first.Code, first.Header().Get("Content-Type"))
	}
	if _, err := jpeg.Decode(bytes.NewReader(first.Body.Bytes())); err != nil {
		t.Fatalf("body is not a JPEG: %v", err)
	}
	// the dark version of the same tile reuses the original: no second download
	if dark := get("/tiles/dark/5/3/2.jpg"); dark.Code != 200 {
		t.Fatalf("dark: %d", dark.Code)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("upstream called %d times, want 1", n)
	}
	// served again from disk
	if again := get("/tiles/light/5/3/2.jpg"); again.Code != 200 || atomic.LoadInt32(&calls) != 1 {
		t.Fatal("second request must come from the cache")
	}
	// only the known styles; out-of-range tiles are not found
	if r := get("/tiles/neon/5/3/2.jpg"); r.Code != 404 {
		t.Fatalf("unknown style: %d, want 404", r.Code)
	}
	if r := get("/tiles/light/3/99/2.jpg"); r.Code != 404 {
		t.Fatalf("tile out of range: %d, want 404", r.Code)
	}
	_ = filepath.Join
}

func TestTruecolorTileIsPlainRGB(t *testing.T) {
	pal := image.NewPaletted(image.Rect(0, 0, 8, 8), color.Palette{color.RGBA{10, 20, 30, 255}, color.RGBA{200, 100, 50, 255}})
	pal.SetColorIndex(1, 1, 1)
	var in bytes.Buffer
	if err := png.Encode(&in, pal); err != nil {
		t.Fatal(err)
	}
	out, err := toTruecolor(in.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ColorModel != color.RGBAModel {
		t.Fatalf("color model = %v, want RGBA (24-bit truecolor)", cfg.ColorModel)
	}
	img, _ := png.Decode(bytes.NewReader(out))
	if r, g, b, _ := img.At(1, 1).RGBA(); r>>8 != 200 || g>>8 != 100 || b>>8 != 50 {
		t.Fatalf("pixel changed: %d %d %d", r>>8, g>>8, b>>8)
	}
}
