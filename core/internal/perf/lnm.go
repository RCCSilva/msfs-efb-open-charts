package perf

import (
	"bytes"
	"encoding/xml"
	"strconv"
	"strings"
)

// Parse reads a performance file of Little Navmap: the XML of the current versions (.lnmperf,
// "LittleNavmap" root) or the INI of the old ones (2.4 and earlier). The profile is cleaned and
// checked.
func Parse(data []byte) (Profile, error) {
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var p Profile
	var err error
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("<")) {
		p, err = parseXML(data)
	} else {
		p, err = parseINI(data)
	}
	if err != nil {
		return Profile{}, err
	}
	if err := p.Clean(); err != nil {
		return Profile{}, err
	}
	return p, nil
}

type xmlPhase struct {
	FuelFlow float64 `xml:"FuelFlowLbsGalPerHour"`
	Speed    float64 `xml:"SpeedKtsTAS"`
	Vert     float64 `xml:"VertSpeedFtPerMin"`
}

func (x xmlPhase) phase() Phase {
	return Phase{SpeedKt: x.Speed, FuelFlow: x.FuelFlow, VertFpm: x.Vert}
}

type xmlFile struct {
	XMLName xml.Name `xml:"LittleNavmap"`
	Perf    struct {
		Options struct {
			Name         string `xml:"Name"`
			AircraftType string `xml:"AircraftType"`
			Simulator    string `xml:"Simulator"`
			Description  string `xml:"Description"`
			FuelAsVolume int    `xml:"FuelAsVolume"`
			JetFuel      int    `xml:"JetFuel"`
		} `xml:"Options"`
		Perf struct {
			Contingency float64  `xml:"ContingencyFuelPercent"`
			Extra       float64  `xml:"ExtraFuelLbsGal"`
			MinRunway   float64  `xml:"MinRunwayLengthFt"`
			Reserve     float64  `xml:"ReserveFuelLbsGal"`
			RunwayType  string   `xml:"RunwayType"`
			Taxi        float64  `xml:"TaxiFuelLbsGal"`
			Usable      float64  `xml:"UsableFuelLbsGal"`
			Alternate   xmlPhase `xml:"Alternate"`
			Climb       xmlPhase `xml:"Climb"`
			Cruise      xmlPhase `xml:"Cruise"`
			Descent     xmlPhase `xml:"Descent"`
		} `xml:"Perf"`
	} `xml:"AircraftPerf"`
}

func fuelUnit(volume bool) string {
	if volume {
		return Gallons
	}
	return Pounds
}

func parseXML(data []byte) (Profile, error) {
	var f xmlFile
	if err := xml.Unmarshal(data, &f); err != nil {
		return Profile{}, invalid("this is not a Little Navmap performance file (%v)", err)
	}
	o, q := f.Perf.Options, f.Perf.Perf
	return Profile{
		Name: o.Name, AircraftType: o.AircraftType, Simulator: o.Simulator, Description: o.Description,
		FuelUnit: fuelUnit(o.FuelAsVolume != 0), Jet: o.JetFuel != 0,
		UsableFuel: q.Usable, TaxiFuel: q.Taxi, ReserveFuel: q.Reserve, ExtraFuel: q.Extra, ContingencyPct: q.Contingency,
		MinRunwayFt: q.MinRunway, RunwayType: q.RunwayType,
		Climb: q.Climb.phase(), Cruise: q.Cruise.phase(), Descent: q.Descent.phase(), Alternate: q.Alternate.phase(),
	}, nil
}

// parseINI reads the old format: "[Options]" and "[Perf]" sections of key=value lines.
func parseINI(data []byte) (Profile, error) {
	kv := map[string]string{}
	seen := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "[Options]" || line == "[Perf]" {
			seen = true
		}
		if i := strings.IndexByte(line, '='); i > 0 && !strings.HasPrefix(line, "[") {
			kv[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		}
	}
	if !seen {
		return Profile{}, invalid("this is not a Little Navmap performance file")
	}
	num := func(k string) float64 { v, _ := strconv.ParseFloat(kv[k], 64); return v }
	flag := func(k string) bool { return strings.EqualFold(kv[k], "true") || kv[k] == "1" }
	return Profile{
		Name: kv["Name"], AircraftType: oldType(kv["AircraftType"]), Description: kv["Description"],
		FuelUnit: fuelUnit(flag("FuelAsVolume")), Jet: flag("JetFuel"),
		UsableFuel: num("UsableFuel"), TaxiFuel: num("TaxiFuelLbsGal"), ReserveFuel: num("ReserveFuelLbsGal"),
		ExtraFuel: num("ExtraFuelLbsGal"), ContingencyPct: num("ContingencyFuelPercent"),
		MinRunwayFt: num("MinRunwayLength"), RunwayType: kv["RunwayType"],
		Climb:     Phase{SpeedKt: num("ClimbSpeedKtsTAS"), FuelFlow: num("ClimbFuelFlowLbsGalPerHour"), VertFpm: num("ClimbVertSpeedFtPerMin")},
		Cruise:    Phase{SpeedKt: num("CruiseSpeedKtsTAS"), FuelFlow: num("CruiseFuelFlowLbsGalPerHour")},
		Descent:   Phase{SpeedKt: num("DescentSpeedKtsTAS"), FuelFlow: num("DescentFuelFlowLbsGalPerHour"), VertFpm: num("DescentVertSpeedFtPerMin")},
		Alternate: Phase{SpeedKt: num("AlternateSpeed"), FuelFlow: num("AlternateFuelFlow")},
	}, nil
}

// oldType takes the ICAO type out of the old files' "TT:ATCCOM.AC_MODEL C152.0.text".
func oldType(s string) string {
	if strings.HasPrefix(s, "TT:ATCCOM.AC_MODEL ") {
		s = strings.TrimPrefix(s, "TT:ATCCOM.AC_MODEL ")
		if i := strings.IndexByte(s, '.'); i > 0 {
			return s[:i]
		}
	}
	return s
}
