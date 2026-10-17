//go:build windows

// navprobe asks the running simulator for navigation data through the SimConnect facility API and
// prints what comes back. It is a probe for the route-management work, not part of the product.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/simconnect"
)

func main() {
	dll := flag.String("dll", "", "path to SimConnect.dll")
	icao := flag.String("icao", "", "airport to ask for (SBSP, KJFK...)")
	list := flag.String("list", "", "count all: airport, waypoint, ndb or vor")
	fix := flag.String("fix", "", "waypoint/VOR/NDB identifier (RENIT, SPL...)")
	region := flag.String("region", "", "region code of the fix (SB, K1...)")
	kind := flag.String("kind", "W", "W waypoint, V vor, N ndb")
	procs := flag.String("procs", "", "airport whose departures and arrivals are printed with every leg")
	lookup := flag.String("lookup", "", "identifier(s) to find as a whole, separated by commas (every region, every kind)")
	names := flag.String("names", "", "OPEN <type>:FIELD,FIELD,... : report which field names are rejected")
	wait := flag.Duration("wait", 60*time.Second, "how long to wait")
	flag.Parse()

	path, err := simconnect.FindDLL(*dll, "")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("using", path)
	switch {
	case *icao != "":
		out, err := simconnect.ProbeAirport(path, *icao, *wait)
		fmt.Print(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case *lookup != "":
		fixes, err := simconnect.LookupFixes(path, strings.Split(*lookup, ","), *wait)
		for _, f := range fixes {
			fmt.Printf("%-8s region=%-3q kind=%c  %.4f %.4f  routes=%d  %s\n", f.Ident, f.Region, f.Kind, f.Lat, f.Lon, len(f.Routes), f.Name)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case *procs != "":
		out, err := simconnect.ProbeProcedures(path, *procs, *wait)
		fmt.Print(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case *names != "":
		open, rest, _ := strings.Cut(*names, ":")
		out, err := simconnect.ProbeNames(path, open, strings.Split(rest, ","), *wait)
		fmt.Print(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case *fix != "":
		out, err := simconnect.ProbeFix(path, *fix, *region, (*kind)[0], *wait)
		fmt.Print(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	case *list != "":
		out, err := simconnect.ProbeList(path, *list, *wait)
		fmt.Print(out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
	default:
		flag.Usage()
	}
}
