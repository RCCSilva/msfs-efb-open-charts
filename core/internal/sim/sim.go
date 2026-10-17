// Package sim is what the rest of the program knows about a flight simulator.
//
// It declares what the program needs from a simulator, as small interfaces that the packages using
// them accept, and a Provider: a struct that gathers the ones a given simulator can fulfil. What is
// specific to a simulator lives in a package of its own (msfs2024, for now) that builds a Provider;
// nothing outside it knows how a simulator is reached. A field of the Provider that is nil is
// something the simulator cannot do, and the program offers less (see Capabilities).
package sim

import (
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

// Position is where the aircraft is.
type Position struct {
	Valid       bool
	Lat, Lon    float64
	HeadingTrue float64
	GroundSpeed float64
	AltitudeFt  float64
	Connected   bool   // the position source is reachable (always true for "fixed")
	Detail      string // why it is not connected, for the viewer
	Source      string // "fixed", "simconnect"
	UpdatedAt   time.Time
}

// PositionSource provides the current aircraft position.
type PositionSource interface {
	Position() Position
}

// PositionFunc makes a function a PositionSource.
type PositionFunc func() Position

// Position calls the function.
func (f PositionFunc) Position() Position { return f() }

type fixedPosition struct{ lat, lon, heading float64 }

func (f fixedPosition) Position() Position {
	return Position{Valid: true, Connected: true, Lat: f.lat, Lon: f.lon, HeadingTrue: f.heading, Source: "fixed", UpdatedAt: time.Now()}
}

// Fixed is a position that never changes: for trying the program without a simulator.
func Fixed(lat, lon, headingTrue float64) PositionSource {
	return fixedPosition{lat: lat, lon: lon, heading: headingTrue}
}

// AirportRef is one line of the simulator's list of airports.
type AirportRef struct {
	ICAO   string
	Region string
	Lat    float64
	Lon    float64
}

// AirportResult is the answer for one airport of NavSource.Airports.
type AirportResult struct {
	Airport *navdata.Airport
	Err     error // navdata.ErrUnknown when the simulator does not know it
}

// NavSource asks the simulator for its navigation data.
type NavSource interface {
	// Airport asks for one airport: position, runways, frequencies and procedures.
	Airport(icao string) (*navdata.Airport, error)
	// Airports asks for many airports at once: the answers are in the order of the codes.
	Airports(icaos []string) ([]AirportResult, error)
	// ListAirports lists every airport the simulator has.
	ListAirports() ([]AirportRef, error)
	// LookupFixes asks for waypoints, VORs and NDBs as a whole: every place with that identifier, in
	// every region.
	LookupFixes(idents []string) ([]navdata.Fix, error)
}

// SeedSource reads the identifiers of the navigation data from the files the simulator keeps on this
// computer (only the names and rough positions): the names that are then asked to the simulator.
type SeedSource interface {
	// Locate lists the files, empty when there are none.
	Locate() []string
	// Read reads them. progress is called now and then and stops the reading by returning false.
	Read(files []string, progress func(done, total int) bool) ([]navdb.SeedIdent, error)
}

// FlightPlanLoader asks the simulator to load a flight plan file.
type FlightPlanLoader interface {
	Load(file string) error
}

// Provider is what a simulator gives the program.
type Provider struct {
	ID       string // "msfs2024"
	Name     string // "Microsoft Flight Simulator 2024"
	Position PositionSource
	Nav      NavSource
	Seed     SeedSource
	Plans    FlightPlanLoader
}

// Capabilities says what the program can do with a simulator.
type Capabilities struct {
	LivePosition bool `json:"livePosition"`
	AskNav       bool `json:"askNav"`
	SeedFiles    bool `json:"seedFiles"`
	LoadPlans    bool `json:"loadPlans"`
}

// Capabilities derives what the provider can do from the fields it filled in.
func (p Provider) Capabilities() Capabilities {
	return Capabilities{
		LivePosition: p.Position != nil,
		AskNav:       p.Nav != nil,
		SeedFiles:    p.Seed != nil,
		LoadPlans:    p.Plans != nil,
	}
}
