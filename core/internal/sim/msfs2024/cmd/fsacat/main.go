// fsacat writes one file of a simulator package archive to a folder, so that it can be studied. A
// probe: it only reads the simulator's files; what it writes is for the developer's analysis only.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/fsarchive"
)

func main() {
	file := flag.String("file", "", "the .fsarchive to read")
	name := flag.String("name", "", "the entry to extract (a suffix of its path)")
	out := flag.String("out", "", "the folder to write to")
	flag.Parse()
	a, err := fsarchive.Open(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer a.Close()
	for _, e := range a.Entries {
		if strings.HasSuffix(e.Name(), *name) {
			b, err := a.Read(e)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			os.MkdirAll(*out, 0o755)
			dst := filepath.Join(*out, filepath.Base(e.Name()))
			os.WriteFile(dst, b, 0o644)
			fmt.Println(dst, len(b))
			return
		}
	}
	fmt.Fprintln(os.Stderr, "not found")
	os.Exit(1)
}
