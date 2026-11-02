package perf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Made up, in the shape of the files Little Navmap writes.
const xmlProfile = `<?xml version="1.0" encoding="UTF-8"?>
<LittleNavmap xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <AircraftPerf>
    <Header><FileVersion>1.0</FileVersion></Header>
    <Options>
      <Name>Test Single</Name><AircraftType>c172</AircraftType><Simulator>MSFS</Simulator>
      <Description>made up</Description><FuelAsVolume>0</FuelAsVolume><JetFuel>0</JetFuel>
    </Options>
    <Perf>
      <ContingencyFuelPercent>10.0</ContingencyFuelPercent>
      <ExtraFuelLbsGal>3.000</ExtraFuelLbsGal>
      <MinRunwayLengthFt>1000.000</MinRunwayLengthFt>
      <ReserveFuelLbsGal>50.000</ReserveFuelLbsGal>
      <RunwayType>SOFT</RunwayType>
      <TaxiFuelLbsGal>3.000</TaxiFuelLbsGal>
      <UsableFuelLbsGal>336.000</UsableFuelLbsGal>
      <Alternate><FuelFlowLbsGalPerHour>45.000</FuelFlowLbsGalPerHour><SpeedKtsTAS>100.000</SpeedKtsTAS></Alternate>
      <Climb><FuelFlowLbsGalPerHour>55.000</FuelFlowLbsGalPerHour><SpeedKtsTAS>96.000</SpeedKtsTAS><VertSpeedFtPerMin>728.000</VertSpeedFtPerMin></Climb>
      <Cruise><FuelFlowLbsGalPerHour>39.000</FuelFlowLbsGalPerHour><SpeedKtsTAS>120.000</SpeedKtsTAS></Cruise>
      <Descent><FuelFlowLbsGalPerHour>25.000</FuelFlowLbsGalPerHour><SpeedKtsTAS>116.000</SpeedKtsTAS><VertSpeedFtPerMin>550.000</VertSpeedFtPerMin></Descent>
    </Perf>
  </AircraftPerf>
</LittleNavmap>
`

const iniProfile = `[Options]
Metadata=Made by Little Navmap
ProgramVersion=2.4.5
Name=Test Old
AircraftType=TT:ATCCOM.AC_MODEL C152.0.text
Description=
FuelAsVolume=true
JetFuel=false

[Perf]
UsableFuel=26
TaxiFuelLbsGal=0.128735
ReserveFuelLbsGal=1
ExtraFuelLbsGal=0
ContingencyFuelPercent=0
ClimbVertSpeedFtPerMin=849.307
ClimbSpeedKtsTAS=63.2595
ClimbFuelFlowLbsGalPerHour=11.5604
CruiseSpeedKtsTAS=102.74
CruiseFuelFlowLbsGalPerHour=6.77701
DescentSpeedKtsTAS=84.4175
DescentVertSpeedFtPerMin=591.188
DescentFuelFlowLbsGalPerHour=3.03103
AlternateSpeed=100
AlternateFuelFlow=7
MinRunwayLength=725
RunwayType=Soft
`

func TestParseXML(t *testing.T) {
	// Act
	p, err := Parse([]byte(xmlProfile))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Test Single", p.Name)
	assert.Equal(t, "C172", p.AircraftType)
	assert.Equal(t, Pounds, p.FuelUnit)
	assert.False(t, p.Jet)
	assert.Equal(t, 336.0, p.UsableFuel)
	assert.Equal(t, 10.0, p.ContingencyPct)
	assert.Equal(t, "soft", p.RunwayType)
	assert.Equal(t, Phase{SpeedKt: 96, FuelFlow: 55, VertFpm: 728}, p.Climb)
	assert.Equal(t, Phase{SpeedKt: 120, FuelFlow: 39}, p.Cruise)
	assert.Equal(t, Phase{SpeedKt: 116, FuelFlow: 25, VertFpm: 550}, p.Descent)
	assert.Equal(t, Phase{SpeedKt: 100, FuelFlow: 45}, p.Alternate)
}

func TestParseOldINI(t *testing.T) {
	// Act
	p, err := Parse([]byte(iniProfile))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Test Old", p.Name)
	assert.Equal(t, "C152", p.AircraftType, "the type is taken out of the simulator's text")
	assert.Equal(t, Gallons, p.FuelUnit)
	assert.Equal(t, 26.0, p.UsableFuel)
	assert.Equal(t, 63.2595, p.Climb.SpeedKt)
	assert.Equal(t, 849.307, p.Climb.VertFpm)
	assert.Equal(t, 100.0, p.Alternate.SpeedKt)
	assert.Equal(t, "soft", p.RunwayType)
}

func TestParseRefusesWhatIsNotAProfile(t *testing.T) {
	for name, data := range map[string]string{
		"empty":      "",
		"text":       "hello",
		"other xml":  "<html></html>",
		"no speeds":  "[Options]\nName=x\nFuelAsVolume=true\n[Perf]\n",
		"broken xml": "<LittleNavmap><AircraftPerf>",
	} {
		// Act
		_, err := Parse([]byte(data))

		// Assert
		assert.ErrorIs(t, err, ErrInvalid, name)
	}
}

func single(t *testing.T) Profile {
	p, err := Parse([]byte(xmlProfile))
	require.NoError(t, err)
	return p
}

func TestEstimateSplitsTheRouteIntoPhases(t *testing.T) {
	// Arrange: 6000 ft of climb at 728 ft/min and 96 kt, then of descent at 550 ft/min and 116 kt
	p := single(t)
	climbMin, descentMin := 6000/728.0, 6000/550.0
	climbNm, descentNm := 96*climbMin/60, 116*descentMin/60
	cruiseNm := 200 - climbNm - descentNm
	cruiseMin := cruiseNm / 120 * 60
	trip := climbMin/60*55 + cruiseMin/60*39 + descentMin/60*25

	// Act
	r, err := p.Estimate(Flight{LegsNm: []float64{0, 80, 120}, CruiseFt: 6000, AlternateNm: 50})

	// Assert
	require.NoError(t, err)
	assert.False(t, r.Truncated)
	assert.Equal(t, 6000.0, r.CruiseFt)
	assert.InDelta(t, climbNm, r.TopOfClimbNm, 0.1)
	assert.InDelta(t, 200-descentNm, r.TopOfDescentNm, 0.1)
	assert.InDelta(t, climbMin+cruiseMin+descentMin, r.Minutes, 0.1)
	assert.InDelta(t, trip, r.TripFuel, 0.01)
	assert.InDelta(t, trip*0.1, r.Contingency, 0.01)
	assert.InDelta(t, 50.0/100*45, r.Alternate, 0.01)
	assert.InDelta(t, 3+trip+trip*0.1+22.5+50+3, r.Required, 0.02)
	assert.True(t, r.EnoughFuel)
	require.Len(t, r.Phases, 3)
	assert.Equal(t, "climb", r.Phases[0].Name)
	assert.Equal(t, "descent", r.Phases[2].Name)
}

func TestEstimateLegsAddUpToTheFlight(t *testing.T) {
	// Arrange
	p := single(t)

	// Act
	r, err := p.Estimate(Flight{LegsNm: []float64{0, 5, 40, 100, 55}, CruiseFt: 8000})

	// Assert
	require.NoError(t, err)
	require.Len(t, r.Legs, 5)
	var mins, fuel float64
	for _, l := range r.Legs {
		mins += l.Minutes
		fuel += l.Fuel
	}
	assert.InDelta(t, r.Minutes, mins, 0.3)
	assert.InDelta(t, r.TripFuel, fuel, 0.3)
	assert.Equal(t, 0.0, r.Legs[0].Minutes, "the first point has no leg")
	assert.Greater(t, r.Legs[1].Fuel/r.Legs[1].Minutes, r.Legs[3].Fuel/r.Legs[3].Minutes, "the leg that climbs burns more per minute than the one that cruises")
}

func TestEstimateShortRouteDoesNotReachTheAltitude(t *testing.T) {
	// Arrange
	p := single(t)

	// Act
	r, err := p.Estimate(Flight{LegsNm: []float64{0, 20}, CruiseFt: 10000})

	// Assert
	require.NoError(t, err)
	assert.True(t, r.Truncated)
	assert.Less(t, r.CruiseFt, 10000.0)
	assert.Greater(t, r.CruiseFt, 0.0)
	assert.Equal(t, 0.0, r.Phases[1].Nm, "no cruise")
	assert.InDelta(t, 20, r.Phases[0].Nm+r.Phases[2].Nm, 0.1)
}

func TestEstimateAirportElevation(t *testing.T) {
	// Arrange
	p := single(t)
	flight := Flight{LegsNm: []float64{0, 200}, CruiseFt: 6000}

	// Act
	low, err := p.Estimate(flight)
	require.NoError(t, err)
	flight.DepartureFt = 3000
	high, err := p.Estimate(flight)
	require.NoError(t, err)

	// Assert
	assert.Less(t, high.Phases[0].Minutes, low.Phases[0].Minutes, "less to climb from a high airport")
}

func TestEstimateWind(t *testing.T) {
	// Arrange
	p := single(t)
	flight := Flight{LegsNm: []float64{0, 300}, CruiseFt: 6000}

	// Act
	calm, _ := p.Estimate(flight)
	flight.WindKt = -20
	head, err := p.Estimate(flight)
	require.NoError(t, err)
	flight.WindKt = -200
	_, tooStrong := p.Estimate(flight)

	// Assert
	assert.Greater(t, head.Minutes, calm.Minutes)
	assert.ErrorIs(t, tooStrong, ErrInvalid)
}

func TestEstimateWarnsWhenTheFuelIsNotEnough(t *testing.T) {
	// Arrange
	p := single(t)
	p.UsableFuel = 100

	// Act
	r, err := p.Estimate(Flight{LegsNm: []float64{0, 400}, CruiseFt: 6000})

	// Assert
	require.NoError(t, err)
	assert.False(t, r.EnoughFuel)
	assert.Equal(t, 100.0, r.Usable)
}

func TestEstimateNeedsAnAltitude(t *testing.T) {
	// Act
	_, err := single(t).Estimate(Flight{LegsNm: []float64{0, 100}})

	// Assert
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestPoundsPerUnit(t *testing.T) {
	// Assert
	assert.Equal(t, 1.0, Profile{FuelUnit: Pounds, Jet: true}.LbPerUnit())
	assert.Equal(t, 6.0, Profile{FuelUnit: Gallons}.LbPerUnit())
	assert.Equal(t, 6.7, Profile{FuelUnit: Gallons, Jet: true}.LbPerUnit())
}
