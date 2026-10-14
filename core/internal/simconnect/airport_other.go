//go:build !windows

package simconnect

import (
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// LookupFixes needs the simulator's SimConnect library, which exists on Windows only.
func LookupFixes(dllPath string, idents []string, wait time.Duration) ([]navdata.Fix, error) {
	return nil, errUnsupported
}

// FetchFixes needs the simulator's SimConnect library, which exists on Windows only.
func FetchFixes(dllPath string, refs []navdata.FixRef, wait time.Duration) ([]navdata.Fix, error) {
	return nil, errUnsupported
}

// FetchAirport needs the simulator's SimConnect library, which exists on Windows only.
func FetchAirport(dllPath, icao string, wait time.Duration) (*navdata.Airport, error) {
	return nil, errUnsupported
}
