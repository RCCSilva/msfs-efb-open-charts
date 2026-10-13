//go:build windows

// navprobe asks the running simulator for navigation data through the SimConnect facility API and
// prints what comes back. It is a probe for the route-management work, not part of the product.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/simconnect"
)

func main() {
	dll := flag.String("dll", "", "path to SimConnect.dll")
	icao := flag.String("icao", "", "airport to ask for (SBSP, KJFK...)")
	list := flag.String("list", "", "count all: airport, waypoint, ndb or vor")
	fix := flag.String("fix", "", "waypoint/VOR/NDB identifier (RENIT, SPL...)")
	region := flag.String("region", "", "region code of the fix (SB, K1...)")
	kind := flag.String("kind", "W", "W waypoint, V vor, N ndb")
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
