package server

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestChartLevelsAreMadeOnDemandAndRefreshed(t *testing.T) {
	dir := t.TempDir()
	src := image.NewRGBA(image.Rect(0, 0, 40, 20))
	for x := 0; x < 40; x++ {
		for y := 0; y < 20; y++ {
			src.Set(x, y, color.RGBA{uint8(x * 6), uint8(y * 12), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "chart.png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	p, ok := chartLevelPath(dir, "chart@2.png")
	if !ok {
		t.Fatal("level 2 was not made")
	}
	f, err := os.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	cfg, err := png.DecodeConfig(f)
	if err != nil || cfg.Width != 20 || cfg.Height != 10 {
		t.Fatalf("level 2 = %dx%d (%v), want 20x10", cfg.Width, cfg.Height, err)
	}
	if _, ok := chartLevelPath(dir, "chart@3.png"); ok {
		t.Fatal("only levels 2 and 4 exist")
	}
	if _, ok := chartLevelPath(dir, "chart@../x.png"); ok {
		t.Fatal("a path must never be accepted as a level name")
	}
}
