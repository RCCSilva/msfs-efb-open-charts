// Package simconnect reads the user aircraft position from Microsoft Flight Simulator through
// the official SimConnect.dll, loaded at run time (no cgo, nothing linked at build time).
//
// Only what the viewer needs is bound: open, define five variables, request them every few
// frames, read dispatch messages, close. The DLL is part of the MSFS SDK; it is looked up in
// the places listed by FindDLL and never downloaded.
package simconnect

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State of the link with the simulator.
type State string

const (
	StateNoDLL   State = "no-dll"      // SimConnect.dll not found
	StateWaiting State = "waiting-sim" // DLL loaded, simulator not reachable yet
	StateOpen    State = "connected"   // connected, no position received yet
	StateData    State = "receiving"   // position flowing
)

// Sample is one reading of the user aircraft.
type Sample struct {
	Lat, Lon    float64
	HeadingTrue float64
	GroundSpeed float64 // knots
	AltitudeFt  float64
	At          time.Time
}

// Client keeps a connection to the simulator alive and remembers the latest sample.
type Client struct {
	mu      sync.Mutex
	state   State
	last    Sample
	detail  string
	dllPath string
}

// FindDLL looks for SimConnect.dll: explicit path, next to the executable, the data directory,
// then the usual MSFS SDK locations.
func FindDLL(explicit, dataDir string) (string, error) {
	var c []string
	if explicit != "" {
		c = append(c, explicit)
	}
	if exe, err := os.Executable(); err == nil {
		c = append(c, filepath.Join(filepath.Dir(exe), "SimConnect.dll"))
	}
	if dataDir != "" {
		c = append(c, filepath.Join(dataDir, "SimConnect.dll"))
	}
	for _, env := range []string{"MSFS2024_SDK", "MSFS_SDK"} {
		if v := os.Getenv(env); v != "" {
			c = append(c, filepath.Join(v, "SimConnect SDK", "lib", "SimConnect.dll"))
		}
	}
	c = append(c, `C:\MSFS 2024 SDK\SimConnect SDK\lib\SimConnect.dll`, `C:\MSFS SDK\SimConnect SDK\lib\SimConnect.dll`)
	for _, p := range c {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", errors.New("SimConnect.dll not found (install the MSFS SDK, or pass -simconnect-dll)")
}

// Status returns the link state and a human-readable detail.
func (c *Client) Status() (State, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state, c.detail
}

// Latest returns the most recent sample (ok is false until one arrives).
func (c *Client) Latest() (Sample, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last, !c.last.At.IsZero()
}

func (c *Client) set(st State, detail string) {
	c.mu.Lock()
	c.state, c.detail = st, detail
	c.mu.Unlock()
}

func (c *Client) put(s Sample) {
	c.mu.Lock()
	c.last, c.state, c.detail = s, StateData, ""
	c.mu.Unlock()
}

// New starts the background loop and returns immediately; use Status/Latest to observe it.
// logf may be nil.
func New(ctx context.Context, dllPath string, logf func(string, ...any)) *Client {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	c := &Client{state: StateWaiting, dllPath: dllPath}
	go c.run(ctx, logf)
	return c
}

var errUnsupported = fmt.Errorf("SimConnect is only available on Windows")
