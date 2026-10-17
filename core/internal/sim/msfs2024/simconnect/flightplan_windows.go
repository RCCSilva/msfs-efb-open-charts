//go:build windows

package simconnect

import (
	"encoding/binary"
	"fmt"
	"time"
)

// LoadFlightPlan asks the running simulator to load a flight plan file (.pln) that is on this
// computer. SimConnect does not answer when it works; an exception arrives when it does not, so the
// connection is kept for a moment to see it.
func LoadFlightPlan(dllPath, file string, wait time.Duration) error {
	facilityMu.Lock()
	defer facilityMu.Unlock()

	d, err := loadFacilities(dllPath)
	if err != nil {
		return err
	}
	load := d.dll.NewProc("SimConnect_FlightPlanLoad")
	if err := load.Find(); err != nil {
		return fmt.Errorf("this SimConnect.dll cannot load a flight plan: %w", err)
	}
	h, err := d.connectOnly("Open Charts flight plan")
	if err != nil {
		return fmt.Errorf("the simulator is not reachable: %w", err)
	}
	defer d.close.Call(h)

	r, _, _ := load.Call(h, cstr(file))
	if e := hr(r); e != nil {
		return fmt.Errorf("FlightPlanLoad: %w", e)
	}
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		msg := d.next(h)
		if msg == nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if binary.LittleEndian.Uint32(msg[8:12]) == recvException {
			return fmt.Errorf("the simulator refused the flight plan (exception %d)", binary.LittleEndian.Uint32(msg[12:16]))
		}
	}
	return nil
}
