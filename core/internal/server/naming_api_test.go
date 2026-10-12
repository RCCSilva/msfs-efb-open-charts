package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
)

func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:50000"
	req.Host = "127.0.0.1:43210"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestPatternsAreCheckedSavedAndUsedForMyPDFs(t *testing.T) {
	s, h := newAccessServer(t, false)

	// a wrong pattern is refused, and nothing is saved
	if r := send(h, "PUT", "/api/naming/patterns", `{"patterns":["{icao}_{type}","{airport}_{title}"]}`); r.Code != 400 {
		t.Fatalf("a wrong pattern must be refused, got %d %s", r.Code, r.Body.String())
	}
	if len(s.settingsSnapshot().NamePatterns) != 0 {
		t.Fatal("nothing should have been saved")
	}

	if r := send(h, "PUT", "/api/naming/patterns", `{"patterns":["{icao}_{type}_{title}_{date:YYYYMMDD}"]}`); r.Code != 200 {
		t.Fatalf("PUT: %d %s", r.Code, r.Body.String())
	}
	var got struct {
		Patterns []string `json:"patterns"`
	}
	r := send(h, "GET", "/api/naming/patterns", "")
	if err := json.Unmarshal(r.Body.Bytes(), &got); err != nil || len(got.Patterns) != 1 {
		t.Fatalf("GET = %s (%v)", r.Body.String(), err)
	}

	// the identity of a file of the user's: read by the pattern, and sure enough to import without asking
	id := s.identify("manual", "SBSP_IAC_ILS-X-RWY-17R_20260806", "SBSP_IAC_ILS-X-RWY-17R_20260806.pdf", "BaseMap_REA")
	if id.ICAO != "SBSP" || id.Type != "IAC" || id.Title != "ILS X RWY 17R" || id.Validity != "2026-08-06" || !id.Confident {
		t.Fatalf("identity = %+v", id)
	}
	// a name that does not fit is left to the usual rules (no ICAO or type invented)
	other := s.identify("manual", "scan 0042", "scan 0042.pdf", "")
	if other.ICAO != "" || other.Confident {
		t.Fatalf("an unknown name must not be confident: %+v", other)
	}
	// the patterns belong to My PDFs only: a provider's files keep the provider's rules
	if d := s.identify("decea-br", "SBSP_IAC_ILS-X-RWY-17R_20260806", "x.pdf", ""); d.Recognizer == "pattern" {
		t.Fatal("a provider's files must not use the user's patterns")
	}

	// the test endpoint
	r = send(h, "POST", "/api/naming/test", `{"pattern":"{type}-{icao}","sample":"SID-SBGR.pdf"}`)
	var tr struct {
		OK   bool   `json:"ok"`
		ICAO string `json:"icao"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(r.Body.Bytes(), &tr); err != nil || !tr.OK || tr.ICAO != "SBGR" || tr.Type != "SID" {
		t.Fatalf("test = %s", r.Body.String())
	}
	_ = settings.MaxNamePatterns
}
