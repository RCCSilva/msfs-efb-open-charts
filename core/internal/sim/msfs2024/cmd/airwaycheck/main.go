// airwaycheck compares the airways read from the simulator's files with the ones the running app has
// stored from SimConnect, for the airways named: for each it says how many pairs of neighbouring
// fixes only the files have, only the app has, and both have. It is a probe for the work of reading
// the airways from the files.
//
//	airwaycheck -app http://127.0.0.1:43210 UZ84 UZ29 Z36
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/navseed"
)

func pair(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "-" + b
}

func main() {
	app := flag.String("app", "http://127.0.0.1:43210", "address of the running app")
	namesFile := flag.String("names", "", "a file with the airway names, separated by spaces")
	quiet := flag.Bool("q", false, "print only the totals and the airways that differ")
	show := flag.String("fix", "", "print the fixes of the files with this name, and stop")
	flag.Parse()
	names := flag.Args()
	if *namesFile != "" {
		b, err := os.ReadFile(*namesFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		names = strings.Fields(string(b))
	}
	fixes, err := navseed.ReadFixes(navseed.Locate(), nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	{
		type key struct {
			id, region string
			class      int
		}
		names, keys := map[string]bool{}, map[key]bool{}
		for _, f := range fixes {
			names[f.Ident] = true
			keys[key{f.Ident, f.Region, f.Class}] = true
		}
		fmt.Printf("%d distinct names, %d distinct fixes (name, region, class)\n", len(names), len(keys))
	}
	if *show != "" {
		for _, f := range fixes {
			if f.Ident == *show {
				fmt.Printf("%+v\n", f)
			}
		}
		return
	}
	fmt.Printf("%d fixes read from the files\n", len(fixes))
	fromFiles := map[string]map[string]bool{}
	known := map[string]bool{}
	for _, f := range fixes {
		known[f.Ident] = true
	}
	unknownEnds, knownEnds := 0, 0
	missing := map[string]int{}
	for _, f := range fixes {
		for _, a := range f.Airways {
			if fromFiles[a.Name] == nil {
				fromFiles[a.Name] = map[string]bool{}
			}
			if a.HasNext {
				fromFiles[a.Name][pair(f.Ident, a.Next.Ident)] = true
			}
			if a.HasPrev {
				fromFiles[a.Name][pair(f.Ident, a.Prev.Ident)] = true
			}
		}
	}
	fmt.Printf("%d airways in the files\n", len(fromFiles))
	totalBoth, totalFiles, totalApp := 0, 0, 0
	for _, name := range names {
		resp, err := http.Get(*app + "/api/nav/airway/" + name)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		var body struct {
			Segments []struct{ From, To string }
		}
		json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		inApp := map[string]bool{}
		for _, s := range body.Segments {
			inApp[pair(s.From, s.To)] = true
		}
		var onlyFiles, onlyApp []string
		both := 0
		for p := range fromFiles[name] {
			if inApp[p] {
				both++
			} else {
				onlyFiles = append(onlyFiles, p)
			}
		}
		for p := range inApp {
			if !fromFiles[name][p] {
				onlyApp = append(onlyApp, p)
				a, b, _ := strings.Cut(p, "-")
				if known[a] && known[b] {
					knownEnds++
				} else {
					unknownEnds++
					for _, id := range []string{a, b} {
						if !known[id] {
							missing[id]++
						}
					}
				}
			}
		}
		sort.Strings(onlyFiles)
		sort.Strings(onlyApp)
		totalBoth += both
		totalFiles += len(onlyFiles)
		totalApp += len(onlyApp)
		if *quiet && len(onlyFiles) == 0 && len(onlyApp) == 0 {
			continue
		}
		fmt.Printf("%-8s both %4d  only files %3d  only app %3d", name, both, len(onlyFiles), len(onlyApp))
		if len(onlyFiles) > 0 {
			fmt.Printf("  files: %s", strings.Join(onlyFiles[:min(3, len(onlyFiles))], " "))
		}
		if len(onlyApp) > 0 {
			fmt.Printf("  app: %s", strings.Join(onlyApp[:min(3, len(onlyApp))], " "))
		}
		fmt.Println()
	}
	var ms []string
	for id := range missing {
		ms = append(ms, id)
	}
	sort.Slice(ms, func(i, j int) bool { return missing[ms[i]] > missing[ms[j]] })
	fmt.Printf("%d fixes the files lack, the most cited: %v\n", len(ms), ms[:min(40, len(ms))])
	fmt.Printf("pairs only in the app: %d have both fixes in the files, %d have a fix the files lack\n", knownEnds, unknownEnds)
	fmt.Printf("total: both %d, only files %d, only app %d\n", totalBoth, totalFiles, totalApp)
}
