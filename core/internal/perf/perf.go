// Package perf holds the performance profile of an aircraft (how fast it climbs, cruises and
// descends, and how much fuel it burns doing it), reads the profiles Little Navmap writes, and
// estimates the time and the fuel of a route from one.
//
// The estimate is a planning aid for simulation: still air, constant speeds and rates for each
// phase, the way the profile states them.
package perf

import (
	"errors"
	"fmt"
	"strings"
)

// Fuel units a profile can use. A profile keeps its fuel in the unit it was written in, so a file
// read and shown again says what it said.
const (
	Gallons = "gal"
	Pounds  = "lb"
)

// Pounds per US gallon, the values Little Navmap uses.
const (
	avgasLbPerGal = 6.0
	jetLbPerGal   = 6.7
)

// Phase is the speed and the fuel burn of one phase of the flight.
type Phase struct {
	SpeedKt  float64 `json:"speedKt"`           // true airspeed
	FuelFlow float64 `json:"fuelFlow"`          // fuel units per hour
	VertFpm  float64 `json:"vertFpm,omitempty"` // feet per minute (climb and descent only)
}

// Profile is the performance of one aircraft.
type Profile struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	AircraftType string `json:"aircraftType,omitempty"` // ICAO type, "C172"
	Simulator    string `json:"simulator,omitempty"`
	Description  string `json:"description,omitempty"`
	FuelUnit     string `json:"fuelUnit"` // gal or lb
	Jet          bool   `json:"jet"`      // jet fuel (heavier per gallon than avgas)

	UsableFuel  float64 `json:"usableFuel"`
	TaxiFuel    float64 `json:"taxiFuel"`
	ReserveFuel float64 `json:"reserveFuel"`
	ExtraFuel   float64 `json:"extraFuel"`
	// ContingencyPct is extra fuel as a percentage of the trip fuel.
	ContingencyPct float64 `json:"contingencyPct"`

	MinRunwayFt float64 `json:"minRunwayFt,omitempty"`
	RunwayType  string  `json:"runwayType,omitempty"` // hard, soft, water

	Climb     Phase `json:"climb"`
	Cruise    Phase `json:"cruise"`
	Descent   Phase `json:"descent"`
	Alternate Phase `json:"alternate"`
}

// ErrInvalid wraps every complaint about a profile.
var ErrInvalid = errors.New("invalid profile")

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Clean trims the text and checks that the profile can be used to estimate a flight.
func (p *Profile) Clean() error {
	p.Name = strings.TrimSpace(p.Name)
	p.AircraftType = strings.ToUpper(strings.TrimSpace(p.AircraftType))
	p.Simulator = strings.TrimSpace(p.Simulator)
	p.Description = strings.TrimSpace(p.Description)
	p.RunwayType = strings.ToLower(strings.TrimSpace(p.RunwayType))
	switch {
	case p.Name == "":
		return invalid("the aircraft needs a name")
	case len([]rune(p.Name)) > 80:
		return invalid("the name is too long (80 characters at most)")
	case len(p.Description) > 4000:
		return invalid("the description is too long")
	case p.FuelUnit != Gallons && p.FuelUnit != Pounds:
		return invalid("the fuel unit must be gal or lb")
	case p.Cruise.SpeedKt <= 0:
		return invalid("the cruise speed must be above zero")
	case p.Climb.SpeedKt <= 0 || p.Climb.VertFpm <= 0:
		return invalid("the climb speed and rate must be above zero")
	case p.Descent.SpeedKt <= 0 || p.Descent.VertFpm <= 0:
		return invalid("the descent speed and rate must be above zero")
	}
	for _, v := range []float64{p.UsableFuel, p.TaxiFuel, p.ReserveFuel, p.ExtraFuel, p.ContingencyPct, p.MinRunwayFt,
		p.Climb.FuelFlow, p.Cruise.FuelFlow, p.Descent.FuelFlow, p.Alternate.FuelFlow, p.Alternate.SpeedKt} {
		if v < 0 {
			return invalid("no value can be negative")
		}
	}
	return nil
}

// LbPerUnit is how many pounds one unit of the profile's fuel weighs.
func (p Profile) LbPerUnit() float64 {
	switch {
	case p.FuelUnit == Pounds:
		return 1
	case p.Jet:
		return jetLbPerGal
	}
	return avgasLbPerGal
}
