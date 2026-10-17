// fsainspect lists what is inside a simulator package archive and, for the BGL files in it, which
// sections and record ids they hold. It is a probe for the navigation work: it only reads.
package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/bgl"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/fsarchive"
)

var digits = regexp.MustCompile(`\d+`)

func main() {
	file := flag.String("file", "", "the .fsarchive to read")
	flag.Parse()
	if *file == "" {
		flag.Usage()
		os.Exit(2)
	}
	a, err := fsarchive.Open(*file)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer a.Close()
	fmt.Printf("%d files\n", len(a.Entries))

	type stat struct {
		files, sections, subsections, records int
		recordIDs                             map[uint16]int
		recordBytes                           map[uint16]int64
		bad                                   int
	}
	byKind := map[string]*stat{} // by file name pattern (nax#.bgl...)
	sectionTypes := map[string]map[uint32]int{}
	for _, e := range a.Entries {
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".bgl") {
			continue
		}
		kind := digits.ReplaceAllString(path.Base(e.Name()), "#")
		st := byKind[kind]
		if st == nil {
			st = &stat{recordIDs: map[uint16]int{}, recordBytes: map[uint16]int64{}}
			byKind[kind] = st
			sectionTypes[kind] = map[uint32]int{}
		}
		st.files++
		data, err := a.Read(e)
		if err != nil {
			st.bad++
			continue
		}
		f, err := bgl.Parse(data)
		if err != nil {
			st.bad++
			if st.bad <= 2 {
				fmt.Printf("  %s: %v\n", e.Name(), err)
			}
			continue
		}
		for _, s := range f.Sections {
			st.sections++
			sectionTypes[kind][s.Type]++
			for _, sub := range s.Subsections {
				st.subsections++
				for _, r := range sub.Records {
					st.records++
					st.recordIDs[r.ID]++
					st.recordBytes[r.ID] += int64(r.Size)
				}
			}
		}
	}
	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		st := byKind[k]
		fmt.Printf("\n%s: %d files (%d unreadable), %d sections, %d subsections, %d records\n", k, st.files, st.bad, st.sections, st.subsections, st.records)
		var ts []uint32
		for t := range sectionTypes[k] {
			ts = append(ts, t)
		}
		sort.Slice(ts, func(i, j int) bool { return ts[i] < ts[j] })
		fmt.Print("  section types:")
		for _, t := range ts {
			fmt.Printf(" 0x%02x x%d", t, sectionTypes[k][t])
		}
		fmt.Println()
		var ids []uint16
		for id := range st.recordIDs {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return st.recordIDs[ids[i]] > st.recordIDs[ids[j]] })
		for i, id := range ids {
			if i >= 14 {
				break
			}
			fmt.Printf("  record 0x%04x: %7d records, %9d bytes (avg %d)\n", id, st.recordIDs[id], st.recordBytes[id], st.recordBytes[id]/int64(st.recordIDs[id]))
		}
	}
}
