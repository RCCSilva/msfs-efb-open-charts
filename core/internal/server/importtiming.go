package server

import (
	"sync"
	"time"
)

// importTiming keeps a running average of how long preparing one chart takes on this computer,
// so the app can tell how long a batch will take. It starts from a typical value and learns.
type importTiming struct {
	mu  sync.Mutex
	avg float64 // seconds per chart; 0 = not measured yet
}

// defaultSecondsPerChart is what a chart took on a typical PC at the normal detail level.
const defaultSecondsPerChart = 1.2

func (t *importTiming) record(d time.Duration) {
	sec := d.Seconds()
	if sec <= 0 || sec > 120 {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.avg == 0 {
		t.avg = sec
	} else {
		t.avg = 0.8*t.avg + 0.2*sec
	}
}

func (t *importTiming) secondsPerChart() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.avg == 0 {
		return defaultSecondsPerChart
	}
	return t.avg
}
