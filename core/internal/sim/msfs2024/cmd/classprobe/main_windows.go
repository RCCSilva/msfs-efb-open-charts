// classprobe finds which fix class of the simulator's files is a waypoint, a VOR or an NDB: it reads
// the class of each name from the files, asks a sample of the names to the running simulator the
// current way (three requests each) and counts, for each class, the kinds the simulator answered.
//
//	classprobe -dll SimConnect.dll -per 120
package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/navseed"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/simconnect"
)

// covers tells whether every kind the simulator answered was among the kinds that were asked.
func covers(asked string, got []string) bool {
	for _, k := range got {
		if !strings.Contains(asked, k) {
			return false
		}
	}
	return true
}

func main() {
	dll := flag.String("dll", "", "path to SimConnect.dll")
	per := flag.Int("per", 120, "names to ask for each class")
	check := flag.Int("check", 0, "instead, check the class-to-kind rule on this many names across all classes")
	bench := flag.Int("bench", 0, "instead, time this many names asked as three kinds and by class")
	flag.Parse()

	paths := navseed.Locate()
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "no simulator packages found")
		os.Exit(1)
	}
	seeds, err := navseed.Read(paths, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// every name with the classes the files give it
	classes := map[string]map[int]bool{}
	byClass := map[int][]string{}
	for _, s := range seeds {
		classes[s.Ident] = map[int]bool{}
		for c := 0; c < 64; c++ {
			if s.Classes&(1<<c) != 0 {
				classes[s.Ident][c] = true
				byClass[c] = append(byClass[c], s.Ident)
			}
		}
	}
	var keys []int
	for k := range byClass {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	kindName := map[byte]string{navdata.FixWaypoint: "waypoint", navdata.FixVOR: "vor", navdata.FixNDB: "ndb"}
	classKind := map[int]string{2: "vor", 3: "ndb", 5: "waypoint", 6: "waypoint"}
	predict := func(n string) string {
		// the files do not hold every fix of a name: a short name that is a VOR or an NDB somewhere is
		// often also a waypoint of another region, so these are asked as all three
		if len(n) <= 3 || classes[n][2] || classes[n][3] {
			return "ndb+vor+waypoint"
		}
		set := map[string]bool{}
		for c := range classes[n] {
			set[classKind[c]] = true
		}
		var ks []string
		for k := range set {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		return strings.Join(ks, "+")
	}
	if *bench > 0 {
		all := make([]string, 0, len(classes))
		for n := range classes {
			all = append(all, n)
		}
		sort.Strings(all)
		step := max(1, len(all)/(*bench))
		var sample []string
		for i := 3; i < len(all) && len(sample) < *bench; i += step {
			sample = append(sample, all[i])
		}
		var asks []navdata.FixRef
		for _, n := range sample {
			mask := 0
			for c := range classes[n] {
				mask |= 1 << c
			}
			for _, k := range navdb.KindsToAsk(n, mask) {
				asks = append(asks, navdata.FixRef{Ident: n, Kind: k})
			}
		}
		const call = 40
		run := func(name string, f func(part []string, from, to int) ([]navdata.Fix, error)) {
			start, found := time.Now(), 0
			for i := 0; i < len(sample); i += call {
				j := min(len(sample), i+call)
				got, err := f(sample[i:j], i, j)
				if err != nil {
					fmt.Fprintln(os.Stderr, err)
					os.Exit(1)
				}
				found += len(got)
			}
			el := time.Since(start)
			fmt.Printf("%-10s %d names: %s (%.1f ms per name), %d fixes found\n", name, len(sample), el.Round(time.Millisecond), float64(el.Microseconds())/1000/float64(len(sample)), found)
		}
		run("three", func(part []string, _, _ int) ([]navdata.Fix, error) {
			return simconnect.LookupFixes(*dll, part, 3*time.Minute)
		})
		run("by class", func(part []string, _, _ int) ([]navdata.Fix, error) {
			var a []navdata.FixRef
			for _, r := range asks {
				for _, n := range part {
					if r.Ident == n {
						a = append(a, r)
						break
					}
				}
			}
			return simconnect.LookupFixKinds(*dll, a, 3*time.Minute)
		})
		fmt.Printf("%d requests by class, %d with three\n", len(asks), 3*len(sample))
		return
	}
	if *check > 0 {
		all := make([]string, 0, len(classes))
		for n := range classes {
			all = append(all, n)
		}
		sort.Strings(all)
		asks, requests := 0, 0
		for _, n := range all {
			asks++
			requests += strings.Count(predict(n), "+") + 1
		}
		fmt.Printf("%d names: %d requests with the classes, %d the current way\n", asks, requests, asks*3)
		step := max(1, len(all)/(*check))
		var sample []string
		for i := 0; i < len(all) && len(sample) < *check; i += step {
			sample = append(sample, all[i])
		}
		fixes, err := simconnect.LookupFixes(*dll, sample, 10*time.Minute)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		got := map[string]map[string]bool{}
		for _, f := range fixes {
			if got[f.Ident] == nil {
				got[f.Ident] = map[string]bool{}
			}
			got[f.Ident][kindName[f.Kind]] = true
		}
		same, diff := 0, 0
		for _, n := range sample {
			var ks []string
			for k := range got[n] {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			if a := strings.Join(ks, "+"); a == "" || strings.Contains(predict(n), a) || covers(predict(n), ks) {
				same++
			} else {
				diff++
				if diff <= 15 {
					fmt.Printf("  differs: %s files=%q simulator=%q (classes %v)\n", n, predict(n), a, classes[n])
				}
			}
		}
		fmt.Printf("checked %d names: %d the same, %d different\n", len(sample), same, diff)
		return
	}
	for _, c := range keys {
		names := byClass[c]
		step := max(1, len(names)/(*per))
		var sample []string
		for i := 0; i < len(names) && len(sample) < *per; i += step {
			sample = append(sample, names[i])
		}
		start := time.Now()
		fixes, err := simconnect.LookupFixes(*dll, sample, 3*time.Minute)
		if err != nil {
			fmt.Fprintln(os.Stderr, "class", c, err)
			continue
		}
		got := map[string]map[string]bool{}
		for _, f := range fixes {
			if got[f.Ident] == nil {
				got[f.Ident] = map[string]bool{}
			}
			got[f.Ident][kindName[f.Kind]] = true
		}
		tally := map[string]int{}
		for _, n := range sample {
			var ks []string
			for k := range got[n] {
				ks = append(ks, k)
			}
			sort.Strings(ks)
			tally[strings.Join(ks, "+")]++
		}
		fmt.Printf("class %d: %d names in the files, %d asked (%s): %v\n", c, len(names), len(sample), time.Since(start).Round(time.Millisecond), tally)
	}
}
