//go:build windows

package simconnect

import (
	"fmt"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// LookupFixes finds what an identifier means in the simulator, without knowing its kind or region.
// The simulator needs the kind in the question, so each identifier is asked as a waypoint, a VOR
// and an NDB. An identifier that exists in more than one place is answered with a list of
// candidates, and every candidate is then asked for in full (with its airways).
//
// An identifier the simulator does not know is simply absent from the result.
func LookupFixes(dllPath string, idents []string, wait time.Duration) ([]navdata.Fix, error) {
	var asks []navdata.FixRef
	for _, id := range idents {
		for _, k := range []byte{navdata.FixWaypoint, navdata.FixVOR, navdata.FixNDB} {
			asks = append(asks, navdata.FixRef{Ident: id, Kind: k})
		}
	}
	return LookupFixKinds(dllPath, asks, wait)
}

// LookupFixKinds is LookupFixes for when the kinds to ask for each identifier are known: only those
// are asked (each ref is an identifier and a kind, with no region).
func LookupFixKinds(dllPath string, first []navdata.FixRef, wait time.Duration) ([]navdata.Fix, error) {
	if len(first) == 0 {
		return nil, nil
	}
	facilityMu.Lock()
	defer facilityMu.Unlock()
	tr := newTracer("lookup")

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
	if err := defineFixes(f, h); err != nil {
		return nil, err
	}
	tr.mark("connect+define")

	deadline := time.Now().Add(wait)
	var out []navdata.Fix
	var second []navdata.FixRef
	asked := map[navdata.FixRef]bool{}
	const batch = 300
	for start := 0; start < len(first); start += batch {
		end := min(len(first), start+batch)
		got, amb, err := fetchFixBatch(f, h, first[start:end], start, deadline)
		if err != nil {
			return out, err
		}
		out = append(out, got...)
		for _, cands := range amb {
			for _, c := range cands {
				ref := navdata.FixRef{Ident: c.Ident, Region: c.Region, Kind: c.Kind}
				if !asked[ref] { // the lists of the three kinds repeat the same candidates
					asked[ref] = true
					second = append(second, ref)
				}
			}
		}
	}
	tr.mark(fmt.Sprintf("round 1 (%d requests)", len(first)))
	for start := 0; start < len(second); start += batch {
		end := min(len(second), start+batch)
		got, _, err := fetchFixBatch(f, h, second[start:end], 100000+start, deadline)
		if err != nil {
			return out, err
		}
		out = append(out, got...)
	}
	tr.mark(fmt.Sprintf("round 2 (%d requests)", len(second)))
	tr.done(fmt.Sprintf("%d requests, %d found", len(first), len(out)))
	return out, nil
}
