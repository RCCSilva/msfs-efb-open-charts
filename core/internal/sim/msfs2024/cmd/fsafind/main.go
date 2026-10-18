// fsafind looks for where an identifier is mentioned inside the BGL files of a simulator package
// archive, by record id. It is a probe for the navigation work (which records hold the airways, the
// procedures...): it only reads, and it prints no data of the simulator beyond ids and counts.
//
//	fsafind -file minimal.fsarchive BOBIC
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/bgl"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/fsarchive"
)

const base38 = " ?0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func pack38(s string) uint64 {
	var v uint64
	for _, c := range strings.ToUpper(s) {
		i := strings.IndexRune(base38, c)
		if i < 0 {
			i = 0
		}
		v = v*38 + uint64(i)
	}
	return v
}

func main() {
	file := flag.String("file", "", "the .fsarchive to read")
	dump := flag.Int("dump", 0, "print this many bytes of the first hit of each record id")
	flag.Parse()
	if *file == "" || flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	a, err := fsarchive.Open(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer a.Close()

	for _, ident := range flag.Args() {
		v := pack38(ident)
		// the identifier is stored as (packed << 6 | kind): the bytes above the first hold packed >> 2
		var pat [4]byte
		binary.LittleEndian.PutUint32(pat[:], uint32(v>>2))
		ascii := []byte(strings.ToUpper(ident))
		fmt.Printf("== %s (packed %d, pattern % x)\n", ident, v, pat)
		type key struct {
			sec  uint32
			rec  uint16
			what string
		}
		hits := map[key]int{}
		first := map[key][]byte{}
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
				for _, sub := range s.Subsections {
					for _, r := range sub.Records {
						b := f.Bytes(r)
						if bytes.Contains(b, pat[:]) {
							k := key{s.Type, r.ID, "packed"}
							hits[k]++
							if first[k] == nil {
								first[k] = b
							}
						}
						if bytes.Contains(b, ascii) {
							k := key{s.Type, r.ID, "ascii"}
							hits[k]++
							if first[k] == nil {
								first[k] = b
							}
						}
					}
				}
			}
		}
		var keys []key
		for k := range hits {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return hits[keys[i]] > hits[keys[j]] })
		for _, k := range keys {
			fmt.Printf("  section 0x%02x record 0x%04x %-6s %d records\n", k.sec, k.rec, k.what, hits[k])
			if *dump > 0 {
				b := first[k]
				fmt.Printf("    first record (%d bytes): % x\n", len(b), b[:min(len(b), *dump)])
			}
		}
	}
}
