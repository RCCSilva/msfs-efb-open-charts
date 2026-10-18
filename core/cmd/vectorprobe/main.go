// vectorprobe extracts the paths of a georeferenced chart PDF as a vector file (proof of concept of
// drawing a chart as vectors in the browser).
//
//	vectorprobe file.pdf out.vec
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: vectorprobe file.pdf out.vec")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r, err := importer.NewRenderer()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pdfium:", err)
		os.Exit(1)
	}
	defer r.Close()
	t0 := time.Now()
	out, st, err := r.ExtractVectors(data)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.WriteFile(os.Args[2], out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%d bytes in, %d bytes out, extracted in %v\nobjects: %v\npaths kept: %d, skipped: %d, segments: %d, deepest form: %d\n",
		len(data), len(out), time.Since(t0).Round(time.Millisecond), st.Objects, st.Paths, st.Skipped, st.Segments, st.MaxDepth)
}
