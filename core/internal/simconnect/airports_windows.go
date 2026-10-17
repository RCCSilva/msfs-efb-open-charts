//go:build windows

package simconnect

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
)

// AirportRef is one line of the simulator's list of airports.
type AirportRef struct {
	ICAO   string
	Region string
	Lat    float64
	Lon    float64
}

// airportEntry is the size of one entry of the list of airports (packed): the identifier (9 bytes),
// the region (3 bytes) and the position as three doubles (latitude, longitude, altitude).
const (
	airportEntry  = 36
	airportRegion = 9
	airportLat    = 12
	airportLon    = 20
)

// ListAirports asks the simulator for the list of every airport it has (identifier, region and
// position only). The detail of an airport is a separate question (FetchAirport).
//
// It uses the plain RequestFacilitiesList: the EX1 variant answers with the few airports around
// the aircraft only. The answer comes in packets (EntryNumber out of OutOf are the packet numbers,
// not the airports).
func ListAirports(dllPath string, wait time.Duration) ([]AirportRef, error) {
	facilityMu.Lock()
	defer facilityMu.Unlock()

	f, err := loadFacilities(dllPath)
	if err != nil {
		return nil, err
	}
	h, err := f.connectOnly("Open Charts airports")
	if err != nil {
		return nil, fmt.Errorf("the simulator is not reachable: %w", err)
	}
	defer f.close.Call(h)

	const listAirport = 0
	r, _, _ := f.reqList.Call(h, listAirport, 3000)
	if e := hr(r); e != nil {
		return nil, fmt.Errorf("RequestFacilitiesList: %w", e)
	}
	var out []AirportRef
	seen := map[string]bool{}
	packets, outOf := 0, 0
	deadline := time.Now().Add(wait)
	quiet := time.Now()
	for time.Now().Before(deadline) {
		msg := f.next(h)
		if msg == nil {
			if time.Since(quiet) > 8*time.Second {
				break
			}
			time.Sleep(5 * time.Millisecond)
			continue
		}
		quiet = time.Now()
		switch binary.LittleEndian.Uint32(msg[8:12]) {
		case recvException:
			return out, fmt.Errorf("the simulator refused the list of airports (exception %d)", binary.LittleEndian.Uint32(msg[12:16]))
		case recvAirportList:
			if len(msg) < listEntryOffset {
				continue
			}
			count := int(binary.LittleEndian.Uint32(msg[16:20]))
			outOf = int(binary.LittleEndian.Uint32(msg[24:28]))
			packets++
			for k := 0; k < count; k++ {
				o := listEntryOffset + k*airportEntry
				if o+airportEntry > len(msg) {
					break
				}
				e := msg[o : o+airportEntry]
				icao := strings.ToUpper(cString(e[:airportRegion]))
				if icao == "" || seen[icao] {
					continue
				}
				seen[icao] = true
				out = append(out, AirportRef{
					ICAO:   icao,
					Region: cString(e[airportRegion : airportRegion+3]),
					Lat:    math.Float64frombits(binary.LittleEndian.Uint64(e[airportLat:])),
					Lon:    math.Float64frombits(binary.LittleEndian.Uint64(e[airportLon:])),
				})
			}
		}
		if outOf > 0 && packets >= outOf {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("the simulator sent no airports")
	}
	return out, nil
}
