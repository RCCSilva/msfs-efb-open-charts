package perf

import (
	"math"
)

// Flight is what is asked of an estimate: a route, with the altitude to cruise at.
type Flight struct {
	// LegsNm is the length of each leg of the route in nautical miles, as route.Legs gives them:
	// the first is 0 (the leg that arrives at the first point).
	LegsNm []float64
	// CruiseFt is the cruise altitude.
	CruiseFt float64
	// DepartureFt and DestinationFt are the elevations of the airports (0 when not known).
	DepartureFt, DestinationFt float64
	// AlternateNm is the distance to the alternate airport, 0 for none.
	AlternateNm float64
	// WindKt is the wind along the route: positive is a tailwind, negative a headwind. One value
	// for the whole flight and every altitude.
	WindKt float64
}

// PhaseResult is one phase of the flight.
type PhaseResult struct {
	Name    string  `json:"name"` // climb, cruise, descent
	Nm      float64 `json:"nm"`
	Minutes float64 `json:"minutes"`
	Fuel    float64 `json:"fuel"`
	FromFt  float64 `json:"fromFt"`
	ToFt    float64 `json:"toFt"`
}

// LegResult is the time and the fuel of one leg of the route (the leg that arrives at point i).
type LegResult struct {
	Nm      float64 `json:"nm"`
	Minutes float64 `json:"minutes"`
	Fuel    float64 `json:"fuel"`
}

// Result is the estimate of a flight. Fuel is in the profile's unit.
type Result struct {
	FuelUnit string        `json:"fuelUnit"`
	Phases   []PhaseResult `json:"phases"`
	Legs     []LegResult   `json:"legs"`
	TotalNm  float64       `json:"totalNm"`
	// CruiseFt is the altitude used. It is lower than the one asked when the route is too short
	// to climb and descend (Truncated).
	CruiseFt  float64 `json:"cruiseFt"`
	Truncated bool    `json:"truncated"`
	// TopOfClimbNm and TopOfDescentNm are measured from the start of the route.
	TopOfClimbNm   float64 `json:"topOfClimbNm"`
	TopOfDescentNm float64 `json:"topOfDescentNm"`

	Minutes     float64 `json:"minutes"`
	TripFuel    float64 `json:"tripFuel"`
	TaxiFuel    float64 `json:"taxiFuel"`
	Contingency float64 `json:"contingency"`
	Reserve     float64 `json:"reserve"`
	Extra       float64 `json:"extra"`
	Alternate   float64 `json:"alternate"`
	// Required is everything to carry: taxi, trip, contingency, alternate, reserve and extra.
	Required    float64 `json:"required"`
	RequiredLb  float64 `json:"requiredLb"`
	Usable      float64 `json:"usable"`
	EnoughFuel  bool    `json:"enoughFuel"`
	EnduranceHr float64 `json:"enduranceHr"` // hours the usable fuel lasts at the cruise burn
}

func round(v float64, places int) float64 {
	m := math.Pow(10, float64(places))
	return math.Round(v*m) / m
}

// Estimate works out the time and the fuel of the flight with the profile.
func (p Profile) Estimate(f Flight) (Result, error) {
	if err := p.Clean(); err != nil {
		return Result{}, err
	}
	if f.CruiseFt <= 0 {
		return Result{}, invalid("the cruise altitude must be above zero")
	}
	gsClimb, gsCruise, gsDescent := p.Climb.SpeedKt+f.WindKt, p.Cruise.SpeedKt+f.WindKt, p.Descent.SpeedKt+f.WindKt
	if gsClimb <= 0 || gsCruise <= 0 || gsDescent <= 0 {
		return Result{}, invalid("the headwind is as strong as the aircraft's speed")
	}
	total := 0.0
	for _, l := range f.LegsNm {
		total += l
	}

	// ground distance per foot of climb or descent
	perFtUp := gsClimb / 60 / p.Climb.VertFpm
	perFtDown := gsDescent / 60 / p.Descent.VertFpm
	alt := f.CruiseFt
	truncated := false
	need := func(h float64) float64 {
		return math.Max(h-f.DepartureFt, 0)*perFtUp + math.Max(h-f.DestinationFt, 0)*perFtDown
	}
	if need(alt) > total {
		// too short to reach the altitude and come down from it: the highest the route allows
		truncated = true
		alt = (total + f.DepartureFt*perFtUp + f.DestinationFt*perFtDown) / (perFtUp + perFtDown)
		alt = math.Max(alt, math.Max(f.DepartureFt, f.DestinationFt))
	}
	climbFt := math.Max(alt-f.DepartureFt, 0)
	descentFt := math.Max(alt-f.DestinationFt, 0)
	climbNm := math.Min(climbFt*perFtUp, total)
	descentNm := math.Min(descentFt*perFtDown, total-climbNm)
	cruiseNm := total - climbNm - descentNm

	phases := []PhaseResult{
		{Name: "climb", Nm: climbNm, Minutes: climbNm / gsClimb * 60, FromFt: f.DepartureFt, ToFt: alt},
		{Name: "cruise", Nm: cruiseNm, Minutes: cruiseNm / gsCruise * 60, FromFt: alt, ToFt: alt},
		{Name: "descent", Nm: descentNm, Minutes: descentNm / gsDescent * 60, FromFt: alt, ToFt: f.DestinationFt},
	}
	flows := []float64{p.Climb.FuelFlow, p.Cruise.FuelFlow, p.Descent.FuelFlow}
	speeds := []float64{gsClimb, gsCruise, gsDescent}
	r := Result{FuelUnit: p.FuelUnit, TotalNm: round(total, 1), CruiseFt: round(alt, 0), Truncated: truncated,
		TopOfClimbNm: round(climbNm, 1), TopOfDescentNm: round(total-descentNm, 1)}
	for i := range phases {
		phases[i].Fuel = phases[i].Minutes / 60 * flows[i]
		r.Minutes += phases[i].Minutes
		r.TripFuel += phases[i].Fuel
	}

	// each leg is cut where the phases change
	bounds := []float64{climbNm, climbNm + cruiseNm, total}
	pos := 0.0
	r.Legs = make([]LegResult, len(f.LegsNm))
	for i, l := range f.LegsNm {
		from, to := pos, pos+l
		for ph := range phases {
			lo := math.Max(from, []float64{0, bounds[0], bounds[1]}[ph])
			hi := math.Min(to, bounds[ph])
			if hi <= lo {
				continue
			}
			mins := (hi - lo) / speeds[ph] * 60
			r.Legs[i].Minutes += mins
			r.Legs[i].Fuel += mins / 60 * flows[ph]
		}
		r.Legs[i].Nm = round(l, 1)
		r.Legs[i].Minutes = round(r.Legs[i].Minutes, 1)
		r.Legs[i].Fuel = round(r.Legs[i].Fuel, 2)
		pos = to
	}

	for i := range phases {
		phases[i].Nm = round(phases[i].Nm, 1)
		phases[i].Minutes = round(phases[i].Minutes, 1)
		phases[i].Fuel = round(phases[i].Fuel, 2)
		phases[i].FromFt, phases[i].ToFt = round(phases[i].FromFt, 0), round(phases[i].ToFt, 0)
	}
	r.Phases = phases
	r.Contingency = r.TripFuel * p.ContingencyPct / 100
	if f.AlternateNm > 0 && p.Alternate.SpeedKt > 0 {
		r.Alternate = f.AlternateNm / p.Alternate.SpeedKt * p.Alternate.FuelFlow
	}
	r.TaxiFuel, r.Reserve, r.Extra, r.Usable = p.TaxiFuel, p.ReserveFuel, p.ExtraFuel, p.UsableFuel
	r.Required = r.TaxiFuel + r.TripFuel + r.Contingency + r.Alternate + r.Reserve + r.Extra
	r.RequiredLb = round(r.Required*p.LbPerUnit(), 0)
	r.EnoughFuel = p.UsableFuel == 0 || r.Required <= p.UsableFuel
	if p.Cruise.FuelFlow > 0 {
		r.EnduranceHr = round(p.UsableFuel/p.Cruise.FuelFlow, 1)
	}
	r.Minutes = round(r.Minutes, 1)
	for _, v := range []*float64{&r.TripFuel, &r.Contingency, &r.Alternate, &r.Required} {
		*v = round(*v, 2)
	}
	return r, nil
}
