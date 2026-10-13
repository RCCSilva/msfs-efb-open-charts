// Package store is the chart library on disk plus an in-memory search index, and the
// groups (named sets of pinned charts).
//
// Layout under the data directory:
//
//	library/<id>/chart.png   rasterised chart
//	library/<id>/chart.json  georeference + metadata (what the viewers read)
//	library/<id>/source.pdf  the original PDF (kept so it can be re-imported)
//	groups.json             groups and the current group
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
	// Cycle is the AIRAC cycle ("2610") when Validity is the first day of one: the identifier that
	// pilots know. It is derived, never stored, so it cannot disagree with the date.
	Cycle  string `json:"cycle,omitempty"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	// MetersPerPixel and ErrorMeters tell how good the georeference is.
	MetersPerPixel float64 `json:"metersPerPixel"`
	ErrorMeters    float64 `json:"errorMeters"`
	AddedAt        string  `json:"addedAt"`
	OriginalFile   string  `json:"originalFile,omitempty"`
	// Unreferenced charts have no georeferencing: they can be looked at, not flown over.
	Unreferenced bool `json:"unreferenced,omitempty"`
	// HasSource tells that the original file (the PDF) is kept and can be shown.
	HasSource bool `json:"hasSource,omitempty"`

	search string // folded haystack
}

// Group is a named set of pinned charts.
type Group struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Pins    []string `json:"pins"`
	Created string   `json:"created"`
}

type groupsFile struct {
	Current string  `json:"current"`
	Groups  []Group `json:"groups"`
	// Legacy: before groups, the same list was called "flights" (in flights.json).
	Flights []Group `json:"flights,omitempty"`
}

// Library is safe for concurrent use.
type Library struct {
	dir string
	mu  sync.RWMutex

	charts map[string]*Chart
	groups groupsFile
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
	if err := l.loadGroups(); err != nil {
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
		Unreferenced bool   `json:"unreferenced"`
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
		MetersPerPixel: m.Georef.MetersPerPixel, ErrorMeters: m.Georef.ErrorMeters, Unreferenced: m.Unreferenced}
	if _, err := os.Stat(filepath.Join(dir, "source.pdf")); err == nil {
		c.HasSource = true
	}
	if c.Name == "" {
		c.Name = naming.Display(c.ICAO, c.Type, c.Title)
	}
	if c.Name == "" {
		c.Name = id
	}
	c.index()
	return c, nil
}

// deriveCycle fills Cycle from Validity.
func (c *Chart) deriveCycle() { c.Cycle = naming.CycleOfDate(c.Validity) }

func (c *Chart) index() {
	c.deriveCycle()
	c.search = naming.Fold(strings.Join([]string{
		c.ID, c.Name, c.ICAO, c.Type, c.Title, c.Source, c.Validity, c.Cycle, naming.TypeWords(c.Type),
	}, " "))
}

// Add stores a chart. chartJSON must already be the final chart.json (with the metadata fields).
func (l *Library) Add(id string, png, chartJSON, pdf []byte) (*Chart, error) {
	return l.AddFiles(id, png, chartJSON, pdf, nil)
}

// AddFiles is Add with more files of the chart by relative path (the tiles of a pyramid).
func (l *Library) AddFiles(id string, png, chartJSON, pdf []byte, extra map[string][]byte) (*Chart, error) {
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
	for rel, data := range extra {
		p := filepath.Join(tmp, filepath.FromSlash(rel))
		if !strings.HasPrefix(p, tmp+string(filepath.Separator)) {
			return nil, fmt.Errorf("extra file %q is outside the chart", rel)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
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

// Delete removes a chart from disk, from the index and from every group's pins.
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
	for i := range l.groups.Groups {
		f := &l.groups.Groups[i]
		f.Pins = removeStr(f.Pins, id)
	}
	return l.saveGroupsLocked()
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
	IDs    []string // restrict to these ids (e.g. a group's pins)
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

// ---------------------------------------------------------------- groups

func (l *Library) groupsPath() string { return filepath.Join(l.dir, "groups.json") }

func (l *Library) loadGroups() error {
	raw, err := os.ReadFile(l.groupsPath())
	if errors.Is(err, os.ErrNotExist) {
		raw, err = os.ReadFile(filepath.Join(l.dir, "flights.json")) // older libraries
	}
	if err == nil {
		if err := json.Unmarshal(raw, &l.groups); err != nil {
			return fmt.Errorf("groups.json: %w", err)
		}
		if len(l.groups.Groups) == 0 && len(l.groups.Flights) > 0 {
			l.groups.Groups = l.groups.Flights
		}
		l.groups.Flights = nil
	}
	if len(l.groups.Groups) == 0 {
		l.groups.Groups = []Group{{ID: "group-1", Name: "Group 1", Pins: []string{}, Created: now()}}
	}
	if l.currentIndex() < 0 {
		l.groups.Current = l.groups.Groups[0].ID
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

func (l *Library) currentIndex() int {
	for i, f := range l.groups.Groups {
		if f.ID == l.groups.Current {
			return i
		}
	}
	return -1
}

func (l *Library) saveGroupsLocked() error {
	raw, err := json.MarshalIndent(l.groups, "", "  ")
	if err != nil {
		return err
	}
	tmp := l.groupsPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, l.groupsPath())
}

// GroupsState is what the API returns.
type GroupsState struct {
	Current string  `json:"current"`
	Groups  []Group `json:"groups"`
}

// Groups returns a copy of the current state.
func (l *Library) Groups() GroupsState {
	l.mu.RLock()
	defer l.mu.RUnlock()
	st := GroupsState{Current: l.groups.Current, Groups: make([]Group, len(l.groups.Groups))}
	for i, f := range l.groups.Groups {
		f.Pins = append([]string{}, f.Pins...)
		st.Groups[i] = f
	}
	return st
}

// CreateGroup adds a group and makes it current.
func (l *Library) CreateGroup(name string) (Group, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = fmt.Sprintf("Group %d", len(l.groups.Groups)+1)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	id := fmt.Sprintf("group-%d", time.Now().UnixNano()/1e6)
	f := Group{ID: id, Name: name, Pins: []string{}, Created: now()}
	l.groups.Groups = append(l.groups.Groups, f)
	l.groups.Current = id
	return f, l.saveGroupsLocked()
}

func (l *Library) groupIndex(id string) int {
	for i, f := range l.groups.Groups {
		if f.ID == id {
			return i
		}
	}
	return -1
}

// SetCurrent selects the current group.
func (l *Library) SetCurrent(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.groupIndex(id) < 0 {
		return os.ErrNotExist
	}
	l.groups.Current = id
	return l.saveGroupsLocked()
}

// RenameGroup changes a group's name.
func (l *Library) RenameGroup(id, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("empty name")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	i := l.groupIndex(id)
	if i < 0 {
		return os.ErrNotExist
	}
	l.groups.Groups[i].Name = name
	return l.saveGroupsLocked()
}

// DeleteGroup removes a group (the last one cannot be removed).
func (l *Library) DeleteGroup(id string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	i := l.groupIndex(id)
	if i < 0 {
		return os.ErrNotExist
	}
	if len(l.groups.Groups) == 1 {
		return errors.New("cannot delete the only group")
	}
	l.groups.Groups = append(l.groups.Groups[:i], l.groups.Groups[i+1:]...)
	if l.groups.Current == id {
		l.groups.Current = l.groups.Groups[0].ID
	}
	return l.saveGroupsLocked()
}

// Pin adds a chart to a group; Unpin removes it.
func (l *Library) Pin(groupID, chartID string) error { return l.setPin(groupID, chartID, true) }

// Unpin removes a chart from a group.
func (l *Library) Unpin(groupID, chartID string) error { return l.setPin(groupID, chartID, false) }

func (l *Library) setPin(groupID, chartID string, pin bool) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	i := l.groupIndex(groupID)
	if i < 0 {
		return os.ErrNotExist
	}
	f := &l.groups.Groups[i]
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
	return l.saveGroupsLocked()
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
