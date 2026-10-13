//go:build windows

package simconnect

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// Facilities: what the simulator itself knows about airports and navaids, asked through the
// SimConnect facility API (the same way Little Navmap loads the MSFS 2024 scenery library).
// This file is a probe for now: it asks and reports what comes back.

const (
	recvFacilityData     = 28
	recvFacilityDataEnd  = 29
	recvFacilityMinimal  = 30
	recvAirportList      = 18
	recvWaypointList     = 21
	recvNDBList          = 20
	recvVORList          = 19
	facilityDefID        = 10
	facilityReqID        = 10
	facilityDataOffset   = recvHeader + 7*4 // header + 7 DWORDs (ids, type, list item, index, size)
	listEntryOffset      = recvHeader + 4*4 // header + RequestID, ArraySize, EntryNumber, OutOf
	facilityTypeAirport  = 0
	facilityTypeRunway   = 1
	facilityTypeFrequncy = 3
	facilityTypeApproach = 5
	facilityTypeFinalLeg = 8
	facilityTypeDepart   = 10
	facilityTypeArrival  = 11
)

type facField struct {
	name string
	kind byte // 'd' float64, 'f' float32, 'i' int32, 's' string (size)
	size int
}

func fd(n string) facField           { return facField{n, 'd', 8} }
func ff(n string) facField           { return facField{n, 'f', 4} }
func fi(n string) facField           { return facField{n, 'i', 4} }
func fs(n string, size int) facField { return facField{n, 's', size} }

// the fields asked for each level, in the order the simulator answers them
var facLevels = []struct {
	open, close string
	typ         uint32
	fields      []facField
}{
	{"OPEN AIRPORT", "", facilityTypeAirport, []facField{
		fd("LATITUDE"), fd("LONGITUDE"), fd("ALTITUDE"), fs("NAME", 32), fs("ICAO", 8), fs("REGION", 8),
		ff("TRANSITION_ALTITUDE"), ff("TRANSITION_LEVEL"),
		fi("N_RUNWAYS"), fi("N_FREQUENCIES"), fi("N_APPROACHES"), fi("N_DEPARTURES"), fi("N_ARRIVALS"),
	}},
	{"OPEN RUNWAY", "CLOSE RUNWAY", facilityTypeRunway, []facField{
		fd("LATITUDE"), fd("LONGITUDE"), fd("ALTITUDE"), ff("HEADING"), ff("LENGTH"), ff("WIDTH"),
		fi("SURFACE"), fi("PRIMARY_NUMBER"), fi("PRIMARY_DESIGNATOR"), fi("SECONDARY_NUMBER"), fi("SECONDARY_DESIGNATOR"),
	}},
	{"OPEN FREQUENCY", "CLOSE FREQUENCY", facilityTypeFrequncy, []facField{
		fi("TYPE"), fi("FREQUENCY"), fs("NAME", 64),
	}},
	{"OPEN APPROACH", "", facilityTypeApproach, []facField{
		fi("TYPE"), fi("SUFFIX"), fi("RUNWAY_NUMBER"), fi("RUNWAY_DESIGNATOR"), fs("FAF_ICAO", 8),
	}},
	{"OPEN FINAL_APPROACH_LEG", "CLOSE FINAL_APPROACH_LEG", facilityTypeFinalLeg, []facField{
		fi("TYPE"), fs("FIX_ICAO", 8), fi("FIX_TYPE"), ff("ALTITUDE1"),
	}},
	{"OPEN DEPARTURE", "CLOSE DEPARTURE", facilityTypeDepart, []facField{fs("NAME", 8)}},
	{"OPEN ARRIVAL", "CLOSE ARRIVAL", facilityTypeArrival, []facField{fs("NAME", 8)}},
}

type facAPI struct {
	*api
	addFac, reqFac, reqFacEx1, reqList, reqListEx1 *syscall.LazyProc
	dll                                            *syscall.LazyDLL
}

func loadFacilities(path string) (*facAPI, error) {
	d := syscall.NewLazyDLL(path)
	if err := d.Load(); err != nil {
		return nil, err
	}
	a := &api{
		open: d.NewProc("SimConnect_Open"), close: d.NewProc("SimConnect_Close"),
		nextDispatch: d.NewProc("SimConnect_GetNextDispatch"),
	}
	f := &facAPI{api: a, dll: d,
		addFac:     d.NewProc("SimConnect_AddToFacilityDefinition"),
		reqFac:     d.NewProc("SimConnect_RequestFacilityData"),
		reqFacEx1:  d.NewProc("SimConnect_RequestFacilityData_EX1"),
		reqList:    d.NewProc("SimConnect_RequestFacilitiesList"),
		reqListEx1: d.NewProc("SimConnect_RequestFacilitiesList_EX1"),
	}
	return f, nil
}

func cstr(s string) uintptr {
	p, _ := syscall.BytePtrFromString(s)
	return uintptr(unsafe.Pointer(p))
}

func decodeFields(data []byte, fields []facField) string {
	var out []string
	off := 0
	for _, f := range fields {
		if off+f.size > len(data) {
			out = append(out, f.name+"=<short>")
			break
		}
		switch f.kind {
		case 'd':
			out = append(out, fmt.Sprintf("%s=%.6f", f.name, math.Float64frombits(binary.LittleEndian.Uint64(data[off:]))))
		case 'f':
			out = append(out, fmt.Sprintf("%s=%.2f", f.name, math.Float32frombits(binary.LittleEndian.Uint32(data[off:]))))
		case 'i':
			out = append(out, fmt.Sprintf("%s=%d", f.name, int32(binary.LittleEndian.Uint32(data[off:]))))
		case 's':
			s := data[off : off+f.size]
			if i := strings.IndexByte(string(s), 0); i >= 0 {
				s = s[:i]
			}
			out = append(out, fmt.Sprintf("%s=%q", f.name, string(s)))
		}
		off += f.size
	}
	return strings.Join(out, " ")
}

// ProbeAirport asks the running simulator for one airport and returns a readable report.
func ProbeAirport(dllPath, icao string, wait time.Duration) (string, error) {
	f, err := loadFacilities(dllPath)
	if err != nil {
		return "", err
	}
	h, err := f.connectOnly("Open Charts probe")
	if err != nil {
		return "", err
	}
	defer f.close.Call(h)

	var rep strings.Builder
	typeName := map[uint32]string{}
	typeFields := map[uint32][]facField{}
	for _, l := range facLevels {
		typeName[l.typ] = strings.TrimPrefix(l.open, "OPEN ")
		typeFields[l.typ] = l.fields
	}

	// the definition: AIRPORT { RUNWAY, FREQUENCY, APPROACH { FINAL_APPROACH_LEG }, DEPARTURE, ARRIVAL }
	add := func(name string) error {
		r, _, _ := f.addFac.Call(h, facilityDefID, cstr(name))
		if e := hr(r); e != nil {
			return fmt.Errorf("define %q: %w", name, e)
		}
		return nil
	}
	for _, l := range facLevels {
		if l.typ == facilityTypeFinalLeg {
			// the legs live inside the approach
		}
		if err := add(l.open); err != nil {
			return rep.String(), err
		}
		for _, fl := range l.fields {
			if err := add(fl.name); err != nil {
				return rep.String(), err
			}
		}
		if l.close != "" {
			if err := add(l.close); err != nil {
				return rep.String(), err
			}
		}
		if l.typ == facilityTypeFinalLeg {
			if err := add("CLOSE APPROACH"); err != nil {
				return rep.String(), err
			}
		}
	}
	if err := add("CLOSE AIRPORT"); err != nil {
		return rep.String(), err
	}

	r, _, _ := f.reqFac.Call(h, facilityDefID, facilityReqID, cstr(icao), 0)
	if e := hr(r); e != nil {
		return rep.String(), fmt.Errorf("request %s: %w", icao, e)
	}
	fmt.Fprintf(&rep, "requested %s\n", icao)

	counts := map[uint32]int{}
	deadline := time.Now().Add(wait)
	quiet := time.Now()
	for time.Now().Before(deadline) {
		msg := f.next(h)
		if msg == nil {
			if time.Since(quiet) > 6*time.Second {
				fmt.Fprintf(&rep, "silent for 6 s: stop\n")
				break
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		quiet = time.Now()
		id := binary.LittleEndian.Uint32(msg[8:12])
		switch id {
		case recvException:
			fmt.Fprintf(&rep, "EXCEPTION %d (send id %d, index %d)\n", binary.LittleEndian.Uint32(msg[12:16]),
				binary.LittleEndian.Uint32(msg[16:20]), binary.LittleEndian.Uint32(msg[20:24]))
		case recvFacilityData:
			if len(msg) < facilityDataOffset {
				continue
			}
			typ := binary.LittleEndian.Uint32(msg[12+12 : 12+16])
			counts[typ]++
			n := counts[typ]
			if n <= 4 || typ == facilityTypeAirport || typ == facilityTypeDepart || typ == facilityTypeArrival || typ == facilityTypeApproach || typ == facilityTypeRunway || typ == facilityTypeFrequncy {
				fmt.Fprintf(&rep, "%-22s %s\n", typeName[typ], decodeFields(msg[facilityDataOffset:], typeFields[typ]))
			}
		case recvFacilityDataEnd:
			fmt.Fprintf(&rep, "END\n")
			deadline = time.Now()
		case recvOpen, recvSimObjData:
		default:
			fmt.Fprintf(&rep, "message id %d (%d bytes)\n", id, len(msg))
		}
	}
	fmt.Fprintf(&rep, "counts by type:")
	for typ, n := range counts {
		fmt.Fprintf(&rep, " %s=%d", typeName[typ], n)
	}
	fmt.Fprintln(&rep)
	return rep.String(), nil
}

// connectOnly opens a connection without the aircraft definition.
func (f *facAPI) connectOnly(name string) (uintptr, error) {
	var h uintptr
	r, _, _ := f.open.Call(uintptr(unsafe.Pointer(&h)), cstr(name), 0, 0, 0, 0)
	if err := hr(r); err != nil {
		return 0, err
	}
	return h, nil
}

// ProbeList asks for every facility of a kind ("airport", "waypoint", "ndb", "vor") and counts them.
func ProbeList(dllPath, kind string, wait time.Duration) (string, error) {
	f, err := loadFacilities(dllPath)
	if err != nil {
		return "", err
	}
	h, err := f.connectOnly("Open Charts probe")
	if err != nil {
		return "", err
	}
	defer f.close.Call(h)
	listType := map[string]uintptr{"airport": 0, "waypoint": 1, "ndb": 2, "vor": 3}[kind]
	var rep strings.Builder
	started := time.Now()
	proc := f.reqList
	if strings.HasPrefix(kind, "ex1-") {
		kind = strings.TrimPrefix(kind, "ex1-")
		listType = map[string]uintptr{"airport": 0, "waypoint": 1, "ndb": 2, "vor": 3}[kind]
		proc = f.reqListEx1
	}
	r, _, _ := proc.Call(h, listType, 30)
	if e := hr(r); e != nil {
		return "", fmt.Errorf("RequestFacilitiesList: %w", e)
	}
	got, outOf := 0, 0
	deadline := time.Now().Add(wait)
	quiet := time.Now()
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
		id := binary.LittleEndian.Uint32(msg[8:12])
		switch id {
		case recvException:
			fmt.Fprintf(&rep, "EXCEPTION %d\n", binary.LittleEndian.Uint32(msg[12:16]))
		case recvAirportList, recvWaypointList, recvNDBList, recvVORList, recvFacilityMinimal:
			if len(msg) < listEntryOffset {
				continue
			}
			size := int(binary.LittleEndian.Uint32(msg[16:20]))
			got += size
			outOf = int(binary.LittleEndian.Uint32(msg[24:28]))
			if got == size { // first chunk: show a sample
				fmt.Fprintf(&rep, "message id %d, first chunk %d entries, out of %d\n", id, size, outOf)
			}
		case recvOpen, recvSimObjData:
		default:
			fmt.Fprintf(&rep, "message id %d (%d bytes)\n", id, len(msg))
		}
		if outOf > 0 && got >= outOf {
			break
		}
	}
	fmt.Fprintf(&rep, "%s: %d received, %d announced, in %.1f s\n", kind, got, outOf, time.Since(started).Seconds())
	return rep.String(), nil
}

// ProbeFix asks for one waypoint/VOR/NDB (kind 'W', 'V', 'N') and the airways that pass through it.
func ProbeFix(dllPath, ident, region string, kind byte, wait time.Duration) (string, error) {
	f, err := loadFacilities(dllPath)
	if err != nil {
		return "", err
	}
	h, err := f.connectOnly("Open Charts probe")
	if err != nil {
		return "", err
	}
	defer f.close.Call(h)
	if err := f.reqFacEx1.Find(); err != nil {
		return "", fmt.Errorf("this SimConnect.dll has no RequestFacilityData_EX1: %w", err)
	}
	open := map[byte]string{'W': "WAYPOINT", 'V': "VOR", 'N': "NDB"}[kind]
	if open == "" {
		return "", fmt.Errorf("kind must be W, V or N")
	}
	node := []facField{fd("LATITUDE"), fd("LONGITUDE"), fd("ALTITUDE"), ff("MAGVAR"), fi("N_ROUTES")}
	switch kind {
	case 'V':
		node = []facField{fd("VOR_LATITUDE"), fd("VOR_LONGITUDE"), fd("VOR_ALTITUDE"), fi("FREQUENCY"), fi("TYPE"),
			ff("NAV_RANGE"), ff("MAGVAR"), fs("NAME", 32), fs("ICAO", 8), fs("REGION", 8)}
	case 'N':
		node = []facField{fd("LATITUDE"), fd("LONGITUDE"), fd("ALTITUDE"), fi("FREQUENCY"), fi("TYPE"),
			ff("RANGE"), ff("MAGVAR"), fs("NAME", 32)}
	}
	route := []facField{
		fs("NAME", 32), fi("TYPE"),
		fs("NEXT_ICAO", 8), fs("NEXT_REGION", 8), fi("NEXT_TYPE"), fd("NEXT_LATITUDE"), fd("NEXT_LONGITUDE"), ff("NEXT_ALTITUDE"),
		fs("PREV_ICAO", 8), fs("PREV_REGION", 8), fi("PREV_TYPE"), fd("PREV_LATITUDE"), fd("PREV_LONGITUDE"), ff("PREV_ALTITUDE"),
	}
	add := func(name string) error {
		r, _, _ := f.addFac.Call(h, facilityDefID, cstr(name))
		if e := hr(r); e != nil {
			return fmt.Errorf("define %q: %w", name, e)
		}
		return nil
	}
	names := []string{"OPEN " + open}
	for _, n := range node {
		names = append(names, n.name)
	}
	if kind == 'W' {
		names = append(names, "OPEN ROUTE")
		for _, n := range route {
			names = append(names, n.name)
		}
		names = append(names, "CLOSE ROUTE")
	}
	names = append(names, "CLOSE "+open)
	for _, n := range names {
		if err := add(n); err != nil {
			return "", err
		}
	}
	var rep strings.Builder
	r, _, _ := f.reqFacEx1.Call(h, facilityDefID, facilityReqID, cstr(ident), cstr(region), uintptr(kind))
	if e := hr(r); e != nil {
		return "", fmt.Errorf("request %s: %w", ident, e)
	}
	fmt.Fprintf(&rep, "requested %s %s kind %c\n", ident, region, kind)
	deadline := time.Now().Add(wait)
	quiet := time.Now()
	routes := 0
	for time.Now().Before(deadline) {
		msg := f.next(h)
		if msg == nil {
			if time.Since(quiet) > 6*time.Second {
				fmt.Fprintf(&rep, "silent for 6 s: stop\n")
				break
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		quiet = time.Now()
		id := binary.LittleEndian.Uint32(msg[8:12])
		switch id {
		case recvException:
			fmt.Fprintf(&rep, "EXCEPTION %d (send id %d, index %d)\n", binary.LittleEndian.Uint32(msg[12:16]),
				binary.LittleEndian.Uint32(msg[16:20]), binary.LittleEndian.Uint32(msg[20:24]))
		case recvFacilityData:
			if len(msg) < facilityDataOffset {
				continue
			}
			typ := binary.LittleEndian.Uint32(msg[24:28])
			fields := node
			label := open
			if typ == 22 { // ROUTE
				fields, label = route, "ROUTE"
				routes++
			}
			if typ == 22 && routes == 1 {
				d := msg[facilityDataOffset:]
				if len(d) > 160 {
					d = d[:160]
				}
				fmt.Fprintf(&rep, "raw route (%d bytes): %x\n", len(msg)-facilityDataOffset, d)
			}
			if typ != 22 || routes <= 12 {
				fmt.Fprintf(&rep, "%-9s %s\n", label, decodeFields(msg[facilityDataOffset:], fields))
			}
		case recvFacilityDataEnd:
			fmt.Fprintf(&rep, "END\n")
			deadline = time.Now()
		case recvOpen, recvSimObjData:
		default:
			fmt.Fprintf(&rep, "message id %d (%d bytes)\n", id, len(msg))
		}
	}
	fmt.Fprintf(&rep, "routes: %d\n", routes)
	return rep.String(), nil
}

// ProbeNames defines "OPEN <open>" followed by each name and reports the names the simulator
// rejected (exception 20). It is how the field names of a facility type are found out.
func ProbeNames(dllPath, open string, names []string, wait time.Duration) (string, error) {
	f, err := loadFacilities(dllPath)
	if err != nil {
		return "", err
	}
	h, err := f.connectOnly("Open Charts probe")
	if err != nil {
		return "", err
	}
	defer f.close.Call(h)
	var all []string
	for _, part := range strings.Split(open, ">") {
		all = append(all, "OPEN "+part)
	}
	all = append(all, names...)
	for _, n := range all {
		r, _, _ := f.addFac.Call(h, facilityDefID, cstr(n))
		if e := hr(r); e != nil {
			return "", fmt.Errorf("define %q: %w", n, e)
		}
	}
	bad := map[int]bool{}
	deadline := time.Now().Add(wait)
	quiet := time.Now()
	for time.Now().Before(deadline) {
		msg := f.next(h)
		if msg == nil {
			if time.Since(quiet) > 2*time.Second {
				break
			}
			time.Sleep(20 * time.Millisecond)
			continue
		}
		quiet = time.Now()
		if binary.LittleEndian.Uint32(msg[8:12]) == recvException {
			bad[int(binary.LittleEndian.Uint32(msg[16:20]))] = true
		}
	}
	var rep strings.Builder
	for i, n := range all {
		state := "ok"
		if bad[i+2] { // the first definition call is send id 2
			state = "REJECTED"
		}
		fmt.Fprintf(&rep, "%-34s %s\n", n, state)
	}
	return rep.String(), nil
}
