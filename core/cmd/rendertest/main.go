// rendertest: spike. Render the first page of each PDF with PDFium (WebAssembly, no cgo),
// write a PNG next to it, and compare against a reference PNG rendered by MuPDF.
//
//	go run ./cmd/rendertest <dir> <dpi> <name1> <name2> ...
package main

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

func main() {
	if len(os.Args) < 4 {
		log.Fatal("usage: rendertest <dir> <dpi> <base-name>...")
	}
	dir := os.Args[1]
	var dpi int
	fmt.Sscanf(os.Args[2], "%d", &dpi)
	names := os.Args[3:]

	t0 := time.Now()
	pool, err := webassembly.Init(webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1})
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	instance, err := pool.GetInstance(30 * time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer instance.Close()
	fmt.Printf("pdfium (wasm) ready in %v\n", time.Since(t0).Round(time.Millisecond))

	for _, base := range names {
		pdfBytes, err := os.ReadFile(filepath.Join(dir, base+".pdf"))
		if err != nil {
			log.Fatal(err)
		}

		start := time.Now()
		doc, err := instance.OpenDocument(&requests.OpenDocument{File: &pdfBytes})
		if err != nil {
			log.Fatalf("%s: open: %v", base, err)
		}
		opened := time.Since(start)

		rendered, err := instance.RenderPageInDPI(&requests.RenderPageInDPI{
			DPI: dpi,
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{Document: doc.Document, Index: 0},
			},
		})
		if err != nil {
			log.Fatalf("%s: render: %v", base, err)
		}
		total := time.Since(start)

		img := rendered.Result.Image
		out := filepath.Join(dir, fmt.Sprintf("%s.%ddpi.pdfium-go.png", base, dpi))
		f, err := os.Create(out)
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			log.Fatal(err)
		}
		f.Close()
		st, _ := os.Stat(out)

		diff := "n/a (no MuPDF reference)"
		refPath := filepath.Join(dir, fmt.Sprintf("%s.%ddpi.mupdfjs.png", base, dpi))
		if rf, err := os.Open(refPath); err == nil {
			ref, err := png.Decode(rf)
			rf.Close()
			if err == nil {
				diff = compare(img, ref)
			}
		}
		fmt.Printf("%s\n   %dx%d | open %v, total %v | PNG %d KB | vs MuPDF: %s\n",
			base, rendered.Result.Width, rendered.Result.Height,
			opened.Round(time.Millisecond), total.Round(time.Millisecond), st.Size()/1024, diff)

		instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})
		rendered.Cleanup()
	}
}

func compare(a *image.RGBA, b image.Image) string {
	bb := b.Bounds()
	w := min(a.Bounds().Dx(), bb.Dx())
	h := min(a.Bounds().Dy(), bb.Dy())
	var sum float64
	var big, n int
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bbv, _ := b.At(x+bb.Min.X, y+bb.Min.Y).RGBA()
			d := (abs(int(ar>>8)-int(br>>8)) + abs(int(ag>>8)-int(bg>>8)) + abs(int(ab>>8)-int(bbv>>8))) / 3
			sum += float64(d)
			if d > 64 {
				big++
			}
			n++
		}
	}
	return fmt.Sprintf("size %dx%d vs %dx%d | meanAbs %.2f | differing>64: %.2f%%",
		a.Bounds().Dx(), a.Bounds().Dy(), bb.Dx(), bb.Dy(), sum/float64(n), 100*float64(big)/float64(n))
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

var _ = pdfium.Pool(nil)
