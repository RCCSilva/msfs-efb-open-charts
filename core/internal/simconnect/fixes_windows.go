//go:build windows

package simconnect

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// Facility data types of fixes.
const (
	ftVOR      = 19
	ftNDB      = 20
	ftWaypoint = 21
	ftRoute    = 22
)

var (
	fixDefIDs = map[byte]uintptr{navdata.FixWaypoint: 21, navdata.FixVOR: 22, navdata.FixNDB: 23}

	waypointFields = []facField{fd("LATITUDE"), fd("LONGITUDE"), fd("ALTITUDE"), ff("MAGVAR"), fi("N_ROUTES")}
	vorFields      = []facField{fd("VOR_LATITUDE"), fd("VOR_LONGITUDE"), fd("VOR_ALTITUDE"), fi("FREQUENCY"), fi("TYPE"),
		ff("NAV_RANGE"), ff("MAGVAR"), fs("NAME", 32)}
	ndbFields = []facField{fd("LATITUDE"), fd("LONGITUDE"), fd("ALTITUDE"), fi("FREQUENCY"), fi("TYPE"),
		ff("RANGE"), ff("MAGVAR"), fs("NAME", 32)}
	routeFields = []facField{
		fs("NAME", 32), fi("TYPE"),
		fs("NEXT_ICAO", 8), fs("NEXT_REGION", 8), fi("NEXT_TYPE"), fd("NEXT_LATITUDE"), fd("NEXT_LONGITUDE"), ff("NEXT_ALTITUDE"),
		fs("PREV_ICAO", 8), fs("PREV_REGION", 8), fi("PREV_TYPE"), fd("PREV_LATITUDE"), fd("PREV_LONGITUDE"), ff("PREV_ALTITUDE"),
	}
)

func fixKindByte(code int) byte {
	if code > 0 && code < 128 {
		return byte(code)
	}
	return 0
}

// FetchFixes asks the simulator for waypoints, VORs and NDBs by identifier and region. Fixes the
// simulator does not know are simply missing from the result. Requests go in batches, and each
// batch is answered before the next is sent (the simulator drops what it cannot keep up with).
func FetchFixes(dllPath string, refs []navdata.FixRef, wait time.Duration) ([]navdata.Fix, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	facilityMu.Lock()
	defer facilityMu.Unlock()

	f, err := loadFacilities(dllPath)
	if err != nil {
		return nil, err
	}
	if err := f.reqFacEx1.Find(); err != nil {
		return nil, fmt.Errorf("this SimConnect.dll cannot ask for fixes (RequestFacilityData_EX1): %w", err)
	}
	h, err := f.connectOnly("Open Charts navdata")
	if err != nil {
		return nil, fmt.Errorf("the simulator is not reachable: %w", err)
	}
	defer f.close.Call(h)

	// one definition per kind
	add := func(def uintptr, names ...string) error {
		for _, n := range names {
			r, _, _ := f.addFac.Call(h, def, cstr(n))
			if e := hr(r); e != nil {
				return fmt.Errorf("define %q: %w", n, e)
			}
		}
		return nil
	}
	fields := func(def uintptr, open string, fl []facField) error {
		if err := add(def, "OPEN "+open); err != nil {
			return err
		}
		for _, x := range fl {
			if err := add(def, x.name); err != nil {
				return err
			}
		}
		return nil
	}
	w := fixDefIDs[navdata.FixWaypoint]
	if err := fields(w, "WAYPOINT", waypointFields); err != nil {
		return nil, err
	}
	if err := fields(w, "ROUTE", routeFields); err != nil {
		return nil, err
	}
	if err := add(w, "CLOSE ROUTE", "CLOSE WAYPOINT"); err != nil {
		return nil, err
	}
	v := fixDefIDs[navdata.FixVOR]
	if err := fields(v, "VOR", vorFields); err != nil {
		return nil, err
	}
	if err := add(v, "CLOSE VOR"); err != nil {
		return nil, err
	}
	n := fixDefIDs[navdata.FixNDB]
	if err := fields(n, "NDB", ndbFields); err != nil {
		return nil, err
	}
	if err := add(n, "CLOSE NDB"); err != nil {
		return nil, err
	}

	const batch = 400
	var out []navdata.Fix
	deadline := time.Now().Add(wait)
	for start := 0; start < len(refs); start += batch {
		end := min(len(refs), start+batch)
		chunk, err := fetchFixBatch(f, h, refs[start:end], start, deadline)
		out = append(out, chunk...)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

func fetchFixBatch(f *facAPI, h uintptr, refs []navdata.FixRef, base int, deadline time.Time) ([]navdata.Fix, error) {
	reqOf := map[uint32]int{} // request id -> index in refs
	pending := 0
	for i, ref := range refs {
		def, ok := fixDefIDs[ref.Kind]
		if !ok {
			continue
		}
		rid := uint32(1000 + base + i)
		r, _, _ := f.reqFacEx1.Call(h, def, uintptr(rid), cstr(ref.Ident), cstr(ref.Region), uintptr(ref.Kind))
		if e := hr(r); e != nil {
			continue
		}
		reqOf[rid] = i
		pending++
	}
	fixes := map[int]*navdata.Fix{}
	owner := map[uint32]int{} // unique id of a waypoint item -> index in refs
	ended := map[uint32]bool{}
	exceptions := 0 // an unknown fix answers with an exception and no end message
	quiet := time.Now()
	for len(ended)+exceptions < pending && time.Now().Before(deadline) {
		msg := f.next(h)
		if msg == nil {
			if time.Since(quiet) > 6*time.Second {
				break
			}
			time.Sleep(5 * time.Millisecond)
			continue
		}
		quiet = time.Now()
		switch binary.LittleEndian.Uint32(msg[8:12]) {
		case recvFacilityDataEnd:
			if len(msg) >= 16 {
				ended[binary.LittleEndian.Uint32(msg[12:16])] = true
			}
		case recvException:
			exceptions++
		case recvFacilityData:
			if len(msg) < facilityDataOffset {
				continue
			}
			rid := binary.LittleEndian.Uint32(msg[12:16])
			uid := binary.LittleEndian.Uint32(msg[16:20])
			parent := binary.LittleEndian.Uint32(msg[20:24])
			typ := binary.LittleEndian.Uint32(msg[24:28])
			rd := &fieldReader{data: msg[facilityDataOffset:]}
			switch typ {
			case ftWaypoint, ftVOR, ftNDB:
				i, ok := reqOf[rid]
				if !ok {
					continue
				}
				ref := refs[i]
				fx := &navdata.Fix{Ident: ref.Ident, Region: ref.Region, Kind: ref.Kind}
				fx.Lat, fx.Lon, fx.AltM = rd.f64(), rd.f64(), rd.f64()
				if typ != ftWaypoint {
					fx.FreqHz = int64(rd.i32())
					rd.i32() // TYPE
					rd.f32() // range
					rd.f32() // MAGVAR
					fx.Name = rd.str(32)
				}
				fixes[i] = fx
				owner[uid] = i
			case ftRoute:
				i, ok := owner[parent]
				if !ok || fixes[i] == nil {
					continue
				}
				var r navdata.Route
				r.Airway = rd.str(32)
				r.Type = rd.i32()
				r.NextIdent, r.NextRegion = rd.str(8), rd.str(8)
				r.NextKind = fixKindByte(rd.i32())
				r.NextLat, r.NextLon = rd.f64(), rd.f64()
				rd.f32()
				r.PrevIdent, r.PrevRegion = rd.str(8), rd.str(8)
				r.PrevKind = fixKindByte(rd.i32())
				r.PrevLat, r.PrevLon = rd.f64(), rd.f64()
				fixes[i].Routes = append(fixes[i].Routes, r)
			}
		}
	}
	out := make([]navdata.Fix, 0, len(fixes))
	for i := range refs {
		if fx := fixes[i]; fx != nil && (fx.Lat != 0 || fx.Lon != 0) {
			out = append(out, *fx)
		}
	}
	return out, nil
}
