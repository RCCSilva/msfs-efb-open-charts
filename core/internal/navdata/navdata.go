// Package navdata holds the navigation data the simulator knows (airports, runways, frequencies,
// procedures with their legs) as plain values. The simulator link produces them, the database
// (package navdb) stores them and the server serves them; none of those depends on the others.
//
// Units follow the simulator: metres for altitudes and lengths, degrees, hertz.
package navdata

import "time"

// Airport is one airport as the simulator describes it.
type Airport struct {
	ICAO       string  `json:"icao"`
	Region     string  `json:"region"`
	Name       string  `json:"name"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	AltitudeM  float64 `json:"altitudeM"`
	TransAltM  float64 `json:"transitionAltitudeM"`
	TransLevel float64 `json:"transitionLevel"`

	Runways     []Runway    `json:"runways"`
	Frequencies []Frequency `json:"frequencies"`
	Procedures  []Procedure `json:"procedures"`

	// FetchedAt and Source say where and when the data came from (the data is the simulator's:
	// it does not follow the AIRAC cycle of the published charts).
	FetchedAt time.Time `json:"fetchedAt"`
	Source    string    `json:"source"`
}

// Runway is one physical runway with both of its ends.
type Runway struct {
	Lat          float64 `json:"lat"`
	Lon          float64 `json:"lon"`
	AltitudeM    float64 `json:"altitudeM"`
	HeadingTrue  float64 `json:"headingTrue"`
	LengthM      float64 `json:"lengthM"`
	WidthM       float64 `json:"widthM"`
	Surface      int     `json:"surface"`
	PrimaryName  string  `json:"primary"`   // "16", "16L"
	SecondaryNam string  `json:"secondary"` // "34", "34R"
}

// Frequency is one communication frequency.
type Frequency struct {
	Type int    `json:"type"`
	Hz   int64  `json:"hz"`
	Name string `json:"name"`
}

// Procedure kinds.
const (
	KindSID      = "SID"
	KindSTAR     = "STAR"
	KindApproach = "APP"
)

// Procedure is a departure, an arrival or an approach.
type Procedure struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	// Runway is the runway an approach is for ("16", "34R"); empty for SID and STAR.
	Runway string `json:"runway,omitempty"`
	// ApproachType and Suffix are the simulator's codes for an approach (ILS, RNAV...); kept as
	// numbers until their meaning is confirmed.
	ApproachType int    `json:"approachType,omitempty"`
	Suffix       int    `json:"suffix,omitempty"`
	FAF          string `json:"faf,omitempty"`

	Transitions []Transition `json:"transitions"`
}

// Transition kinds.
const (
	TransCommon  = "COMMON"  // the legs every use of the procedure flies
	TransRunway  = "RUNWAY"  // from/to a runway
	TransEnroute = "ENROUTE" // from/to the airway system
	TransFinal   = "FINAL"   // the final legs of an approach
	TransMissed  = "MISSED"  // the missed approach
)

// Transition is a named part of a procedure with its legs, in flying order.
type Transition struct {
	Kind   string `json:"kind"`
	Name   string `json:"name,omitempty"`
	Runway string `json:"runway,omitempty"`
	Legs   []Leg  `json:"legs"`
}

// Leg is one leg of a procedure.
type Leg struct {
	Type        int     `json:"type"`    // ARINC leg type (see LegTypeName)
	FixType     string  `json:"fixType"` // "W" waypoint, "V" VOR, "N" NDB, "R" runway, "A" airport...
	FixICAO     string  `json:"fix"`     // identifier of the fix
	FixRegion   string  `json:"fixRegion"`
	FlyOver     bool    `json:"flyOver"`
	TurnLeft    bool    `json:"turnLeft,omitempty"`
	TurnRight   bool    `json:"turnRight,omitempty"`
	Course      float64 `json:"course,omitempty"`
	DistanceNm  float64 `json:"distance,omitempty"`
	AltDesc     int     `json:"altitudeDescriptor,omitempty"`
	Altitude1M  float64 `json:"altitude1M,omitempty"`
	Altitude2M  float64 `json:"altitude2M,omitempty"`
	SpeedLimit  float64 `json:"speedLimit,omitempty"` // knots, -1 when there is none
	VerticalDeg float64 `json:"verticalAngle,omitempty"`
}

var legTypeName = map[int]string{
	1: "AF", 2: "CA", 3: "CD", 4: "CF", 5: "CI", 6: "CR", 7: "DF", 8: "FA", 9: "FC", 10: "FD",
	11: "FM", 12: "HA", 13: "HF", 14: "HM", 15: "IF", 16: "PI", 17: "RF", 18: "TF", 19: "VA",
	20: "VD", 21: "VI", 22: "VM", 23: "VR",
}

// LegTypeName is the ARINC 424 name of a leg type ("TF", "DF"...), or "" when unknown.
func LegTypeName(t int) string { return legTypeName[t] }
