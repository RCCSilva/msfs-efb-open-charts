// Package jobs tracks long operations (importing many charts) so a UI can show progress.
// Jobs only run when the user asks for them; nothing is scheduled.
package jobs

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// Item is the outcome for one file.
type Item struct {
	File    string `json:"file"`
	ChartID string `json:"chartId,omitempty"`
	Name    string `json:"name,omitempty"`
	Status  string `json:"status"` // "ok", "skipped", "error"
	Message string `json:"message,omitempty"`
}

// Snapshot is the copyable, JSON-ready state of a job.
type Snapshot struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	State   string `json:"state"` // "running", "done"
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	OK      int    `json:"ok"`
	Skipped int    `json:"skipped"`
	Failed  int    `json:"failed"`
	Current string `json:"current,omitempty"`
	// Stage names what a single-file job is doing now ("waiting", "engine", "render", "save")
	// and Percent is the server's estimate of how far it is. The UI only displays them.
	Stage   string     `json:"stage,omitempty"`
	Percent int        `json:"percent"`
	Started time.Time  `json:"started"`
	Ended   *time.Time `json:"ended,omitempty"`
	Items   []Item     `json:"items"`
}

// Job is the live job a worker reports to.
type Job struct {
	mu sync.Mutex
	s  Snapshot
}

// ID of the job.
func (j *Job) ID() string { return j.s.ID }

// Manager keeps the most recent jobs in memory.
type Manager struct {
	mu   sync.Mutex
	jobs map[string]*Job
	max  int
	ord  []string
}

// NewManager creates a manager that remembers up to max jobs.
func NewManager(max int) *Manager { return &Manager{jobs: map[string]*Job{}, max: max} }

// Start registers a job and runs fn in a goroutine; fn reports through the Job.
func (m *Manager) Start(kind string, total int, fn func(j *Job)) *Job {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	j := &Job{s: Snapshot{ID: hex.EncodeToString(b), Kind: kind, State: "running", Total: total,
		Started: time.Now().UTC(), Items: []Item{}}}
	m.mu.Lock()
	m.jobs[j.s.ID] = j
	m.ord = append(m.ord, j.s.ID)
	for len(m.ord) > m.max {
		delete(m.jobs, m.ord[0])
		m.ord = m.ord[1:]
	}
	m.mu.Unlock()
	go func() {
		fn(j)
		j.mu.Lock()
		end := time.Now().UTC()
		j.s.State, j.s.Ended, j.s.Current, j.s.Stage, j.s.Percent = "done", &end, "", "", 100
		j.mu.Unlock()
	}()
	return j
}

// Get returns a copy of the job's state.
func (m *Manager) Get(id string) (Snapshot, bool) {
	m.mu.Lock()
	j, ok := m.jobs[id]
	m.mu.Unlock()
	if !ok {
		return Snapshot{}, false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	c := j.s
	c.Items = append([]Item{}, j.s.Items...)
	return c, true
}

// SetCurrent records what is being processed.
func (j *Job) SetCurrent(s string) {
	j.mu.Lock()
	j.s.Current = s
	j.mu.Unlock()
}

// SetStage records the stage of a single-file job and how far along it is (0-100).
func (j *Job) SetStage(stage string, percent int) {
	j.mu.Lock()
	j.s.Stage, j.s.Percent = stage, percent
	j.mu.Unlock()
}

// Add records the outcome of one file.
func (j *Job) Add(it Item) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.s.Items = append(j.s.Items, it)
	j.s.Done++
	switch it.Status {
	case "ok":
		j.s.OK++
	case "skipped":
		j.s.Skipped++
	default:
		j.s.Failed++
	}
}
