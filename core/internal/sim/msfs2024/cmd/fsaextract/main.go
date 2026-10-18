// fsaextract writes to a folder the records of one section type of the BGL files of an archive that
// mention a text, one file each, so that their layout can be studied. It is a probe: it only reads
// the simulator's files, and what it writes is for the developer's own analysis (never committed).
//
//	fsaextract -file minimal.fsarchive -section 0x22 -text UZ84 -out dir -max 5
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/bgl"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/fsarchive"
)

func main() {
	file := flag.String("file", "", "the .fsarchive to read")
	sec := flag.Uint("section", 0x22, "section type")
	text := flag.String("text", "", "ASCII text the record must contain")
	out := flag.String("out", "", "folder to write the records to")
	packed := flag.String("packed", "", "instead of text: a fix name, found by its packed identifier (any class)")
	maxN := flag.Int("max", 5, "how many records to write")
	flag.Parse()
	a, err := fsarchive.Open(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer a.Close()
	os.MkdirAll(*out, 0o755)
	var pats [][]byte
	if *packed != "" {
		var v uint64
		for _, c := range strings.ToUpper(*packed) {
			v = v*38 + uint64(strings.IndexRune(" ?0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ", c))
		}
		for _, cls := range []uint64{2, 3, 5, 6} {
			p := make([]byte, 8)
			binary.LittleEndian.PutUint64(p, v<<6|cls)
			pats = append(pats, p)
		}
	}
	n := 0
	for _, e := range a.Entries {
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".bgl") {
			continue
		}
		data, err := a.Read(e)
		if err != nil {
			continue
		}
		f, err := bgl.Parse(data)
		if err != nil {
			continue
		}
		for _, s := range f.Sections {
			if uint(s.Type) != *sec {
				continue
			}
			for si, sub := range s.Subsections {
				for ri, r := range sub.Records {
					b := f.Bytes(r)
					found := *packed == "" && bytes.Contains(b, []byte(*text))
					for _, p := range pats {
						found = found || bytes.Contains(b, p)
					}
					if !found {
						continue
					}
					name := fmt.Sprintf("%s_s%d_r%d_id%04x.bin", filepath.Base(e.Name()), si, ri, r.ID)
					os.WriteFile(filepath.Join(*out, name), b, 0o644)
					fmt.Printf("%s: %d bytes, record id 0x%04x\n", name, len(b), r.ID)
					if n++; n >= *maxN {
						return
					}
				}
			}
		}
	}
}
