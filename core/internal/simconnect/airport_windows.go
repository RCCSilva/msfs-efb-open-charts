//go:build windows

package simconnect

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// Facility data types of the levels FetchAirport reads (SIMCONNECT_FACILITY_DATA_TYPE).
const (
	ftAirport    = 0
	ftRunway     = 1
	ftFrequency  = 3
	ftApproach   = 5
	ftApprTrans  = 6
	ftLeg        = 7
	ftFinalLeg   = 8
	ftMissedLeg  = 9
	ftDeparture  = 10
	ftArrival    = 11
	ftRunwayTran = 12
	ftEnrouteTrn = 13
)

var (
	airportFields = []facField{
		fd("LATITUDE"), fd("LONGITUDE"), fd("ALTITUDE"), fs("NAME", 32), fs("ICAO", 8), fs("REGION", 8),
		ff("TRANSITION_ALTITUDE"), ff("TRANSITION_LEVEL"),
	}
	runwayFields = []facField{
		fd("LATITUDE"), fd("LONGITUDE"), fd("ALTITUDE"), ff("HEADING"), ff("LENGTH"), ff("WIDTH"),
		fi("SURFACE"), fi("PRIMARY_NUMBER"), fi("PRIMARY_DESIGNATOR"), fi("SECONDARY_NUMBER"), fi("SECONDARY_DESIGNATOR"),
	}
	frequencyFields = []facField{fi("TYPE"), fi("FREQUENCY"), fs("NAME", 64)}
	approachFields  = []facField{fi("TYPE"), fi("SUFFIX"), fi("RUNWAY_NUMBER"), fi("RUNWAY_DESIGNATOR"), fs("FAF_ICAO", 8)}
	apprTransFields = []facField{fs("NAME", 8), fi("N_APPROACH_LEGS")}
	procedureFields = []facField{fs("NAME", 8), fi("N_RUNWAY_TRANSITIONS"), fi("N_ENROUTE_TRANSITIONS"), fi("N_APPROACH_LEGS")}
	runwayTransFlds = []facField{fi("RUNWAY_NUMBER"), fi("RUNWAY_DESIGNATOR"), fi("N_APPROACH_LEGS")}
	enrouteTransFld = []facField{fs("NAME", 8), fi("N_APPROACH_LEGS")}
)

var facilityMu sync.Mutex // one conversation with the simulator at a time

// fieldReader walks the data of one facility message field by field, in the order they were defined.
type fieldReader struct {
	data []byte
	off  int
}

func (r *fieldReader) f64() float64 {
	if r.off+8 > len(r.data) {
		r.off += 8
		return 0
	}
	v := math.Float64frombits(binary.LittleEndian.Uint64(r.data[r.off:]))
	r.off += 8
	return v
}
func (r *fieldReader) f32() float64 {
	if r.off+4 > len(r.data) {
		r.off += 4
		return 0
	}
	v := float64(math.Float32frombits(binary.LittleEndian.Uint32(r.data[r.off:])))
	r.off += 4
	return v
}
func (r *fieldReader) i32() int {
	if r.off+4 > len(r.data) {
		r.off += 4
		return 0
	}
	v := int(int32(binary.LittleEndian.Uint32(r.data[r.off:])))
	r.off += 4
	return v
}
func (r *fieldReader) str(n int) string {
	if r.off+n > len(r.data) {
		r.off += n
		return ""
	}
	s := r.data[r.off : r.off+n]
	r.off += n
	if i := strings.IndexByte(string(s), 0); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(string(s))
}

func runwayDesignator(d int) string {
	switch d {
	case 1:
		return "L"
	case 2:
		return "R"
	case 3:
		return "C"
	case 4:
		return "W"
	case 5:
		return "A"
	case 6:
		return "B"
	}
	return ""
}

func runwayName(num, des int) string {
	if num <= 0 {
		return ""
	}
	return fmt.Sprintf("%02d%s", num, runwayDesignator(des))
}

func readLeg(r *fieldReader) navdata.Leg {
	var l navdata.Leg
	l.Type = r.i32()
	if ft := r.i32(); ft > 0 && ft < 128 {
		l.FixType = string(rune(ft))
	}
	l.FixRegion = r.str(8)
	l.FixICAO = r.str(8)
	l.FlyOver = r.i32() != 0
	r.i32() // DISTANCE_MINUTE
	r.i32() // TRUE_DEGREE
	turn := r.i32()
	l.TurnLeft, l.TurnRight = turn == 1, turn == 2 // 1 left, 2 right (0 none, 3 either): as the SDK lists them
	r.str(8)                                       // ORIGIN_ICAO
	r.str(8)                                       // ORIGIN_REGION
	r.i32()                                        // ORIGIN_TYPE
	r.f32()                                        // THETA
	r.f32()                                        // RHO
	l.Course = r.f32()
	l.DistanceNm = r.f32()
	l.AltDesc = r.i32()
	l.Altitude1M = r.f32()
	l.Altitude2M = r.f32()
	l.SpeedLimit = r.f32()
	l.VerticalDeg = r.f32()
	return l
}

type procBuild struct {
	p     navdata.Procedure
	trans []*navdata.Transition
}

// FetchAirport asks the running simulator for one airport: position, runways, frequencies and all
// procedures (departures, arrivals, approaches) with their legs. It does not need the aircraft to
// be anywhere near the airport.
func FetchAirport(dllPath, icao string, wait time.Duration) (*navdata.Airport, error) {
	facilityMu.Lock()
	defer facilityMu.Unlock()

	f, err := loadFacilities(dllPath)
	if err != nil {
		return nil, err
	}
	h, err := f.connectOnly("Open Charts navdata")
	if err != nil {
		return nil, fmt.Errorf("the simulator is not reachable: %w", err)
	}
	defer f.close.Call(h)

	add := func(names ...string) error {
		for _, n := range names {
			r, _, _ := f.addFac.Call(h, facilityDefID, cstr(n))
			if e := hr(r); e != nil {
				return fmt.Errorf("define %q: %w", n, e)
			}
		}
		return nil
	}
	level := func(open, close string, fields []facField, children func() error) error {
		if err := add("OPEN " + open); err != nil {
			return err
		}
		for _, fl := range fields {
			if err := add(fl.name); err != nil {
				return err
			}
		}
		if children != nil {
			if err := children(); err != nil {
				return err
			}
		}
		return add("CLOSE " + close)
	}
	leg := func(kind string) error { return level(kind, kind, procLeg, nil) }
	proc := func(kind string) error {
		return level(kind, kind, procedureFields, func() error {
			if err := leg("APPROACH_LEG"); err != nil {
				return err
			}
			if err := level("RUNWAY_TRANSITION", "RUNWAY_TRANSITION", runwayTransFlds, func() error { return leg("APPROACH_LEG") }); err != nil {
				return err
			}
			return level("ENROUTE_TRANSITION", "ENROUTE_TRANSITION", enrouteTransFld, func() error { return leg("APPROACH_LEG") })
		})
	}
	err = level("AIRPORT", "AIRPORT", airportFields, func() error {
		if err := level("RUNWAY", "RUNWAY", runwayFields, nil); err != nil {
			return err
		}
		if err := level("FREQUENCY", "FREQUENCY", frequencyFields, nil); err != nil {
			return err
		}
		if err := level("APPROACH", "APPROACH", approachFields, func() error {
			if err := level("APPROACH_TRANSITION", "APPROACH_TRANSITION", apprTransFields, func() error { return leg("APPROACH_LEG") }); err != nil {
				return err
			}
			if err := leg("FINAL_APPROACH_LEG"); err != nil {
				return err
			}
			return leg("MISSED_APPROACH_LEG")
		}); err != nil {
			return err
		}
		if err := proc("DEPARTURE"); err != nil {
			return err
		}
		return proc("ARRIVAL")
	})
	if err != nil {
		return nil, err
	}

	r, _, _ := f.reqFac.Call(h, facilityDefID, facilityReqID, cstr(strings.ToUpper(icao)), 0)
	if e := hr(r); e != nil {
		return nil, fmt.Errorf("request %s: %w", icao, e)
	}

	a := &navdata.Airport{Source: "msfs-simconnect", FetchedAt: time.Now()}
	var procs []*procBuild
	procOf := map[uint32]*procBuild{}           // unique id of an approach/departure/arrival
	transOf := map[uint32]*navdata.Transition{} // unique id of a transition level
	special := map[uint32]map[string]*navdata.Transition{}
	gotAirport := false
	var failure string

	deadline := time.Now().Add(wait)
	quiet := time.Now()
	finished := false
	for !finished && time.Now().Before(deadline) {
		msg := f.next(h)
		if msg == nil {
			if time.Since(quiet) > 8*time.Second {
				break
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
		quiet = time.Now()
		switch binary.LittleEndian.Uint32(msg[8:12]) {
		case recvException:
			failure = fmt.Sprintf("simulator exception %d (request %d)", binary.LittleEndian.Uint32(msg[12:16]), binary.LittleEndian.Uint32(msg[16:20]))
		case recvFacilityDataEnd:
			finished = true
		case recvFacilityData:
			if len(msg) < facilityDataOffset {
				continue
			}
			uid := binary.LittleEndian.Uint32(msg[16:20])
			parent := binary.LittleEndian.Uint32(msg[20:24])
			typ := binary.LittleEndian.Uint32(msg[24:28])
			rd := &fieldReader{data: msg[facilityDataOffset:]}
			switch typ {
			case ftAirport:
				gotAirport = true
				a.Lat, a.Lon, a.AltitudeM = rd.f64(), rd.f64(), rd.f64()
				a.Name = rd.str(32)
				a.ICAO = rd.str(8)
				a.Region = rd.str(8)
				a.TransAltM, a.TransLevel = rd.f32(), rd.f32()
			case ftRunway:
				var r navdata.Runway
				r.Lat, r.Lon, r.AltitudeM = rd.f64(), rd.f64(), rd.f64()
				r.HeadingTrue, r.LengthM, r.WidthM = rd.f32(), rd.f32(), rd.f32()
				r.Surface = rd.i32()
				pn, pd, sn, sd := rd.i32(), rd.i32(), rd.i32(), rd.i32()
				r.PrimaryName, r.SecondaryNam = runwayName(pn, pd), runwayName(sn, sd)
				a.Runways = append(a.Runways, r)
			case ftFrequency:
				var fr navdata.Frequency
				fr.Type = rd.i32()
				fr.Hz = int64(rd.i32())
				fr.Name = rd.str(64)
				a.Frequencies = append(a.Frequencies, fr)
			case ftApproach:
				pb := &procBuild{p: navdata.Procedure{Kind: navdata.KindApproach}}
				pb.p.ApproachType = rd.i32()
				pb.p.Suffix = rd.i32()
				pn, pd := rd.i32(), rd.i32()
				pb.p.Runway = runwayName(pn, pd)
				pb.p.FAF = rd.str(8)
				procs = append(procs, pb)
				procOf[uid] = pb
			case ftDeparture, ftArrival:
				kind := navdata.KindSID
				if typ == ftArrival {
					kind = navdata.KindSTAR
				}
				pb := &procBuild{p: navdata.Procedure{Kind: kind, Name: rd.str(8)}}
				procs = append(procs, pb)
				procOf[uid] = pb
			case ftRunwayTran:
				if pb := procOf[parent]; pb != nil {
					num, des := rd.i32(), rd.i32()
					t := &navdata.Transition{Kind: navdata.TransRunway, Runway: runwayName(num, des)}
					pb.trans = append(pb.trans, t)
					transOf[uid] = t
				}
			case ftEnrouteTrn, ftApprTrans:
				if pb := procOf[parent]; pb != nil {
					t := &navdata.Transition{Kind: navdata.TransEnroute, Name: rd.str(8)}
					pb.trans = append(pb.trans, t)
					transOf[uid] = t
				}
			case ftLeg, ftFinalLeg, ftMissedLeg:
				var t *navdata.Transition
				switch {
				case transOf[parent] != nil: // under a runway/enroute/approach transition
					t = transOf[parent]
				case procOf[parent] != nil: // directly under the procedure
					pb := procOf[parent]
					kind := navdata.TransCommon
					if typ == ftFinalLeg {
						kind = navdata.TransFinal
					} else if typ == ftMissedLeg {
						kind = navdata.TransMissed
					}
					if special[parent] == nil {
						special[parent] = map[string]*navdata.Transition{}
					}
					if t = special[parent][kind]; t == nil {
						t = &navdata.Transition{Kind: kind}
						special[parent][kind] = t
						pb.trans = append(pb.trans, t)
					}
				}
				if t != nil {
					t.Legs = append(t.Legs, readLeg(rd))
				}
			}
		}
	}
	if !gotAirport {
		if failure != "" {
			return nil, fmt.Errorf("airport %s: %s", icao, failure)
		}
		return nil, fmt.Errorf("airport %s: the simulator has no such airport", strings.ToUpper(icao))
	}
	for _, pb := range procs {
		for _, t := range pb.trans {
			pb.p.Transitions = append(pb.p.Transitions, *t)
		}
		a.Procedures = append(a.Procedures, pb.p)
	}
	return a, nil
}
