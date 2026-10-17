//go:build windows

package simconnect

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"syscall"
	"time"
	"unsafe"
)

const (
	defID = 1
	reqID = 1

	dataTypeFloat64 = 4
	periodSimFrame  = 3

	recvException  = 1
	recvOpen       = 2
	recvQuit       = 3
	recvSimObjData = 8

	recvHeader  = 12 // dwSize, dwVersion, dwID
	simObjData  = recvHeader + 7*4
	sampleBytes = 5 * 8
)

var vars = []struct{ name, unit string }{
	{"PLANE LATITUDE", "degrees"},
	{"PLANE LONGITUDE", "degrees"},
	{"PLANE HEADING DEGREES TRUE", "degrees"},
	{"GROUND VELOCITY", "knots"},
	{"PLANE ALTITUDE", "feet"},
}

type api struct {
	open, close, addDef, request, nextDispatch *syscall.LazyProc
}

func load(path string) (*api, error) {
	d := syscall.NewLazyDLL(path)
	if err := d.Load(); err != nil {
		return nil, err
	}
	return &api{
		open: d.NewProc("SimConnect_Open"), close: d.NewProc("SimConnect_Close"),
		addDef: d.NewProc("SimConnect_AddToDataDefinition"), request: d.NewProc("SimConnect_RequestDataOnSimObject"),
		nextDispatch: d.NewProc("SimConnect_GetNextDispatch"),
	}, nil
}

func hr(r uintptr) error {
	if uint32(r) == 0 {
		return nil
	}
	return fmt.Errorf("HRESULT 0x%08X", uint32(r))
}

func (a *api) connect(name string) (uintptr, error) {
	n, _ := syscall.BytePtrFromString(name)
	var h uintptr
	r, _, _ := a.open.Call(uintptr(unsafe.Pointer(&h)), uintptr(unsafe.Pointer(n)), 0, 0, 0, 0)
	if err := hr(r); err != nil {
		return 0, err
	}
	for i, v := range vars {
		nm, _ := syscall.BytePtrFromString(v.name)
		un, _ := syscall.BytePtrFromString(v.unit)
		// handle, DefineID, DatumName, UnitsName, DatumType, fEpsilon (0), DatumID
		r, _, _ = a.addDef.Call(h, defID, uintptr(unsafe.Pointer(nm)), uintptr(unsafe.Pointer(un)), dataTypeFloat64, 0, uintptr(i))
		if err := hr(r); err != nil {
			a.close.Call(h)
			return 0, fmt.Errorf("define %s: %w", v.name, err)
		}
	}
	// handle, RequestID, DefineID, ObjectID (0 = user aircraft), Period, Flags, Origin, Interval, Limit
	r, _, _ = a.request.Call(h, reqID, defID, 0, periodSimFrame, 0, 0, 6, 0)
	if err := hr(r); err != nil {
		a.close.Call(h)
		return 0, fmt.Errorf("request: %w", err)
	}
	return h, nil
}

// next returns one dispatch message copied into Go memory, or nil when there is none.
func (a *api) next(h uintptr) []byte {
	var p unsafe.Pointer
	var cb uint32
	r, _, _ := a.nextDispatch.Call(h, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&cb)))
	if uint32(r) != 0 || p == nil || cb < recvHeader {
		return nil
	}
	return append([]byte(nil), unsafe.Slice((*byte)(p), int(cb))...)
}

func (c *Client) run(ctx context.Context, logf func(string, ...any)) {
	a, err := load(c.dllPath)
	if err != nil {
		c.set(StateNoDLL, err.Error())
		logf("simconnect: cannot load %s: %v", c.dllPath, err)
		return
	}
	logf("simconnect: using %s", c.dllPath)
	for ctx.Err() == nil {
		h, err := a.connect("Open Charts")
		if err != nil {
			c.set(StateWaiting, "simulator not reachable")
			if !sleep(ctx, 3*time.Second) {
				return
			}
			continue
		}
		logf("simconnect: connected")
		c.set(StateOpen, "")
		c.pump(ctx, a, h, logf)
		a.close.Call(h)
		logf("simconnect: disconnected")
		c.set(StateWaiting, "simulator closed")
		if !sleep(ctx, 2*time.Second) {
			return
		}
	}
}

// pump reads messages until the simulator quits, the context ends or the link goes silent.
func (c *Client) pump(ctx context.Context, a *api, h uintptr, logf func(string, ...any)) {
	lastMsg := time.Now()
	for ctx.Err() == nil {
		msg := a.next(h)
		if msg == nil {
			if time.Since(lastMsg) > 30*time.Second {
				return // silent for too long (sim crashed?): reconnect
			}
			if !sleep(ctx, 40*time.Millisecond) {
				return
			}
			continue
		}
		lastMsg = time.Now()
		switch binary.LittleEndian.Uint32(msg[8:12]) {
		case recvQuit:
			return
		case recvException:
			if len(msg) >= recvHeader+8 {
				logf("simconnect: exception %d (send id %d)", binary.LittleEndian.Uint32(msg[12:16]), binary.LittleEndian.Uint32(msg[16:20]))
			}
		case recvOpen:
			logf("simconnect: simulator ready")
		case recvSimObjData:
			if len(msg) < simObjData+sampleBytes || binary.LittleEndian.Uint32(msg[12:16]) != reqID {
				continue
			}
			f := func(i int) float64 {
				return math.Float64frombits(binary.LittleEndian.Uint64(msg[simObjData+i*8:]))
			}
			c.put(Sample{Lat: f(0), Lon: f(1), HeadingTrue: f(2), GroundSpeed: f(3), AltitudeFt: f(4), At: time.Now()})
		}
	}
}

func sleep(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}
