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

// AirportRef is one line of the simulator's list of airports.
type AirportRef struct {
	ICAO   string
	Region string
	Lat    float64
	Lon    float64
}

// ListAirports needs the simulator's SimConnect library, which exists on Windows only.
func ListAirports(dllPath string, wait time.Duration) ([]AirportRef, error) {
	return nil, errUnsupported
}

// AirportResult is the answer for one airport of FetchAirports.
type AirportResult struct {
	Airport *navdata.Airport
	Err     error
}

// FetchAirports needs the simulator's SimConnect library, which exists on Windows only.
func FetchAirports(dllPath string, icaos []string, wait time.Duration) ([]AirportResult, error) {
	return nil, errUnsupported
}

// LoadFlightPlan needs the simulator's SimConnect library, which exists on Windows only.
func LoadFlightPlan(dllPath, file string, wait time.Duration) error { return errUnsupported }
