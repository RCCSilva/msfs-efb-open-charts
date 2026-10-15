package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// The footprint of a chart is the four corners of its map area on the ground. The planner draws
// them to show which charts exist around a place; the picture itself is fetched only when asked.

type corner struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type footprint struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	ICAO    string   `json:"icao"`
	Type    string   `json:"type"`
	Title   string   `json:"title"`
	Corners []corner `json:"corners"`

	minLat, minLon, maxLat, maxLon float64
	mtime                          time.Time
}

var (
	footprintMu    sync.Mutex
	footprintCache = map[string]*footprint{}
)

func (s *Server) footprintOf(c *store.Chart) *footprint {
	path := filepath.Join(s.lib.ChartDir(c.ID), "chart.json")
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	footprintMu.Lock()
	cached := footprintCache[c.ID]
	footprintMu.Unlock()
	if cached != nil && cached.mtime.Equal(st.ModTime()) {
		return cached
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var meta struct {
		GeoCorners []corner `json:"geoCorners"`
	}
	if json.Unmarshal(raw, &meta) != nil || len(meta.GeoCorners) < 3 {
		return nil
	}
	f := &footprint{ID: c.ID, Name: c.Name, ICAO: c.ICAO, Type: c.Type, Title: c.Title, Corners: meta.GeoCorners, mtime: st.ModTime()}
	f.minLat, f.maxLat, f.minLon, f.maxLon = 90, -90, 180, -180
	for _, p := range f.Corners {
		f.minLat, f.maxLat = min(f.minLat, p.Lat), max(f.maxLat, p.Lat)
		f.minLon, f.maxLon = min(f.minLon, p.Lon), max(f.maxLon, p.Lon)
	}
	footprintMu.Lock()
	footprintCache[c.ID] = f
	footprintMu.Unlock()
	return f
}

// GET /api/charts/footprints?minLat=&minLon=&maxLat=&maxLon=&limit=300
// The georeferenced charts whose area touches the rectangle.
func (s *Server) handleFootprints(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	num := func(k string, def float64) float64 {
		if v, err := strconv.ParseFloat(q.Get(k), 64); err == nil {
			return v
		}
		return def
	}
	minLat, minLon, maxLat, maxLon := num("minLat", -90), num("minLon", -180), num("maxLat", 90), num("maxLon", 180)
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 300
	}
	out := []*footprint{}
	cut := false
	for _, c := range s.lib.Search(store.Query{Type: q.Get("type"), ICAO: q.Get("icao")}) {
		if c.Unreferenced {
			continue
		}
		f := s.footprintOf(c)
		if f == nil || f.maxLat < minLat || f.minLat > maxLat || f.maxLon < minLon || f.minLon > maxLon {
			continue
		}
		if len(out) >= limit {
			cut = true
			break
		}
		out = append(out, f)
	}
	writeJSON(w, 200, map[string]any{"charts": out, "cut": cut})
}
