package route

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// Arrange
	points := exportRoute()

	// Act
	icao := ICAORoute(points)
	whole := Text(points)

	// Assert
	assert.Equal(t, "GERTU UZ25 PABIN", icao)
	assert.Equal(t, "SBSP GERTU UZ25 PABIN SBBR", whole)
}

func TestAProcedureIsNamedOnceThenThePlaceItEndsAt(t *testing.T) {
	// Arrange
	points := []Point{
		{Kind: "airport", Ident: "SBSJ"},
		{Kind: "procedure", Ident: "BOBI3A", Via: "SID"},
		{Kind: "waypoint", Ident: "ABCDE", Via: "BOBI3A"},
		{Kind: "waypoint", Ident: "DAGUB", Via: "BOBI3A"},
		{Kind: "airport", Ident: "SBGR"},
	}

	// Act
	got := ICAORoute(points)

	// Assert
	assert.Equal(t, "BOBI3A DAGUB", got)
}

func TestIVAOLinkCarriesOriginDestinationAndRoute(t *testing.T) {
	// Arrange
	points := exportRoute()

	// Act
	link, plan, err := IVAOLink(points)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, IVAOPlan{DepartureID: "SBSP", ArrivalID: "SBBR", Route: "GERTU UZ25 PABIN"}, plan)
	u, err := url.Parse(link)
	require.NoError(t, err)
	assert.Equal(t, "fpl.ivao.aero", u.Host)
	assert.Equal(t, "/flight-plans/create", u.Path)
	raw, err := base64.StdEncoding.DecodeString(u.Query().Get("flightPlan"))
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, map[string]any{"departureId": "SBSP", "arrivalId": "SBBR", "route": "GERTU UZ25 PABIN"}, got)
}

func TestIVAOLinkNeedsAirportsAtBothEnds(t *testing.T) {
	// Arrange
	points := exportRoute()[:3] // ends at a waypoint

	// Act
	_, _, err := IVAOLink(points)

	// Assert
	assert.ErrorIs(t, err, ErrNoEnds)
}

func TestDMS(t *testing.T) {
	cases := []struct {
		name     string
		v        float64
		pos, neg string
		want     string
	}{
		{"north", 46.565583, "N", "S", "N46" + deg + " 33' 56.10" + secs},
		{"west", -120.531744, "E", "W", "W120" + deg + " 31' 54.28" + secs},
		{"south, a round half", -23.5, "N", "S", "S23" + deg + " 30' 0.00" + secs},
		{"the rounding carries into the minutes and the degrees", 0.99999999, "N", "S", "N1" + deg + " 0' 0.00" + secs},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange: the case

			// Act
			got := dms(c.v, c.pos, c.neg)

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}

func TestLLAHasTheAltitudeInFeet(t *testing.T) {
	// Arrange / Act
	got := lla(46.565583, -120.531744, 1095)

	// Assert
	assert.Equal(t, "N46"+deg+" 33' 56.10"+secs+",W120"+deg+" 31' 54.28"+secs+",+001095.00", got)
}

func TestPLNIsTheSimulatorsFlightPlan(t *testing.T) {
	// Arrange
	points := exportRoute()

	// Act
	b, err := PLN(points, 0)

	// Assert
	require.NoError(t, err)
	s := string(b)
	for _, want := range []string{
		"<Title>SBSP to SBBR</Title>", "<FPType>IFR</FPType>", "<CruisingAlt>10000</CruisingAlt>",
		"<DepartureID>SBSP</DepartureID>", "<DestinationID>SBBR</DestinationID>",
		`<ATCWaypoint id="GERTU">`, "<ATCWaypointType>Intersection</ATCWaypointType>", "<ATCAirway>UZ25</ATCAirway>",
		"<ICAORegion>SB</ICAORegion>", "<ICAOIdent>PABIN</ICAOIdent>", "<ATCWaypointType>Airport</ATCWaypointType>",
	} {
		assert.Contains(t, s, want)
	}
	assert.Equal(t, 6, strings.Count(s, "<ATCWaypoint "))
	assert.Equal(t, 3, strings.Count(s, "<ATCAirway>"))
	var doc struct{ XMLName xml.Name }
	require.NoError(t, xml.Unmarshal([]byte(strings.TrimPrefix(s, "\ufeff")), &doc), "it is well-formed XML")
	assert.Equal(t, "SimBase.Document", doc.XMLName.Local)
}

func TestPLNDoesNotWriteAProcedureAsAWaypoint(t *testing.T) {
	// Arrange
	points := []Point{
		{Kind: "airport", Ident: "SBSJ"},
		{Kind: "procedure", Ident: "BOBI3A", Via: "SID"},
		{Kind: "waypoint", Ident: "DAGUB", Region: "SB", Via: "BOBI3A"},
		{Kind: "airport", Ident: "SBGR"},
	}

	// Act
	b, err := PLN(points, 35000)

	// Assert
	require.NoError(t, err)
	s := string(b)
	assert.NotContains(t, s, "BOBI3A", "a procedure is not a waypoint, and its legs have no airway")
	assert.Equal(t, 3, strings.Count(s, "<ATCWaypoint "))
	assert.Contains(t, s, "<CruisingAlt>35000</CruisingAlt>")
}

func TestPLNNeedsAirportsAtBothEnds(t *testing.T) {
	// Arrange
	points := []Point{{Kind: "airport", Ident: "SBSJ"}, {Kind: "procedure", Ident: "BOBI3A"}}

	// Act
	_, err := PLN(points, 0)

	// Assert
	assert.ErrorIs(t, err, ErrNoEnds)
}

// The simulator's flight plan files write an angle as N46° 33' 56.10"
const (
	deg  = "°"
	secs = "\""
)
