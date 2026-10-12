package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func putSettings(h http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("PUT", "/api/settings", strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:50000"
	req.Host = "127.0.0.1:43210"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAircraftSymbolDefaultsToAYellowPlaneAndCanBeChanged(t *testing.T) {
	_, h := newAccessServer(t, false)
	state := func() map[string]any {
		r := do(h, reqOpt{from: "127.0.0.1", host: "127.0.0.1:43210", path: "/api/viewer-state"})
		var m map[string]any
		if err := json.Unmarshal(r.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	st := state()
	if st["aircraft"] != "plane" || st["aircraftColor"] != "yellow" || st["aircraftHex"] == "" {
		t.Fatalf("default symbol = %v %v %v", st["aircraft"], st["aircraftColor"], st["aircraftHex"])
	}
	if r := putSettings(h, `{"aircraft":"helicopter","aircraftColor":"blue"}`); r.Code != 200 {
		t.Fatalf("PUT: %d %s", r.Code, r.Body.String())
	}
	st = state()
	if st["aircraft"] != "helicopter" || st["aircraftColor"] != "blue" {
		t.Fatalf("after PUT = %v %v", st["aircraft"], st["aircraftColor"])
	}
	for _, bad := range []string{`{"aircraft":"rocket"}`, `{"aircraftColor":"plaid"}`} {
		if r := putSettings(h, bad); r.Code != 400 {
			t.Errorf("%s: %d, want 400", bad, r.Code)
		}
	}
	if st = state(); st["aircraft"] != "helicopter" || st["aircraftColor"] != "blue" {
		t.Fatalf("a refused change altered the symbol: %v %v", st["aircraft"], st["aircraftColor"])
	}
}
