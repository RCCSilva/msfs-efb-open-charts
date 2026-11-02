package server

import (
	"encoding/json"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
)

// Made up, in the shape of the files Little Navmap writes.
const lnmFile = `<?xml version="1.0"?>
<LittleNavmap><AircraftPerf>
<Options><Name>Test Single</Name><AircraftType>C172</AircraftType><FuelAsVolume>1</FuelAsVolume><JetFuel>0</JetFuel></Options>
<Perf><ContingencyFuelPercent>10</ContingencyFuelPercent><ReserveFuelLbsGal>8</ReserveFuelLbsGal><TaxiFuelLbsGal>1</TaxiFuelLbsGal><UsableFuelLbsGal>50</UsableFuelLbsGal>
<Alternate><FuelFlowLbsGalPerHour>9</FuelFlowLbsGalPerHour><SpeedKtsTAS>100</SpeedKtsTAS></Alternate>
<Climb><FuelFlowLbsGalPerHour>11</FuelFlowLbsGalPerHour><SpeedKtsTAS>80</SpeedKtsTAS><VertSpeedFtPerMin>700</VertSpeedFtPerMin></Climb>
<Cruise><FuelFlowLbsGalPerHour>8</FuelFlowLbsGalPerHour><SpeedKtsTAS>110</SpeedKtsTAS></Cruise>
<Descent><FuelFlowLbsGalPerHour>5</FuelFlowLbsGalPerHour><SpeedKtsTAS>100</SpeedKtsTAS><VertSpeedFtPerMin>500</VertSpeedFtPerMin></Descent>
</Perf></AircraftPerf></LittleNavmap>`

type aircraftView struct {
	ID       int64
	Name     string
	FuelUnit string
	Cruise   struct{ SpeedKt float64 }
}

func importAircraft(t *testing.T, s *Server, body string) aircraftView {
	t.Helper()
	w := post(s.handleAircraftImport, body)
	require.Equal(t, 200, w.Code, w.Body.String())
	var a aircraftView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &a))
	return a
}

func TestALittleNavmapFileBecomesAnAircraftInTheList(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})

	// Act
	a := importAircraft(t, s, lnmFile)
	list := httptest.NewRecorder()
	s.handleAircraftList(list, httptest.NewRequest("GET", "/api/aircraft", nil))

	// Assert
	assert.NotZero(t, a.ID)
	assert.Equal(t, "Test Single", a.Name)
	assert.Equal(t, "gal", a.FuelUnit)
	assert.Equal(t, 110.0, a.Cruise.SpeedKt)
	assert.Contains(t, list.Body.String(), `"name":"Test Single"`)
}

func TestAFileThatIsNotAProfileIsRefused(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})

	// Act
	w := post(s.handleAircraftImport, "<html></html>")

	// Assert
	assert.Equal(t, 400, w.Code)
}

func TestAnAircraftCanBeEditedByHand(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})
	a := importAircraft(t, s, lnmFile)
	w := httptest.NewRecorder()
	s.handleAircraftList(w, httptest.NewRequest("GET", "/", nil))
	var all struct{ Aircraft []map[string]any }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &all))
	p := all.Aircraft[0]
	p["name"] = "Mine"
	p["cruise"] = map[string]any{"speedKt": 125, "fuelFlow": 9}
	body, _ := json.Marshal(p)

	// Act
	req := httptest.NewRequest("PUT", "/api/aircraft/"+strconv.FormatInt(a.ID, 10), strings.NewReader(string(body)))
	req.SetPathValue("id", strconv.FormatInt(a.ID, 10))
	out := httptest.NewRecorder()
	s.handleAircraftSave(out, req)

	// Assert
	require.Equal(t, 200, out.Code, out.Body.String())
	var saved aircraftView
	require.NoError(t, json.Unmarshal(out.Body.Bytes(), &saved))
	assert.Equal(t, a.ID, saved.ID)
	assert.Equal(t, "Mine", saved.Name)
	assert.Equal(t, 125.0, saved.Cruise.SpeedKt)
}

func TestTheEstimateUsesTheAircraftAndTheAltitude(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})
	a := importAircraft(t, s, lnmFile)

	// Act
	w := post(s.handleRouteEstimate, `{"profileId":`+strconv.FormatInt(a.ID, 10)+`,"legsNm":[0,60,90],"cruiseFt":6000}`)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	var r struct {
		TotalNm  float64
		Minutes  float64
		Required float64
		Legs     []struct{ Minutes float64 }
		Phases   []struct{ Name string }
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &r))
	assert.Equal(t, 150.0, r.TotalNm)
	assert.Greater(t, r.Minutes, 60.0)
	assert.Greater(t, r.Required, 9.0, "more than the reserve and the taxi")
	assert.Len(t, r.Legs, 3)
	assert.Len(t, r.Phases, 3)
}

func TestTheEstimateRefusesWhatItCannotUse(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})
	a := importAircraft(t, s, lnmFile)

	// Act
	noAircraft := post(s.handleRouteEstimate, `{"profileId":999,"legsNm":[0,10],"cruiseFt":5000}`)
	noAltitude := post(s.handleRouteEstimate, `{"profileId":`+strconv.FormatInt(a.ID, 10)+`,"legsNm":[0,10]}`)

	// Assert
	assert.Equal(t, 404, noAircraft.Code)
	assert.Equal(t, 400, noAltitude.Code)
}

func TestARouteIsSavedWithItsAircraftAndAltitude(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})
	a := importAircraft(t, s, lnmFile)

	// Act
	w := post(s.handleRouteSave, `{"text":"SBSP SBBR","profileId":`+strconv.FormatInt(a.ID, 10)+`,"cruiseFt":9000}`)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	var saved struct {
		ProfileID *int64
		CruiseFt  int
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &saved))
	require.NotNil(t, saved.ProfileID)
	assert.Equal(t, a.ID, *saved.ProfileID)
	assert.Equal(t, 9000, saved.CruiseFt)
}
