// Package msfs2024 is what is specific to Microsoft Flight Simulator 2024: how the program reaches it
// (SimConnect), where it keeps its navigation files and how a provider for the rest of the program
// (package sim) is built from that.
package msfs2024

import (
	"context"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/efbapp"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/navseed"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/simconnect"
)

// How long each question to the simulator may take.
const (
	airportWait  = 30 * time.Second
	airportsWait = 120 * time.Second
	listWait     = 90 * time.Second
	fixesWait    = 30 * time.Second
	planWait     = 1500 * time.Millisecond
)

// New builds the provider of the simulator reached through the SimConnect library at dll. The
// position is read all the time (the connection is kept and reopened when the simulator comes and
// goes); the other questions open a connection each.
func New(ctx context.Context, dll string, logf func(string, ...any)) sim.Provider {
	return sim.Provider{
		ID:       "msfs2024",
		Name:     "Microsoft Flight Simulator 2024",
		Short:    "MSFS",
		Position: position{client: simconnect.New(ctx, dll, logf)},
		Nav:      nav{dll: dll},
		Seed:     seed{},
		Plans:    plans{dll: dll},
		App:      efbapp.Installer{},
	}
}

// position is the aircraft, as SimConnect reports it.
type position struct{ client *simconnect.Client }

func (p position) Position() sim.Position {
	st, detail := p.client.Status()
	pos := sim.Position{Source: "simconnect", Connected: st == simconnect.StateOpen || st == simconnect.StateData, Detail: detail}
	if s, ok := p.client.Latest(); ok && time.Since(s.At) < 5*time.Second {
		pos.Valid, pos.Lat, pos.Lon, pos.HeadingTrue = true, s.Lat, s.Lon, s.HeadingTrue
		pos.GroundSpeed, pos.AltitudeFt, pos.UpdatedAt = s.GroundSpeed, s.AltitudeFt, s.At
	}
	return pos
}

// nav asks the simulator for navigation data through SimConnect's facility API.
type nav struct{ dll string }

func (n nav) Airport(icao string) (*navdata.Airport, error) {
	return simconnect.FetchAirport(n.dll, icao, airportWait)
}

func (n nav) Airports(icaos []string) ([]sim.AirportResult, error) {
	res, err := simconnect.FetchAirports(n.dll, icaos, airportsWait)
	out := make([]sim.AirportResult, len(res))
	for i, r := range res {
		out[i] = sim.AirportResult{Airport: r.Airport, Err: r.Err}
	}
	return out, err
}

func (n nav) ListAirports() ([]sim.AirportRef, error) {
	list, err := simconnect.ListAirports(n.dll, listWait)
	out := make([]sim.AirportRef, 0, len(list))
	for _, a := range list {
		out = append(out, sim.AirportRef{ICAO: a.ICAO, Region: a.Region, Lat: a.Lat, Lon: a.Lon})
	}
	return out, err
}

func (n nav) LookupFixes(idents []string) ([]navdata.Fix, error) {
	return simconnect.LookupFixes(n.dll, idents, fixesWait)
}

func (n nav) LookupFixKinds(asks []navdata.FixRef) ([]navdata.Fix, error) {
	return simconnect.LookupFixKinds(n.dll, asks, fixesWait)
}

// seed reads the identifiers from the navigation packages the simulator streams to this computer.
type seed struct{}

func (seed) Locate() []string { return navseed.Locate() }

func (seed) Read(files []string, progress func(done, total int) bool) ([]navdb.SeedIdent, error) {
	return navseed.Read(files, progress)
}

// plans loads a flight plan file through SimConnect.
type plans struct{ dll string }

func (p plans) Load(file string) error { return simconnect.LoadFlightPlan(p.dll, file, planWait) }

// AppInstaller is the installer of the app inside the simulator, for when the rest of the provider
// is not needed (the installer's own commands do not talk to a running simulator).
func AppInstaller() sim.AppInstaller { return efbapp.Installer{} }
