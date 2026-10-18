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
	// LookupFixKinds is LookupFixes when the kinds to ask for each identifier are known: each ask is
	// an identifier and one kind (no region), and only those are put to the simulator.
	LookupFixKinds(asks []navdata.FixRef) ([]navdata.Fix, error)
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

// AppState is the state of the program's app inside one installation of the simulator.
type AppState struct {
	Installed bool `json:"installed"`
	Legacy    bool `json:"legacy"` // an older package of the app is present
	// ServerURL is the address written into the installed app. If it differs from where the
	// server really is (the port changed), the app cannot reach it until it is updated.
	ServerURL   string `json:"serverURL,omitempty"`
	URLMismatch bool   `json:"urlMismatch"`
	Version     string `json:"version,omitempty"`
	Build       string `json:"build,omitempty"` // what the installed app says it is: "0.1.1" or "0.1.1-8fa9b5b"
	UpToDate    bool   `json:"upToDate"`
	Path        string `json:"path"`
}

// AppInstallation is one place where the simulator is installed and the app can be put.
type AppInstallation struct {
	Edition       string   `json:"edition"` // "steam" | "store"
	PackagesPath  string   `json:"packagesPath"`
	Target        string   `json:"target"`        // where the app goes (for MSFS 2024, the Community folder)
	AutoStartFile string   `json:"autoStartFile"` // the file that starts the program with the simulator
	App           AppState `json:"app"`
	AutoStart     bool     `json:"autoStart"`
}

// AppInstaller puts the program's app inside the simulator (for MSFS 2024, the EFB app) and has the
// program start with the simulator. Nothing runs unless the user asks.
type AppInstaller interface {
	// Detect lists the installations of the simulator that can be found on this computer.
	Detect(exePath string) []AppInstallation
	// ValidTarget checks that a folder is one the app can be put in.
	ValidTarget(target string) error
	// Install puts the app in target, told to reach the program at serverURL.
	Install(target, serverURL string) error
	// Remove takes the app out of target.
	Remove(target string) error
	// State says whether the app is in target and whether it is current.
	State(target string) AppState
	// InstalledServerURL is the address written into the app installed in target ("" when none).
	InstalledServerURL(target string) string
	// AutoStart says whether the program starts with the simulator, by the given file.
	AutoStart(file string) bool
	// SetAutoStart makes the program start (or not) with the simulator. args go after the program.
	SetAutoStart(file, exePath string, args []string, enable bool) error
}

// Provider is what a simulator gives the program.
type Provider struct {
	ID       string // "msfs2024"
	Name     string // "Microsoft Flight Simulator 2024"
	Short    string // "MSFS": for the little places, such as the tray
	Position PositionSource
	Nav      NavSource
	Seed     SeedSource
	Plans    FlightPlanLoader
	App      AppInstaller
}

// Capabilities says what the program can do with a simulator.
type Capabilities struct {
	LivePosition bool `json:"livePosition"`
	AskNav       bool `json:"askNav"`
	SeedFiles    bool `json:"seedFiles"`
	LoadPlans    bool `json:"loadPlans"`
	InSimApp     bool `json:"inSimApp"`
}

// Capabilities derives what the provider can do from the fields it filled in.
func (p Provider) Capabilities() Capabilities {
	return Capabilities{
		LivePosition: p.Position != nil,
		AskNav:       p.Nav != nil,
		SeedFiles:    p.Seed != nil,
		LoadPlans:    p.Plans != nil,
		InSimApp:     p.App != nil,
	}
}
