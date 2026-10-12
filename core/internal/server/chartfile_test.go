package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// After a chart is edited, what the browser reads must be the edited data, not a cached copy.
func TestChartFilesAreRevalidatedAndShowEdits(t *testing.T) {
	lib, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	meta := `{"id":"c1","name":"SBSJ · ADC","source":"manual","icao":"SBSJ","type":"ADC","title":"",
	  "image":{"file":"chart.png","width":10,"height":10,"dpi":72},"georef":{"errorMeters":1,"metersPerPixel":2},
	  "originalFile":"sbsj.pdf","addedAt":"2026-10-12T00:00:00Z"}`
	if _, err := lib.Add("c1", []byte("png"), []byte(meta), []byte("%PDF-1.4")); err != nil {
		t.Fatal(err)
	}
	srv := New(lib, nil, func() Position { return Position{} }, "127.0.0.1:8080").Handler()
	do := func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = "127.0.0.1:8080"
		req.RemoteAddr = "127.0.0.1:50000" // this computer
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}

	first := do("GET", "/charts/c1/chart.json", "")
	if first.Code != http.StatusOK {
		t.Fatalf("chart.json: %d", first.Code)
	}
	if cc := first.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("chart.json Cache-Control = %q, want no-cache (it must be revalidated)", cc)
	}
	if cc := do("GET", "/charts/c1/chart.png", "").Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("chart.png Cache-Control = %q, want no-cache", cc)
	}

	// edit through the API; the very next read must show the new properties and name
	patch := do("PATCH", "/api/charts/c1", `{"icao":"SBSJ","type":"IAC","title":"RNP A RWY 15"}`)
	if patch.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", patch.Code, patch.Body.String())
	}
	after := do("GET", "/charts/c1/chart.json", "").Body.String()
	if !strings.Contains(after, "SBSJ · IAC · RNP A RWY 15") {
		t.Fatalf("chart.json did not show the edited name: %s", after)
	}

	// an invalid value is rejected with 400 and changes nothing
	bad := do("PATCH", "/api/charts/c1", `{"validity":"31/12/2026"}`)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("invalid validity: %d, want 400", bad.Code)
	}
	if got := do("GET", "/api/charts/c1", "").Body.String(); !strings.Contains(got, "RNP A RWY 15") {
		t.Fatalf("rejected patch changed the chart: %s", got)
	}
}
