package server

import (
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PerfWindow is what a viewer counted about its own work during a few seconds: how many tiles it
// drew, how often the map moved, how many times the page was changed. It exists to find out what
// makes a screen flicker inside the simulator, which cannot be seen from here.
type PerfWindow struct {
	At      time.Time      `json:"at"`
	Kind    string         `json:"kind"` // "simulator" | "browser"
	Seconds int            `json:"seconds"`
	Mode    string         `json:"mode"`
	Counts  map[string]int `json:"counts"`
	Note    string         `json:"note,omitempty"`
}

var (
	perfMu      sync.Mutex
	perfWindows = map[string][]PerfWindow{}
)

const perfKeep = 12

func (s *Server) handlePostClientPerf(w http.ResponseWriter, r *http.Request) {
	var in struct {
		UA      string         `json:"ua"`
		Seconds int            `json:"seconds"`
		Mode    string         `json:"mode"`
		Counts  map[string]int `json:"counts"`
		Note    string         `json:"note"`
	}
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err)
		return
	}
	if len(in.Counts) > 40 {
		writeErr(w, 400, errTooMany)
		return
	}
	pw := PerfWindow{At: time.Now(), Kind: kindOf(in.UA), Seconds: in.Seconds, Mode: in.Mode, Counts: in.Counts, Note: truncate(in.Note, 300)}
	perfMu.Lock()
	list := append(perfWindows[pw.Kind], pw)
	if len(list) > perfKeep {
		list = list[len(list)-perfKeep:]
	}
	perfWindows[pw.Kind] = list
	perfMu.Unlock()

	keys := make([]string, 0, len(in.Counts))
	for k, v := range in.Counts {
		if v != 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(" " + k + "=" + strconv.Itoa(in.Counts[k]))
	}
	log.Printf("perf (%s, %ds, %s):%s %s", pw.Kind, in.Seconds, in.Mode, b.String(), pw.Note)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleGetClientPerf(w http.ResponseWriter, r *http.Request) {
	perfMu.Lock()
	defer perfMu.Unlock()
	out := map[string][]PerfWindow{}
	for k, v := range perfWindows {
		out[k] = append([]PerfWindow{}, v...)
	}
	writeJSON(w, 200, out)
}

var errTooMany = &perfError{"too many counters"}

type perfError struct{ s string }

func (e *perfError) Error() string { return e.s }

func truncate(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
