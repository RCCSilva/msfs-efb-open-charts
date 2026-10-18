package navload

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Timings add up where the time of a load goes: for each named step, how many times it ran, how many
// items it handled and how long it took in all. It exists to measure (see the load's log) before
// anything is made faster.
type Timings struct {
	mu    sync.Mutex
	order []string
	m     map[string]*stat
}

type stat struct {
	calls, items int
	total, max   time.Duration
}

// NewTimings makes an empty Timings.
func NewTimings() *Timings { return &Timings{m: map[string]*stat{}} }

// Add records one run of the step name that handled n items in d. A nil Timings records nothing.
func (t *Timings) Add(name string, d time.Duration, n int) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	s := t.m[name]
	if s == nil {
		s = &stat{}
		t.m[name] = s
		t.order = append(t.order, name)
	}
	s.calls++
	s.items += n
	s.total += d
	s.max = max(s.max, d)
}

// Time runs f and records it as a run of the step name that handled n items.
func (t *Timings) Time(name string, n int, f func()) {
	start := time.Now()
	f()
	t.Add(name, time.Since(start), n)
}

// Report is one line per step, the one that took the longest first.
func (t *Timings) Report() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	names := append([]string(nil), t.order...)
	sort.SliceStable(names, func(i, j int) bool { return t.m[names[i]].total > t.m[names[j]].total })
	var b strings.Builder
	for _, n := range names {
		s := t.m[n]
		per := ""
		if s.items > 0 {
			per = fmt.Sprintf(", %.1f ms per item", float64(s.total.Microseconds())/1000/float64(s.items))
		}
		fmt.Fprintf(&b, "  %-22s %9s in %5d runs (longest %s), %d items%s\n", n, s.total.Round(time.Millisecond), s.calls, s.max.Round(time.Millisecond), s.items, per)
	}
	return b.String()
}
