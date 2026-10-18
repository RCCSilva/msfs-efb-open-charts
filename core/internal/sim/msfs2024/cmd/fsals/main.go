// fsals lists the files inside a simulator package archive, with their sizes. A probe: it only reads.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/fsarchive"
)

func main() {
	file := flag.String("file", "", "the .fsarchive to read")
	flag.Parse()
	a, err := fsarchive.Open(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer a.Close()
	for _, e := range a.Entries {
		fmt.Printf("%12d  %s\n", e.UncompressedSize, e.Name())
	}
}
