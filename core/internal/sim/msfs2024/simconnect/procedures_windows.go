//go:build windows

package simconnect

import (
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// Facility data types of the nested levels of a departure or arrival.
const (
	facilityTypeLeg           = 7
	facilityTypeRunwayTrans   = 12
	facilityTypeEnrouteTrans  = 13
	facilityTypeProcedureNone = 0
)

var procLeg = []facField{
	fi("TYPE"), fi("FIX_TYPE"), fs("FIX_REGION", 8), fs("FIX_ICAO", 8),
	fi("FLY_OVER"), fi("DISTANCE_MINUTE"), fi("TRUE_DEGREE"), fi("TURN_DIRECTION"),
	fs("ORIGIN_ICAO", 8), fs("ORIGIN_REGION", 8), fi("ORIGIN_TYPE"),
	ff("THETA"), ff("RHO"), ff("COURSE"), ff("ROUTE_DISTANCE"),
	fi("APPROACH_ALT_DESC"), ff("ALTITUDE1"), ff("ALTITUDE2"), ff("SPEED_LIMIT"), ff("VERTICAL_ANGLE"),
}

// Leg types of the SDK (SIMCONNECT_..._LEG_TYPE), by value, for the report.
var legTypeName = map[int32]string{
	1: "AF", 2: "CA", 3: "CD", 4: "CF", 5: "CI", 6: "CR", 7: "DF", 8: "FA", 9: "FC", 10: "FD",
	11: "FM", 12: "HA", 13: "HF", 14: "HM", 15: "IF", 16: "PI", 17: "RF", 18: "TF", 19: "VA",
	20: "VD", 21: "VI", 22: "VM", 23: "VR",
}

// ProbeProcedures asks for the departures and arrivals of an airport with all their legs
// (common route, runway transitions, enroute transitions) and prints them as a tree.
func ProbeProcedures(dllPath, icao string, wait time.Duration) (string, error) {
	f, err := loadFacilities(dllPath)
	if err != nil {
		return "", err
	}
	h, err := f.connectOnly("Open Charts probe")
	if err != nil {
		return "", err
	}
	defer f.close.Call(h)

	add := func(name string) error {
		r, _, _ := f.addFac.Call(h, facilityDefID, cstr(name))
		if e := hr(r); e != nil {
			return fmt.Errorf("define %q: %w", name, e)
		}
		return nil
	}
	addAll := func(names ...string) error {
		for _, n := range names {
			if err := add(n); err != nil {
				return err
			}
		}
		return nil
	}
	leg := func() error {
		if err := add("OPEN APPROACH_LEG"); err != nil {
			return err
		}
		for _, fl := range procLeg {
			if err := add(fl.name); err != nil {
				return err
			}
		}
		return add("CLOSE APPROACH_LEG")
	}
	procedure := func(kind string) error {
		if err := addAll("OPEN "+kind, "NAME", "N_RUNWAY_TRANSITIONS", "N_ENROUTE_TRANSITIONS", "N_APPROACH_LEGS"); err != nil {
			return err
		}
		if err := leg(); err != nil { // the common route
			return err
		}
		if err := addAll("OPEN RUNWAY_TRANSITION", "RUNWAY_NUMBER", "RUNWAY_DESIGNATOR", "N_APPROACH_LEGS"); err != nil {
			return err
		}
		if err := leg(); err != nil {
			return err
		}
		if err := add("CLOSE RUNWAY_TRANSITION"); err != nil {
			return err
		}
		if err := addAll("OPEN ENROUTE_TRANSITION", "NAME", "N_APPROACH_LEGS"); err != nil {
			return err
		}
		if err := leg(); err != nil {
			return err
		}
		if err := add("CLOSE ENROUTE_TRANSITION"); err != nil {
			return err
		}
		return add("CLOSE " + kind)
	}
	if err := addAll("OPEN AIRPORT", "ICAO"); err != nil {
		return "", err
	}
	if err := procedure("DEPARTURE"); err != nil {
		return "", err
	}
	if err := procedure("ARRIVAL"); err != nil {
		return "", err
	}
	if err := add("CLOSE AIRPORT"); err != nil {
		return "", err
	}

	r, _, _ := f.reqFac.Call(h, facilityDefID, facilityReqID, cstr(icao), 0)
	if e := hr(r); e != nil {
		return "", fmt.Errorf("request %s: %w", icao, e)
	}

	var rep strings.Builder
	depth := map[uint32]int{}
	kindOf := map[uint32]uint32{}
	deadline := time.Now().Add(wait)
	quiet := time.Now()
	legs := 0
	for time.Now().Before(deadline) {
		msg := f.next(h)
		if msg == nil {
			if time.Since(quiet) > 6*time.Second {
				break
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		quiet = time.Now()
		switch binary.LittleEndian.Uint32(msg[8:12]) {
		case recvException:
			fmt.Fprintf(&rep, "EXCEPTION %d (send id %d)\n", binary.LittleEndian.Uint32(msg[12:16]), binary.LittleEndian.Uint32(msg[16:20]))
		case recvFacilityDataEnd:
			deadline = time.Now()
		case recvFacilityData:
			if len(msg) < facilityDataOffset {
				continue
			}
			uid := binary.LittleEndian.Uint32(msg[16:20])
			parent := binary.LittleEndian.Uint32(msg[20:24])
			typ := binary.LittleEndian.Uint32(msg[24:28])
			d := depth[parent] + 1
			if parent == 0 {
				d = 0
			}
			depth[uid] = d
			kindOf[uid] = typ
			data := msg[facilityDataOffset:]
			indent := strings.Repeat("  ", d)
			switch typ {
			case facilityTypeAirport:
				fmt.Fprintf(&rep, "%s\n", decodeFields(data, []facField{fs("ICAO", 8)}))
			case facilityTypeDepart, facilityTypeArrival:
				name := map[uint32]string{facilityTypeDepart: "DEPARTURE", facilityTypeArrival: "ARRIVAL"}[typ]
				fmt.Fprintf(&rep, "%s%s %s\n", indent, name, decodeFields(data, []facField{fs("NAME", 8), fi("N_RUNWAY_TRANSITIONS"), fi("N_ENROUTE_TRANSITIONS"), fi("N_APPROACH_LEGS")}))
			case facilityTypeRunwayTrans:
				fmt.Fprintf(&rep, "%sRUNWAY_TRANSITION %s\n", indent, decodeFields(data, []facField{fi("RUNWAY_NUMBER"), fi("RUNWAY_DESIGNATOR"), fi("N_APPROACH_LEGS")}))
			case facilityTypeEnrouteTrans:
				fmt.Fprintf(&rep, "%sENROUTE_TRANSITION %s\n", indent, decodeFields(data, []facField{fs("NAME", 8), fi("N_APPROACH_LEGS")}))
			case facilityTypeLeg:
				legs++
				fmt.Fprintf(&rep, "%sleg %s\n", indent, decodeFields(data, procLeg))
			default:
				fmt.Fprintf(&rep, "%s(type %d)\n", indent, typ)
			}
		}
	}
	fmt.Fprintf(&rep, "legs: %d\n", legs)
	return rep.String(), nil
}
