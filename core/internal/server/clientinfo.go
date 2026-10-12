package server

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ClientInfo is what a viewer reports about the browser it runs in. It exists to answer
// questions that cannot be answered from here (does the simulator's browser have WebGL?).
type ClientInfo struct {
	At       time.Time `json:"at"`
	Local    bool      `json:"local"` // from this computer (the simulator or a browser on it)
	Kind     string    `json:"kind"`  // "simulator" | "browser"
	UA       string    `json:"ua"`
	DPR      float64   `json:"dpr"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	WebGL    bool      `json:"webgl"`
	WebGL2   bool      `json:"webgl2"`
	Touch    bool      `json:"touch"`
	Renderer string    `json:"renderer,omitempty"`
}

var (
	clientMu   sync.Mutex
	clientInfo = map[string]ClientInfo{} // by kind: the latest of each
)

// kindOf guesses whether the client is the simulator's browser from its user agent.
func kindOf(ua string) string {
	l := strings.ToLower(ua)
	if strings.Contains(l, "coherent") || strings.Contains(l, "gameface") || strings.Contains(l, "msfs") ||
		strings.Contains(l, "flightsimulator") {
		return "simulator"
	}
	return "browser"
}

func (s *Server) handlePostClientInfo(w http.ResponseWriter, r *http.Request) {
	var in ClientInfo
	if err := readJSON(r, &in); err != nil {
		writeErr(w, 400, err)
		return
	}
	if len(in.UA) > 400 {
		in.UA = in.UA[:400]
	}
	if len(in.Renderer) > 200 {
		in.Renderer = in.Renderer[:200]
	}
	in.At = time.Now()
	in.Local = isLocalRequest(r)
	in.Kind = kindOf(in.UA)
	clientMu.Lock()
	clientInfo[in.Kind] = in
	clientMu.Unlock()
	log.Printf("client (%s, local=%v): webgl=%v webgl2=%v dpr=%.2f %dx%d touch=%v ua=%q",
		in.Kind, in.Local, in.WebGL, in.WebGL2, in.DPR, in.Width, in.Height, in.Touch, in.UA)
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *Server) handleGetClientInfo(w http.ResponseWriter, r *http.Request) {
	clientMu.Lock()
	defer clientMu.Unlock()
	out := make([]ClientInfo, 0, len(clientInfo))
	for _, k := range []string{"simulator", "browser"} {
		if c, ok := clientInfo[k]; ok {
			out = append(out, c)
		}
	}
	writeJSON(w, 200, out)
}
