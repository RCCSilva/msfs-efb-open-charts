// Package navload loads the navigation data of the simulator into the database, all at once. It has
// three steps, always in this order:
//
//  1. the identifiers are read from the simulator's own package files, when they are on this computer
//     (no region, no airways: only the names of what exists);
//  2. every identifier is then asked to the simulator, as a whole: that is what brings the region, the
//     position and the airways. The simulator is always the source of data;
//  3. the airports: the simulator lists them, and each one that is not stored yet is asked in full.
//
// The load runs in the background and its state (Job) can be read at any time, stopped, and is kept
// with the database so that a restart of the program does not make it forget that one was running.
package navload

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
)

// Mode says which steps of the load are done.
type Mode string

const (
	Files    Mode = "files"    // step 1 only: read the identifiers from the files
	Fixes    Mode = "fixes"    // steps 1 and 2
	Airports Mode = "airports" // step 3 only
	All      Mode = "all"      // steps 1, 2 and 3
)

// ParseMode reads a mode; anything unknown is All.
func ParseMode(s string) Mode {
	switch Mode(s) {
	case Files, Fixes, Airports:
		return Mode(s)
	}
	return All
}

// ErrRunning is returned when a load is already running.
var ErrRunning = errors.New("a load is already running")

// Store is the navigation database as the load uses it.
type Store interface {
	SaveSeed(list []navdb.SeedIdent) error
	Counts() (navdb.Counts, error)
	SeedPending(n int) ([]string, error)
	MissingNeighbors(n int) ([]string, error)
	Airports(prefix string) ([]navdb.Summary, error)
	SaveAirport(a *navdata.Airport) error
	Meta(key string) string
	SetMeta(key, value string) error
}

// AirportSource is what the load asks the simulator about airports.
type AirportSource interface {
	ListAirports() ([]sim.AirportRef, error)
	Airports(icaos []string) ([]sim.AirportResult, error)
}

// IdentLoader asks the simulator for identifiers as a whole and stores what it knows.
type IdentLoader interface {
	LoadIdentsCount(idents []string, markEmpty bool) (int, error)
}

// Deps is what a Loader needs. Airports and Seed may be nil: the simulator cannot be asked, or has
// no files to read.
type Deps struct {
	Store    Store
	Airports AirportSource
	Seed     sim.SeedSource
	Idents   IdentLoader
	// Turn is taken around each question to the simulator, so that whatever else asks it (the route
	// page) and the load take turns.
	Turn sync.Locker
}

// View is the state of the load, as the page shows it.
type View struct {
	Running bool      `json:"running"`
	Phase   string    `json:"phase"` // idle, interrupted, reading, asking, listing, airports, done, stopped, error
	Message string    `json:"message"`
	Total   int       `json:"total"`
	Done    int       `json:"done"`
	Started time.Time `json:"startedAt"`
	Ended   time.Time `json:"endedAt"`
}

// Job is the state of a load; the server keeps one for as long as it runs.
type Job struct {
	mu   sync.Mutex
	v    View
	stop bool
}

const metaKey = "navload"

// Snapshot is the state now.
func (j *Job) Snapshot() View {
	j.mu.Lock()
	defer j.mu.Unlock()
	v := j.v
	if v.Phase == "" {
		v.Phase = "idle"
	}
	return v
}

// Running tells whether a load is running.
func (j *Job) Running() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.v.Running
}

// Begin marks a load as started; ErrRunning when one already is.
func (j *Job) Begin() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.v.Running {
		return ErrRunning
	}
	j.v = View{Running: true, Phase: "reading", Started: time.Now()}
	j.stop = false
	return nil
}

// Stop asks the running load to stop (what was loaded stays).
func (j *Job) Stop() {
	j.mu.Lock()
	j.stop = true
	j.mu.Unlock()
}

func (j *Job) stopped() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.stop
}

func (j *Job) set(f func(v *View)) {
	j.mu.Lock()
	f(&j.v)
	j.mu.Unlock()
}

// Meta is where the state is kept between runs of the program.
type Meta interface {
	Meta(key string) string
	SetMeta(key, value string) error
}

// Save keeps the state, so that a restart of the program does not make it forget that a load was running.
func (j *Job) Save(m Meta) {
	if b, err := json.Marshal(j.Snapshot()); err == nil {
		_ = m.SetMeta(metaKey, string(b))
	}
}

// Restore reads the state kept with the database: a load that was running when the program went
// away is shown as interrupted (it is not resumed by itself). A database with no load in its
// history (a new one) shows none.
func (j *Job) Restore(m Meta) {
	var v View
	if err := json.Unmarshal([]byte(m.Meta(metaKey)), &v); err != nil {
		j.set(func(cur *View) { *cur = View{} })
		j.mu.Lock()
		j.stop = false
		j.mu.Unlock()
		return
	}
	if v.Running {
		v.Running, v.Phase, v.Message = false, "interrupted", ""
	}
	j.set(func(cur *View) { *cur = v })
}

// Loader does a load.
type Loader struct {
	job *Job
	d   Deps
}

// New builds a Loader that reports on job.
func New(job *Job, d Deps) *Loader { return &Loader{job: job, d: d} }

var icaoRe = regexp.MustCompile(`^[A-Z0-9]{2,8}$`)

// Run does the load and returns when it ends. The job must have been started (Job.Begin). prefix
// limits the airports to the codes that start with it ("SB"); empty: every airport.
func (l *Loader) Run(mode Mode, prefix string) {
	if mode != Airports {
		if !l.loadFixes(mode) {
			return
		}
	}
	if mode == All || mode == Airports {
		if !l.loadAirports(strings.ToUpper(strings.TrimSpace(prefix))) {
			return
		}
	}
	l.finish("done", "")
}

func (l *Loader) finish(phase, msg string) {
	l.job.set(func(v *View) { v.Running, v.Phase, v.Message, v.Ended = false, phase, msg, time.Now() })
	l.job.Save(l.d.Store)
}

func (l *Loader) seedFiles() []string {
	if l.d.Seed == nil {
		return nil
	}
	return l.d.Seed.Locate()
}

// loadFixes is steps 1 and 2: the identifiers from the simulator's files, then each of them asked
// to the simulator as a whole. It returns false when the load ended (finish was called).
func (l *Loader) loadFixes(mode Mode) bool {
	store := l.d.Store

	// 1. the identifiers, from the simulator's files
	paths := l.seedFiles()
	if len(paths) > 0 {
		list, err := l.d.Seed.Read(paths, func(done, total int) bool {
			l.job.set(func(v *View) { v.Total, v.Done = total, done })
			return !l.job.stopped()
		})
		if err != nil {
			if l.job.stopped() {
				l.finish("stopped", "")
				return false
			}
			l.finish("error", err.Error())
			return false
		}
		if err := store.SaveSeed(list); err != nil {
			l.finish("error", err.Error())
			return false
		}
	}
	if mode == Files {
		if len(paths) == 0 {
			l.finish("error", "no navigation package of the simulator was found on this computer")
			return false
		}
		l.finish("done", "")
		return false
	}

	// 2. each identifier is asked to the simulator, as a whole
	c, err := store.Counts()
	if err != nil {
		l.finish("error", err.Error())
		return false
	}
	if c.SeedIdents == 0 {
		l.finish("error", "no identifiers to ask for: the simulator's files were not found")
		return false
	}
	l.job.set(func(v *View) { v.Phase, v.Total, v.Done = "asking", c.SeedIdents, c.SeedIdents-c.SeedPending })
	const batch = 40
	answered, empty := false, 0
	for {
		if l.job.stopped() {
			l.finish("stopped", "")
			return false
		}
		idents, err := store.SeedPending(batch)
		if err != nil {
			l.finish("error", err.Error())
			return false
		}
		if len(idents) == 0 {
			// the names of the files are done: the fixes that airways cite and the files did not
			// have are asked too, until no airway leads to a place the database does not have
			idents, err = store.MissingNeighbors(batch)
			if err != nil {
				l.finish("error", err.Error())
				return false
			}
			if len(idents) == 0 {
				return true
			}
			l.job.set(func(v *View) { v.Total += len(idents) })
		}
		// the route page and the load take turns: a batch holds the lock, then lets it go
		l.d.Turn.Lock()
		n, err := l.d.Idents.LoadIdentsCount(idents, answered)
		l.d.Turn.Unlock()
		if err != nil {
			l.finish("error", err.Error())
			return false
		}
		if n > 0 {
			answered = true
		} else if !answered {
			// nothing came back and nothing ever did in this load: the simulator is probably at its
			// menu, not in a flight. Nothing was marked as "does not exist"; stop instead of doing so.
			empty++
			if empty >= 3 {
				l.finish("error", "the simulator answered nothing for 120 identifiers in a row: load a flight and try again")
				return false
			}
		}
		l.job.set(func(v *View) { v.Done += len(idents) })
		l.job.Save(l.d.Store)
	}
}

// loadAirports is step 3: the simulator lists its airports, and each one that is not stored yet
// (and starts with the prefix, when there is one) is asked in full, with its procedures.
func (l *Loader) loadAirports(prefix string) bool {
	if l.d.Airports == nil {
		l.finish("error", "there is no link to the simulator to list its airports")
		return false
	}
	l.job.set(func(v *View) { v.Phase, v.Message, v.Total, v.Done = "listing", "", 0, 0 })
	l.job.Save(l.d.Store)
	all, err := l.d.Airports.ListAirports()
	if err != nil {
		l.finish("error", err.Error())
		return false
	}
	have := map[string]bool{}
	if stored, err := l.d.Store.Airports(""); err == nil {
		for _, a := range stored {
			have[a.ICAO] = true
		}
	}
	var todo []string
	for _, a := range all {
		icao := strings.ToUpper(a.ICAO)
		if !icaoRe.MatchString(icao) || have[icao] || (prefix != "" && !strings.HasPrefix(icao, prefix)) {
			continue
		}
		have[icao] = true // a repeat in the list is asked once
		todo = append(todo, icao)
	}
	sort.Strings(todo)
	l.job.set(func(v *View) { v.Phase, v.Total, v.Done = "airports", len(todo), 0 })
	l.job.Save(l.d.Store)
	// the simulator answers each request after a fixed delay, so many are asked at once
	const chunk = 200
	failures := 0
	for from := 0; from < len(todo); from += chunk {
		if l.job.stopped() {
			l.finish("stopped", "")
			return false
		}
		codes := todo[from:min(len(todo), from+chunk)]
		l.d.Turn.Lock()
		answers, err := l.d.Airports.Airports(codes)
		if err == nil && len(answers) != len(codes) {
			err = errors.New("the simulator answered a different number of airports than asked")
		}
		if err == nil {
			for _, ans := range answers {
				if ans.Err == nil {
					ans.Err = l.d.Store.SaveAirport(ans.Airport)
				}
				switch {
				case ans.Err == nil:
					failures = 0
				case errors.Is(ans.Err, navdata.ErrUnknown):
					// the simulator listed it and then did not know it: skip
				default:
					failures++
					err = ans.Err
				}
			}
		} else {
			failures++
		}
		l.d.Turn.Unlock()
		if failures >= 5 {
			l.finish("error", err.Error())
			return false
		}
		l.job.set(func(v *View) { v.Done = from + len(codes) })
		l.job.Save(l.d.Store)
	}
	return true
}
