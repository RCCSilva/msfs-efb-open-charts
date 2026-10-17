package route

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"net/url"
	"strings"
	"testing"
)

// SBSP > GERTU > UZ25 (PUGOR, KOMTO) > PABIN > SBBR, as the resolver returns it: the places an
// airway passes through carry the airway in Via.
func exportRoute() []Point {
	return []Point{
		{Kind: "airport", Ident: "SBSP", Lat: -23.6261, Lon: -46.6564},
		{Kind: "waypoint", Ident: "GERTU", Region: "SB", Lat: -23.1, Lon: -46.2},
		{Kind: "waypoint", Ident: "PUGOR", Region: "SB", Lat: -22.5, Lon: -45.9, Via: "UZ25"},
		{Kind: "waypoint", Ident: "KOMTO", Region: "SB", Lat: -22.0, Lon: -45.5, Via: "UZ25"},
		{Kind: "waypoint", Ident: "PABIN", Region: "SB", Lat: -21.5, Lon: -45.1, Via: "UZ25"},
		{Kind: "airport", Ident: "SBBR", Lat: -15.8711, Lon: -47.9186},
	}
}

func TestICAORouteLeavesOutTheEnds(t *testing.T) {
	if got := ICAORoute(exportRoute()); got != "GERTU UZ25 PABIN" {
		t.Fatalf("%q", got)
	}
	if got := Text(exportRoute()); got != "SBSP GERTU UZ25 PABIN SBBR" {
		t.Fatalf("%q", got)
	}
	// a procedure is named once, then the place it ends at
	proc := []Point{
		{Kind: "airport", Ident: "SBSJ"},
		{Kind: "procedure", Ident: "BOBI3A", Via: "SID"},
		{Kind: "waypoint", Ident: "ABCDE", Via: "BOBI3A"},
		{Kind: "waypoint", Ident: "DAGUB", Via: "BOBI3A"},
		{Kind: "airport", Ident: "SBGR"},
	}
	if got := ICAORoute(proc); got != "BOBI3A DAGUB" {
		t.Fatalf("%q", got)
	}
}

func TestIVAOLinkCarriesOriginDestinationAndRoute(t *testing.T) {
	link, plan, err := IVAOLink(exportRoute())
	if err != nil {
		t.Fatal(err)
	}
	if plan.DepartureID != "SBSP" || plan.ArrivalID != "SBBR" || plan.Route != "GERTU UZ25 PABIN" {
		t.Fatalf("%+v", plan)
	}
	u, err := url.Parse(link)
	if err != nil || u.Host != "fpl.ivao.aero" || u.Path != "/flight-plans/create" {
		t.Fatalf("%s %v", link, err)
	}
	raw, err := base64.StdEncoding.DecodeString(u.Query().Get("flightPlan"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil || got["departureId"] != "SBSP" || got["arrivalId"] != "SBBR" || got["route"] != "GERTU UZ25 PABIN" || len(got) != 3 {
		t.Fatalf("%s %v", raw, err)
	}
	if _, _, err := IVAOLink(exportRoute()[:3]); err != ErrNoEnds {
		t.Fatalf("a route that does not end at an airport: %v", err)
	}
}

func TestDMS(t *testing.T) {
	cases := []struct {
		v        float64
		pos, neg string
		want     string
	}{
		{46.565583, "N", "S", `N46° 33' 56.10"`},
		{-120.531744, "E", "W", `W120° 31' 54.28"`},
		{-23.5, "N", "S", `S23° 30' 0.00"`},
		{0.99999999, "N", "S", `N1° 0' 0.00"`}, // the rounding carries into the minutes and the degrees
	}
	for _, c := range cases {
		if got := dms(c.v, c.pos, c.neg); got != c.want {
			t.Errorf("%v: %s, want %s", c.v, got, c.want)
		}
	}
	if got := lla(46.565583, -120.531744, 1095); got != `N46° 33' 56.10",W120° 31' 54.28",+001095.00` {
		t.Fatalf("%s", got)
	}
}

func TestPLNIsTheSimulatorsFlightPlan(t *testing.T) {
	b, err := PLN(exportRoute(), 0)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"<Title>SBSP to SBBR</Title>", "<FPType>IFR</FPType>", "<CruisingAlt>10000</CruisingAlt>",
		"<DepartureID>SBSP</DepartureID>", "<DestinationID>SBBR</DestinationID>",
		`<ATCWaypoint id="GERTU">`, "<ATCWaypointType>Intersection</ATCWaypointType>", "<ATCAirway>UZ25</ATCAirway>",
		"<ICAORegion>SB</ICAORegion>", "<ICAOIdent>PABIN</ICAOIdent>", "<ATCWaypointType>Airport</ATCWaypointType>",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Count(s, "<ATCWaypoint ") != 6 || strings.Count(s, "<ATCAirway>") != 3 {
		t.Errorf("waypoints %d, airways %d", strings.Count(s, "<ATCWaypoint "), strings.Count(s, "<ATCAirway>"))
	}
	// it is well-formed XML
	var doc struct {
		XMLName xml.Name
	}
	if err := xml.Unmarshal([]byte(strings.TrimPrefix(s, "\ufeff")), &doc); err != nil || doc.XMLName.Local != "SimBase.Document" {
		t.Fatalf("%v %v", doc, err)
	}
	// a procedure is not a waypoint, and its legs have no airway
	proc := []Point{
		{Kind: "airport", Ident: "SBSJ"},
		{Kind: "procedure", Ident: "BOBI3A", Via: "SID"},
		{Kind: "waypoint", Ident: "DAGUB", Region: "SB", Via: "BOBI3A"},
		{Kind: "airport", Ident: "SBGR"},
	}
	b, _ = PLN(proc, 35000)
	s = string(b)
	if strings.Contains(s, "BOBI3A") || strings.Count(s, "<ATCWaypoint ") != 3 || !strings.Contains(s, "<CruisingAlt>35000</CruisingAlt>") {
		t.Fatalf("%s", s)
	}
	if _, err := PLN(proc[:2], 0); err != ErrNoEnds {
		t.Fatalf("%v", err)
	}
}
