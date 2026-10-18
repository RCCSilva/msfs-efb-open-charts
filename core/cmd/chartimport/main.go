// chartimport converts georeferenced chart PDFs into chart.png + chart.json.
//
//	chartimport [-dpi N] [-source S] [-name N] -out DIR file.pdf [file2.pdf ...]
//
// With several inputs, each one goes to DIR/<file stem>/.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
)

func main() {
	dpi := flag.Int("dpi", 0, "render resolution (0 = automatic)")
	out := flag.String("out", "", "output directory (required)")
	source := flag.String("source", "manual", "source id recorded in chart.json")
	name := flag.String("name", "", "display name (single input only)")
	detail := flag.Float64("detail", 0, "draw big charts sharper: metres per pixel wanted at the finest zoom (0 = off)")
	flag.Parse()
	files := flag.Args()
	if *out == "" || len(files) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	t0 := time.Now()
	r, err := importer.NewRenderer()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pdfium:", err)
		os.Exit(1)
	}
	defer r.Close()
	fmt.Printf("renderer ready in %v\n", time.Since(t0).Round(time.Millisecond))

	failed := 0
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", f, err)
			failed++
			continue
		}
		id := importer.StemID(f)
		start := time.Now()
		res, err := r.Import(data, importer.Options{ID: id, Name: *name, Source: *source, DPI: *dpi, DetailMPerPx: *detail})
		if err != nil {
			fmt.Printf("FAIL  %-60s %v\n", filepath.Base(f), err)
			failed++
			continue
		}
		dir := *out
		if len(files) > 1 {
			dir = filepath.Join(*out, id)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(dir, "chart.png"), res.PNG, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if err := os.WriteFile(filepath.Join(dir, "chart.json"), res.JSON, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		var tileBytes int64
		for rel, data := range res.Files {
			p := filepath.Join(dir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			if err := os.WriteFile(p, data, 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			tileBytes += int64(len(data))
		}
		m := res.Meta
		if m.Tiles != nil {
			fmt.Printf("      %d tiles, %.1f MB, zoom %d..%d, finest level %dx%d\n", len(res.Files), float64(tileBytes)/1e6,
				m.Tiles.MinZoom, m.Tiles.MaxZoom, m.Tiles.Width, m.Tiles.Height)
		}
		fmt.Printf("OK    %-60s %dx%d @%ddpi | %.2f m/px | fit %.2f m | scale %.4f,%.4f | %v\n",
			filepath.Base(f), m.Image.Width, m.Image.Height, m.Image.DPI,
			m.Georef.MetersPerPixel, m.Georef.ErrorMeters, m.Scale.X, m.Scale.Y,
			time.Since(start).Round(time.Millisecond))
	}
	if failed > 0 {
		os.Exit(1)
	}
}
