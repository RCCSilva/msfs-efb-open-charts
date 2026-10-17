package route

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strings"
)

// What a resolved route becomes outside the program: the route field of a flight plan, the link
// that fills the IVAO flight plan form, and the flight plan file of the simulator (.pln).
// They send what the route has: the origin, the destination and the places between them.

// ErrNoEnds is returned when the route does not start and end at an airport.
var ErrNoEnds = errors.New("the route must start at an airport and end at another one")

// ends checks that the route has an airport at each end.
func ends(points []Point) error {
	n := len(points)
	if n < 2 || points[0].Kind != "airport" || points[n-1].Kind != "airport" {
		return ErrNoEnds
	}
	return nil
}

// tokens writes the places as a pilot does: an airway or a procedure is named once, followed by
// the place it ends at (the places it passes through are not needed in the text).
func tokens(points []Point) []string {
	var out []string
	for i := 0; i < len(points); i++ {
		p := points[i]
		switch {
		case p.Kind == "procedure":
			out = append(out, p.Ident)
			j := i
			for j+1 < len(points) && points[j+1].Via == p.Ident {
				j++
			}
			if j > i {
				out = append(out, points[j].Ident)
			}
			i = j
		case p.Via != "" && i > 0:
			j := i
			for j+1 < len(points) && points[j+1].Via == p.Via {
				j++
			}
			out = append(out, p.Via, points[j].Ident)
			i = j
		default:
			out = append(out, p.Ident)
		}
	}
	return out
}

// Text is the whole route as a pilot writes it, origin and destination included.
func Text(points []Point) string { return strings.Join(tokens(points), " ") }

// ICAORoute is the route field of an ICAO flight plan: what is between the origin and the
// destination (those have their own fields).
func ICAORoute(points []Point) string {
	t := tokens(points)
	if len(points) > 0 && points[0].Kind == "airport" && len(t) > 0 {
		t = t[1:]
	}
	if n := len(points); n > 1 && points[n-1].Kind == "airport" && len(t) > 0 && t[len(t)-1] == points[n-1].Ident {
		t = t[:len(t)-1]
	}
	return strings.Join(t, " ")
}

// IVAOPlan is what the link carries to the IVAO flight plan form. The form fills what it receives.
type IVAOPlan struct {
	DepartureID string `json:"departureId"`
	ArrivalID   string `json:"arrivalId"`
	Route       string `json:"route"`
}

const ivaoCreate = "https://fpl.ivao.aero/flight-plans/create?flightPlan="

// IVAOLink builds the address that opens the IVAO flight plan form with the origin, the
// destination and the route filled in. The plan travels in the address, as base64 of a JSON.
func IVAOLink(points []Point) (string, IVAOPlan, error) {
	if err := ends(points); err != nil {
		return "", IVAOPlan{}, err
	}
	plan := IVAOPlan{DepartureID: points[0].Ident, ArrivalID: points[len(points)-1].Ident, Route: ICAORoute(points)}
	raw, err := json.Marshal(plan)
	if err != nil {
		return "", plan, err
	}
	return ivaoCreate + url.QueryEscape(base64.StdEncoding.EncodeToString(raw)), plan, nil
}

// dms writes an angle as the simulator's flight plan files do: N46° 33' 56.10"
func dms(v float64, pos, neg string) string {
	h := pos
	if v < 0 {
		h, v = neg, -v
	}
	hundredths := int64(math.Round(v * 3600 * 100)) // seconds, to 1/100: the rounding carries into the minutes
	deg := hundredths / (3600 * 100)
	rest := hundredths % (3600 * 100)
	return fmt.Sprintf("%s%d° %d' %d.%02d\"", h, deg, rest/6000, rest%6000/100, rest%100)
}

func lla(lat, lon, altFt float64) string {
	return dms(lat, "N", "S") + "," + dms(lon, "E", "W") + "," + fmt.Sprintf("%+010.2f", altFt)
}

func esc(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

// DefaultCruiseFt is the cruise altitude written in the file: the route does not carry one yet.
const DefaultCruiseFt = 10000

// PLN writes the route as a flight plan file of the simulator. Procedures are not written as such:
// the places they pass through are waypoints of the plan.
func PLN(points []Point, cruiseFt int) ([]byte, error) {
	if err := ends(points); err != nil {
		return nil, err
	}
	if cruiseFt <= 0 {
		cruiseFt = DefaultCruiseFt
	}
	dep, dst := points[0], points[len(points)-1]
	var b strings.Builder
	b.WriteString("\ufeff<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<SimBase.Document Type=\"AceXML\" version=\"1,0\">\n    <Descr>AceXML Document</Descr>\n    <FlightPlan.FlightPlan>\n")
	fmt.Fprintf(&b, "        <Title>%s</Title>\n", esc(dep.Ident+" to "+dst.Ident))
	b.WriteString("        <FPType>IFR</FPType>\n")
	fmt.Fprintf(&b, "        <CruisingAlt>%d</CruisingAlt>\n", cruiseFt)
	fmt.Fprintf(&b, "        <DepartureID>%s</DepartureID>\n", esc(dep.Ident))
	fmt.Fprintf(&b, "        <DepartureLLA>%s</DepartureLLA>\n", lla(dep.Lat, dep.Lon, 0))
	fmt.Fprintf(&b, "        <DestinationID>%s</DestinationID>\n", esc(dst.Ident))
	fmt.Fprintf(&b, "        <DestinationLLA>%s</DestinationLLA>\n", lla(dst.Lat, dst.Lon, 0))
	fmt.Fprintf(&b, "        <Descr>%s, %s</Descr>\n", esc(dep.Ident), esc(dst.Ident))
	fmt.Fprintf(&b, "        <DepartureName>%s</DepartureName>\n", esc(dep.Ident))
	fmt.Fprintf(&b, "        <DestinationName>%s</DestinationName>\n", esc(dst.Ident))
	b.WriteString("        <AppVersion>\n            <AppVersionMajor>11</AppVersionMajor>\n            <AppVersionBuild>282174</AppVersionBuild>\n        </AppVersion>\n")
	procedure := ""
	for _, p := range points {
		if p.Kind == "procedure" {
			procedure = p.Ident
			continue
		}
		typ := map[string]string{"airport": "Airport", "waypoint": "Intersection", "vor": "VOR", "ndb": "NDB"}[p.Kind]
		if typ == "" {
			typ = "User"
		}
		fmt.Fprintf(&b, "        <ATCWaypoint id=\"%s\">\n            <ATCWaypointType>%s</ATCWaypointType>\n", esc(p.Ident), typ)
		fmt.Fprintf(&b, "            <WorldPosition>%s</WorldPosition>\n", lla(p.Lat, p.Lon, 0))
		if p.Via != "" && p.Via != procedure {
			fmt.Fprintf(&b, "            <ATCAirway>%s</ATCAirway>\n", esc(p.Via))
		}
		if typ != "User" {
			b.WriteString("            <ICAO>\n")
			if p.Kind != "airport" && p.Region != "" {
				fmt.Fprintf(&b, "                <ICAORegion>%s</ICAORegion>\n", esc(p.Region))
			}
			fmt.Fprintf(&b, "                <ICAOIdent>%s</ICAOIdent>\n            </ICAO>\n", esc(p.Ident))
		}
		b.WriteString("        </ATCWaypoint>\n")
	}
	b.WriteString("    </FlightPlan.FlightPlan>\n</SimBase.Document>\n")
	return []byte(b.String()), nil
}
