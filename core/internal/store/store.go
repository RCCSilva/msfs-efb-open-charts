// Package store is the chart library on disk plus an in-memory search index, and the
// flights (named sets of pinned charts).
//
// Layout under the data directory:
//
//	library/<id>/chart.png   rasterised chart
//	library/<id>/chart.json  georeference + metadata (what the viewers read)
//	library/<id>/source.pdf  the original PDF (kept so it can be re-imported)
//	flights.json             flights and the current flight
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/naming"
)

// Chart is the library entry shown by the API (a subset of chart.json).
type Chart struct {
	ID       string `json:"id"`
	Name     string `json:"name"`   // display name (editable)
	Source   string `json:"source"` // "decea-br", "manual"
	ICAO     string `json:"icao"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Validity string `json:"validity"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	// MetersPerPixel and ErrorMeters tell how good the georeference is.
	MetersPerPixel float64 `json:"metersPerPixel"`
	ErrorMeters    float64 `json:"errorMeters"`
	AddedAt        string  `json:"addedAt"`
	OriginalFile   string  `json:"originalFile,omitempty"`

	search string // folded haystack
}

// Flight is a named set of pinned charts.
type Flight struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Pins    []string `json:"pins"`
	Created string   `json:"created"`
}

type flightsFile struct {
	Current string   `json:"current"`
	Flights []Flight `json:"flights"`
}

// Library is safe for concurrent use.
type Library struct {
	dir string
	mu  sync.RWMutex

	charts  map[string]*Chart
	flights flightsFile
}

// Open loads (or creates) a library in dir.
func Open(dir string) (*Library, error) {
	l := &Library{dir: dir, charts: map[string]*Chart{}}
	if err := os.MkdirAll(filepath.Join(dir, "library"), 0o755); err != nil {
		return nil, err
	}
	if err := l.reload(); err != nil {
		return nil, err
	}
	if err := l.loadFlights(); err != nil {
		return nil, err
	}
	return l, nil
}

// Dir returns the data directory.
func (l *Library) Dir() string { return l.dir }

// ChartDir is the folder of one chart.
func (l *Library) ChartDir(id string) string { return filepath.Join(l.dir, "library", id) }

var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,120}$`)

// ValidID reports whether id is a safe folder name.
func ValidID(id string) bool { return idRe.MatchString(id) }

// Slug makes a safe id fragment.
func Slug(s string) string {
	s = strings.Join(naming.Tokens(s), "-")
	if s == "" {
		s = "chart"
	}
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}

func (l *Library) reload() error {
	entries, err := os.ReadDir(filepath.Join(l.dir, "library"))
	if err != nil {
		return err
	}
	l.charts = map[string]*Chart{}
	for _, e := range entries {
		if !e.IsDir() || !ValidID(e.Name()) {
			continue
		}
		c, err := readChart(l.ChartDir(e.Name()), e.Name())
		if err != nil {
			continue // folder without a valid chart.json
		}
		l.charts[c.ID] = c
	}
	return nil
}

func readChart(dir, id string) (*Chart, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "chart.json"))
	if err != nil {
		return nil, err
	}
	var m struct {
		Name         string `json:"name"`
		Source       string `json:"source"`
		ICAO         string `json:"icao"`
		Type         string `json:"type"`
		Title        string `json:"title"`
		Validity     string `json:"validity"`
		AddedAt      string `json:"addedAt"`
		OriginalFile string `json:"originalFile"`
		Image        struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"image"`
		Georef struct {
			ErrorMeters    float64 `json:"errorMeters"`
			MetersPerPixel float64 `json:"metersPerPixel"`
		} `json:"georef"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	c := &Chart{ID: id, Name: m.Name, Source: m.Source, ICAO: m.ICAO, Type: m.Type, Title: m.Title,
		Validity: m.Validity, AddedAt: m.AddedAt, OriginalFile: m.OriginalFile, Width: m.Image.Width, Height: m.Image.Height,
		MetersPerPixel: m.Georef.MetersPerPixel, ErrorMeters: m.Georef.ErrorMeters}
	if c.Name == "" {
		c.Name = naming.Display(c.ICAO, c.Type, c.Title)
	}
	if c.Name == "" {
		c.Name = id
	}
	c.index()
	return c, nil
}

func (c *Chart) index() {
	c.search = naming.Fold(strings.Join([]string{
		c.ID, c.Name, c.ICAO, c.Type, c.Title, c.Source, c.Validity, naming.TypeWords(c.Type),
	}, " "))
}

// Add stores a chart. chartJSON must already be the final chart.json (with the metadata fields).
func (l *Library) Add(id string, png, chartJSON, pdf []byte) (*Chart, error) {
	if !ValidID(id) {
		return nil, fmt.Errorf("invalid chart id %q", id)
	}
	dir := l.ChartDir(id)
	tmp := dir + ".tmp"
	_ = os.RemoveAll(tmp)
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return nil, err
	}
	for name, data := range map[string][]byte{"chart.png": png, "chart.json": chartJSON, "source.pdf": pdf} {
		if len(data) == 0 {
			continue
		}
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0o644); err != nil {
			return nil, err
		}
	}
	_ = os.RemoveAll(dir)
	if err := os.Rename(tmp, dir); err != nil {
		return nil, err
	}
	c, err := readChart(dir, id)
	if err != nil {
		return nil, err
	}
	l.mu.Lock()
	l.charts[id] = c
	l.mu.Unlock()
	return c, nil
}

// UniqueID returns base, or base-2, base-3... if taken.
func (l *Library) UniqueID(base string) string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	id := base
	for n := 2; l.charts[id] != nil; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	return id
}

// Get returns one chart.
func (l *Library) Get(id string) (*Chart, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	c, ok := l.charts[id]
	return c, ok
}

// Delete removes a chart from disk, from the index and from every flight's pins.
func (l *Library) Delete(id string) error {
	if !ValidID(id) {
		return fmt.Errorf("invalid chart id")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.charts[id]; !ok {
		return os.ErrNotExist
	}
	if err := os.RemoveAll(l.ChartDir(id)); err != nil {
		return err
	}
	delete(l.charts, id)
	for i := range l.flights.Flights {
		f := &l.flights.Flights[i]
		f.Pins = removeStr(f.Pins, id)
	}
	return l.saveFlightsLocked()
}

// Rename changes the display name (stored in chart.json).
func (l *Library) Rename(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("empty name")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.charts[id]
	if !ok {
		return os.ErrNotExist
	}
	path := filepath.Join(l.ChartDir(id), "chart.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	m["name"] = name
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return err
	}
	c.Name = name
	c.index()
	return nil
}

// Query describes a search.
type Query struct {
	Text   string
	Type   string
	ICAO   string
	Source string
	IDs    []string // restrict to these ids (e.g. a flight's pins)
}

// Search returns charts ordered by relevance (then by ICAO, type, name). Every word of Text must
// match the start of some word of the chart.
func (l *Library) Search(q Query) []*Chart {
	words := naming.Tokens(q.Text)
	allow := map[string]bool{}
	for _, id := range q.IDs {
		allow[id] = true
	}
	l.mu.RLock()
	defer l.mu.RUnlock()

	type scored struct {
		c *Chart
		s int
	}
	var out []scored
	for _, c := range l.charts {
		if q.IDs != nil && !allow[c.ID] {
			continue
		}
		if q.Type != "" && !strings.EqualFold(c.Type, q.Type) {
			continue
		}
		if q.ICAO != "" && !strings.EqualFold(c.ICAO, q.ICAO) {
			continue
		}
		if q.Source != "" && c.Source != q.Source {
			continue
		}
		score := 0
		ok := true
		hay := strings.Fields(c.search)
		for _, w := range words {
			best := 0
			for _, h := range hay {
				switch {
				case h == w:
					best = max(best, 3)
				case strings.HasPrefix(h, w):
					best = max(best, 2)
				case len(w) >= 3 && strings.Contains(h, w):
					best = max(best, 1)
				}
			}
			if best == 0 {
				ok = false
				break
			}
			score += best
			if strings.EqualFold(c.ICAO, w) {
				score += 4 // typing an ICAO should put that airport first
			}
		}
		if ok {
			out = append(out, scored{c, score})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.s != b.s {
			return a.s > b.s
		}
		if a.c.ICAO != b.c.ICAO {
			return a.c.ICAO < b.c.ICAO
		}
		if a.c.Type != b.c.Type {
			return a.c.Type < b.c.Type
		}
		return a.c.Name < b.c.Name
	})
	res := make([]*Chart, len(out))
	for i, s := range out {
		res[i] = s.c
	}
	return res
}

// Count of charts.
func (l *Library) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.charts)
}

// ---------------------------------------------------------------- flights

func (l *Library) flightsPath() string { return filepath.Join(l.dir, "flights.json") }

func (l *Library) loadFlights() error {
	raw, err := os.ReadFile(l.flightsPath())
	if err == nil {
		if err := json.Unmarshal(raw, &l.flights); err != nil {
			return fmt.Errorf("flights.json: %w", err)
		}
	}
	if len(l.flights.Flights) == 0 {
		l.flights.Flights = []Flight{{ID: "flight-1", Name: "Flight 1", Pins: []string{}, Created: now()}}
	}
	if l.currentIndex() < 0 {
		l.flights.Current = l.flights.Flights[0].ID
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func (l *Library) currentIndex() int {
	for i, f := range l.flights.Flights {
		if f.ID == l.flights.Current {
			return i
		}
	}
	return -1
}

func (l *Library) saveFlightsLocked() error {
	raw, err := json.MarshalIndent(l.flights, "", "  ")
	if err != nil {
		return err
	}
	tmp := l.flightsPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, l.flightsPath())
}

// FlightsState is what the API returns.
type FlightsState struct {
	Current string   `json:"current"`
	Flights []Flight `json:"flights"`
}

// Flights returns a copy of the current state.
func (l *Library) Flights() FlightsState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	st := FlightsState{Current: l.flights.Current, Flights: make([]Flight, len(l.flights.Flights))}
	for i, f := range l.flights.Flights {
		f.Pins = append([]string{}, f.Pins...)
		st.Flights[i] = f
	}
	return st
}

// CreateFlight adds a flight and makes it current.
func (l *Library) CreateFlight(name string) (Flight, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("Flight %d", len(l.flights.Flights)+1)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	id := fmt.Sprintf("flight-%d", time.Now().UnixNano()/1e6)
	f := Flight{ID: id, Name: name, Pins: []string{}, Created: now()}
	l.flights.Flights = append(l.flights.Flights, f)
	l.flights.Current = id
	return f, l.saveFlightsLocked()
}

func (l *Library) flightIndex(id string) int {
	for i, f := range l.flights.Flights {
		if f.ID == id {
			return i
		}
	}
	return -1
}

// SetCurrent selects the current flight.
func (l *Library) SetCurrent(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.flightIndex(id) < 0 {
		return os.ErrNotExist
	}
	l.flights.Current = id
	return l.saveFlightsLocked()
}

// RenameFlight changes a flight's name.
func (l *Library) RenameFlight(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("empty name")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	i := l.flightIndex(id)
	if i < 0 {
		return os.ErrNotExist
	}
	l.flights.Flights[i].Name = name
	return l.saveFlightsLocked()
}

// DeleteFlight removes a flight (the last one cannot be removed).
func (l *Library) DeleteFlight(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	i := l.flightIndex(id)
	if i < 0 {
		return os.ErrNotExist
	}
	if len(l.flights.Flights) == 1 {
		return errors.New("cannot delete the only flight")
	}
	l.flights.Flights = append(l.flights.Flights[:i], l.flights.Flights[i+1:]...)
	if l.flights.Current == id {
		l.flights.Current = l.flights.Flights[0].ID
	}
	return l.saveFlightsLocked()
}

// Pin adds a chart to a flight; Unpin removes it.
func (l *Library) Pin(flightID, chartID string) error { return l.setPin(flightID, chartID, true) }

// Unpin removes a chart from a flight.
func (l *Library) Unpin(flightID, chartID string) error { return l.setPin(flightID, chartID, false) }

func (l *Library) setPin(flightID, chartID string, pin bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	i := l.flightIndex(flightID)
	if i < 0 {
		return os.ErrNotExist
	}
	f := &l.flights.Flights[i]
	if pin {
		if _, ok := l.charts[chartID]; !ok {
			return os.ErrNotExist
		}
		for _, p := range f.Pins {
			if p == chartID {
				return nil
			}
		}
		f.Pins = append(f.Pins, chartID)
	} else {
		f.Pins = removeStr(f.Pins, chartID)
	}
	return l.saveFlightsLocked()
}

func removeStr(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// FindByOriginal returns the chart that was imported from a file with this name (case-insensitive).
func (l *Library) FindByOriginal(name string) (*Chart, bool) {
	name = strings.ToLower(filepath.Base(name))
	if name == "" || name == "." {
		return nil, false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	for _, c := range l.charts {
		if c.OriginalFile != "" && strings.ToLower(c.OriginalFile) == name {
			return c, true
		}
	}
	return nil, false
}
