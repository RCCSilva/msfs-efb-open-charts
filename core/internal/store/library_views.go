package store

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// AirportSummary is one aerodrome of the library: how many charts it has and of which types.
// Charts without an ICAO (visual charts, for example) are grouped under an empty ICAO.
type AirportSummary struct {
	ICAO  string         `json:"icao"`
	Count int            `json:"count"`
	Types map[string]int `json:"types"`
}

// Airports lists the aerodromes of the library, optionally only those whose ICAO starts with
// prefix, ordered by ICAO (charts without an ICAO last).
func (l *Library) Airports(prefix string) []AirportSummary {
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	l.mu.RLock()
	defer l.mu.RUnlock()
	by := map[string]*AirportSummary{}
	for _, c := range l.charts {
		icao := strings.ToUpper(c.ICAO)
		if prefix != "" && !strings.HasPrefix(icao, prefix) {
			continue
		}
		a := by[icao]
		if a == nil {
			a = &AirportSummary{ICAO: icao, Types: map[string]int{}}
			by[icao] = a
		}
		a.Count++
		typ := c.Type
		if typ == "" {
			typ = "?"
		}
		a.Types[typ]++
	}
	out := make([]AirportSummary, 0, len(by))
	for _, a := range by {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].ICAO == "") != (out[j].ICAO == "") {
			return out[j].ICAO == "" // charts without an ICAO last
		}
		return out[i].ICAO < out[j].ICAO
	})
	return out
}

// Types lists the chart types present in the library, sorted.
func (l *Library) Types() []string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	seen := map[string]bool{}
	for _, c := range l.charts {
		if c.Type != "" {
			seen[c.Type] = true
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// DeleteMany removes several charts in one go: from disk, from the index and from every group's
// pins, saving the groups once. It returns how many were removed; ids that do not exist are
// skipped (not an error: two windows may have the same list open).
func (l *Library) DeleteMany(ids []string) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	removed := map[string]bool{}
	for _, id := range ids {
		if !ValidID(id) {
			return len(removed), fmt.Errorf("invalid chart id %q", id)
		}
		if _, ok := l.charts[id]; !ok || removed[id] {
			continue
		}
		if err := os.RemoveAll(l.ChartDir(id)); err != nil {
			return len(removed), err
		}
		delete(l.charts, id)
		removed[id] = true
	}
	if len(removed) == 0 {
		return 0, nil
	}
	for i := range l.groups.Groups {
		g := &l.groups.Groups[i]
		keep := g.Pins[:0]
		for _, p := range g.Pins {
			if !removed[p] {
				keep = append(keep, p)
			}
		}
		g.Pins = keep
	}
	return len(removed), l.saveGroupsLocked()
}
