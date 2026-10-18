package simconnect

import (
	"log"
	"os"
	"sync/atomic"
	"time"
)

// Trace turns on a line in the log for some of the calls to the simulator, with where the time of the
// call went (connecting, each round of requests). It is for measuring a load, so it is off unless the
// environment says OPENCHARTS_TRACE=1; and even on it logs only the first calls and then one in 50,
// as a full load makes thousands.
var Trace = os.Getenv("OPENCHARTS_TRACE") == "1"

var traced atomic.Int64

// sampled tells whether this call is one of the logged ones.
func sampled() bool {
	if !Trace {
		return false
	}
	n := traced.Add(1)
	return n <= 10 || n%50 == 0
}

// tracer times the parts of one call. A nil tracer (a call that is not logged) does nothing.
type tracer struct {
	what  string
	start time.Time
	last  time.Time
	parts []string
}

// newTracer starts timing a call, or returns nil when the call is not one to be logged.
func newTracer(what string) *tracer {
	if !sampled() {
		return nil
	}
	now := time.Now()
	return &tracer{what: what, start: now, last: now}
}

// mark closes the part that was running: its name and how long it took since the last mark.
func (t *tracer) mark(part string) {
	if t == nil {
		return
	}
	now := time.Now()
	t.parts = append(t.parts, part+" "+now.Sub(t.last).Round(time.Millisecond).String())
	t.last = now
}

// done writes the line.
func (t *tracer) done(detail string) {
	if t == nil {
		return
	}
	log.Printf("simconnect trace: %s %s: %v (total %s)", t.what, detail, t.parts, time.Since(t.start).Round(time.Millisecond))
}
