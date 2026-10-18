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
	"fmt"
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

// IdentBatch is how many identifiers are asked to the simulator at once. The simulator takes about
// 5 ms for each request and each identifier is three requests, so the batch only changes the fixed
// cost of a call. In a controlled test a bigger batch looked faster (19.4 ms per identifier at 40,
// 16.9 at 100, 15.3 at 300), but in a real load there was no measurable difference (18.6 ms at 40,
// 18.4 at 100). So it stays at 40: a call takes about 0.6 s, which is how long the route page and
// Stop may wait for it (a batch of 100 takes 1.6 s for the same speed).
const IdentBatch = 40

// ImportBatch is how many fixes of the files are stored in one transaction.
const ImportBatch = 2000

// ErrRunning is returned when a load is already running.
var ErrRunning = errors.New("a load is already running")

// Store is the navigation database as the load uses it.
type Store interface {
	SaveSeed(list []navdb.SeedIdent) error
	SaveFileFixes(fixes []navdata.Fix) (int, error)
	MarkIdentsCovered(idents []string) error
	FillRoutePositions() (int, error)
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
	// Timings and Logf, when set, measure the load: where its time goes is logged as it runs and
	// when it ends.
	Timings *Timings
	Logf    func(format string, args ...any)
}

// View is the state of the load, as the page shows it.
type View struct {
	Running bool      `json:"running"`
	Phase   string    `json:"phase"` // idle, interrupted, reading, importing, asking, neighbours, listing, airports, done, stopped, error
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
	job       *Job
	d         Deps
	began     time.Time
	stepStart time.Time
	stepDone  int // what was already done when the step began (a resumed load)
	lastLog   time.Time
}

// New builds a Loader that reports on job.
func New(job *Job, d Deps) *Loader { return &Loader{job: job, d: d} }

var icaoRe = regexp.MustCompile(`^[A-Z0-9]{2,8}$`)

// Run does the load and returns when it ends. The job must have been started (Job.Begin). prefix
// limits the airports to the codes that start with it ("SB"); empty: every airport.
func (l *Loader) Run(mode Mode, prefix string) {
	l.began = time.Now()
	l.logf("started (mode %s, prefix %q)", mode, prefix)
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

func (l *Loader) logf(format string, args ...any) {
	if l.d.Logf != nil {
		l.d.Logf("load: "+format, args...)
	}
}

// timed runs f as one run of the step name that handled n items.
func (l *Loader) timed(name string, n int, f func()) { l.d.Timings.Time(name, n, f) }

// progress logs how far the load is, at most every 30 seconds.
func (l *Loader) progress(what string, done int) {
	if l.d.Logf == nil || time.Since(l.lastLog) < 30*time.Second {
		return
	}
	l.lastLog = time.Now()
	l.logf("%s: %d done, %.1f per second since the step began", what, done, float64(done-l.stepDone)/time.Since(l.stepStart).Seconds())
}

func (l *Loader) finish(phase, msg string) {
	l.logf("%s (%s) after %s\n%s", phase, msg, time.Since(l.began).Round(time.Second), l.d.Timings.Report())
	l.job.set(func(v *View) { v.Running, v.Phase, v.Message, v.Ended = false, phase, msg, time.Now() })
	l.job.Save(l.d.Store)
}

func (l *Loader) seedFiles() []string {
	if l.d.Seed == nil {
		return nil
	}
	return l.d.Seed.Locate()
}

// importFixes brings the waypoints that the simulator's files hold into the database, with their
// airways, without asking the simulator anything: only what the database does not have is added. The
// names the files give as waypoints only are then known as a whole, so the simulator is not asked
// about them. It returns false when the load ended (finish was called).
func (l *Loader) importFixes(paths []string, seed []navdb.SeedIdent) bool {
	l.job.set(func(v *View) { v.Phase, v.Total, v.Done = "importing", 0, 0 })
	var fixes []navdata.Fix
	var err error
	l.timed("files.waypoints", 0, func() {
		fixes, err = l.d.Seed.Waypoints(paths, func(done, total int) bool {
			l.job.set(func(v *View) { v.Total, v.Done = total, done })
			return !l.job.stopped()
		})
	})
	if err != nil {
		if l.job.stopped() {
			l.finish("stopped", "")
		} else {
			l.finish("error", err.Error())
		}
		return false
	}
	l.job.set(func(v *View) { v.Total, v.Done = len(fixes), 0 })
	added := 0
	for start := 0; start < len(fixes); start += ImportBatch {
		if l.job.stopped() {
			l.finish("stopped", "")
			return false
		}
		part := fixes[start:min(len(fixes), start+ImportBatch)]
		var n int
		l.timed("db.saveFileFixes", len(part), func() { n, err = l.d.Store.SaveFileFixes(part) })
		if err != nil {
			l.finish("error", err.Error())
			return false
		}
		added += n
		l.job.set(func(v *View) { v.Done = start + len(part) })
	}
	var covered []string
	for _, s := range seed {
		if k := navdb.KindsToAsk(s.Ident, s.Classes); len(k) == 1 && k[0] == navdata.FixWaypoint {
			covered = append(covered, s.Ident)
		}
	}
	for start := 0; start < len(covered); start += ImportBatch {
		part := covered[start:min(len(covered), start+ImportBatch)]
		l.timed("db.markCovered", len(part), func() { err = l.d.Store.MarkIdentsCovered(part) })
		if err != nil {
			l.finish("error", err.Error())
			return false
		}
	}
	l.fillPositions()
	l.logf("the files gave %d waypoints, %d of them new; %d names need no question to the simulator", len(fixes), added, len(covered))
	return true
}

// fillPositions gives the neighbours of the airways that the files named but could not place the
// position of the fix that is stored for them by now.
func (l *Loader) fillPositions() {
	var n int
	var err error
	l.timed("db.fillPositions", 0, func() { n, err = l.d.Store.FillRoutePositions() })
	if err != nil {
		l.logf("filling the positions of the airways failed: %v", err)
		return
	}
	l.logf("%d positions of airway neighbours filled in", n)
}

// loadFixes is steps 1 and 2: the identifiers from the simulator's files, then each of them asked
// to the simulator as a whole. It returns false when the load ended (finish was called).
func (l *Loader) loadFixes(mode Mode) bool {
	store := l.d.Store

	// 1. the identifiers, from the simulator's files
	paths := l.seedFiles()
	if len(paths) > 0 {
		var list []navdb.SeedIdent
		var err error
		l.timed("files.read", 0, func() {
			list, err = l.d.Seed.Read(paths, func(done, total int) bool {
				l.job.set(func(v *View) { v.Total, v.Done = total, done })
				return !l.job.stopped()
			})
		})
		if err != nil {
			if l.job.stopped() {
				l.finish("stopped", "")
				return false
			}
			l.finish("error", err.Error())
			return false
		}
		var saveErr error
		l.timed("files.save", len(list), func() { saveErr = store.SaveSeed(list) })
		if err := saveErr; err != nil {
			l.finish("error", err.Error())
			return false
		}
		if !l.importFixes(paths, list) {
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
	const batch = IdentBatch
	answered, empty := false, 0
	neighbours := false
	l.stepStart, l.stepDone = time.Now(), c.SeedIdents-c.SeedPending
	for {
		if l.job.stopped() {
			l.finish("stopped", "")
			return false
		}
		var idents []string
		var err error
		l.timed("db.seedPending", 0, func() { idents, err = store.SeedPending(batch) })
		if err != nil {
			l.finish("error", err.Error())
			return false
		}
		if len(idents) == 0 {
			// the names of the files are done: the fixes that airways cite and the files did not
			// have are asked too, until no airway leads to a place the database does not have
			l.timed("db.missingNeighbors", 0, func() { idents, err = store.MissingNeighbors(batch) })
			if err != nil {
				l.finish("error", err.Error())
				return false
			}
			if len(idents) == 0 {
				return true
			}
			if !neighbours {
				// a second step, whose size nobody knows: every answer may name places that an airway
				// cites and the database lacks. The page shows how many were asked, not a percentage.
				neighbours = true
				l.stepStart, l.stepDone = time.Now(), 0
				l.logf("the names of the files are asked; now the places the airways cite")
				l.job.set(func(v *View) { v.Phase, v.Total, v.Done = "neighbours", 0, 0 })
			}
		}
		// the route page and the load take turns: a batch holds the lock, then lets it go
		var n int
		l.d.Turn.Lock()
		l.timed("ask.batch (sim+db)", len(idents), func() { n, err = l.d.Idents.LoadIdentsCount(idents, answered) })
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
				l.finish("error", fmt.Sprintf("the simulator answered nothing for %d identifiers in a row: load a flight and try again", 3*batch))
				return false
			}
		}
		l.job.set(func(v *View) { v.Done += len(idents) })
		l.timed("db.saveJob", 0, func() { l.job.Save(l.d.Store) })
		l.progress("asking", l.job.Snapshot().Done)
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
	var all []sim.AirportRef
	var err error
	l.timed("airports.list (sim)", 0, func() { all, err = l.d.Airports.ListAirports() })
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
	l.stepStart, l.stepDone = time.Now(), 0
	l.logf("%d airports to ask", len(todo))
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
		var answers []sim.AirportResult
		var err error
		l.timed("airports.ask (sim)", len(codes), func() { answers, err = l.d.Airports.Airports(codes) })
		if err == nil && len(answers) != len(codes) {
			err = errors.New("the simulator answered a different number of airports than asked")
		}
		if err == nil {
			for _, ans := range answers {
				if ans.Err == nil {
					l.timed("airports.save (db)", 1, func() { ans.Err = l.d.Store.SaveAirport(ans.Airport) })
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
		l.progress("airports", from+len(codes))
	}
	return true
}
