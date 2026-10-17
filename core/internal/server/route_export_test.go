package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

func exportServer(t *testing.T) (*Server, string) {
	t.Helper()
	lib, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	db, err := navdb.Open(filepath.Join(t.TempDir(), "n.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, a := range []*navdata.Airport{
		{ICAO: "SBSP", Name: "Congonhas", Lat: -23.6261, Lon: -46.6564},
		{ICAO: "SBBR", Name: "Brasilia", Lat: -15.8711, Lon: -47.9186},
	} {
		if err := db.SaveAirport(a); err != nil {
			t.Fatal(err)
		}
	}
	s := New(lib, nil, func() Position { return Position{} }, "127.0.0.1:43212")
	s.SetNav(db, nil, nil, nil)
	dir := t.TempDir()
	s.UseSettings(dir, dir, settings.Settings{}, true)
	return s, dir
}

func post(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
	return w
}

func TestRouteExportToIVAOAndPLN(t *testing.T) {
	s, dir := exportServer(t)

	w := post(s.handleRouteIVAO, `{"route":"SBSP SBBR"}`)
	if w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var out struct{ URL, Departure, Arrival, Route string }
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || out.Departure != "SBSP" || out.Arrival != "SBBR" || out.Route != "" {
		t.Fatalf("%+v %v", out, err)
	}
	u, _ := url.Parse(out.URL)
	raw, err := base64.StdEncoding.DecodeString(u.Query().Get("flightPlan"))
	if err != nil || !strings.Contains(string(raw), `"departureId":"SBSP"`) {
		t.Fatalf("%s %v", raw, err)
	}

	w = post(s.handleRoutePLN, `{"route":"SBSP SBBR"}`)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Disposition"), `filename="SBSP-SBBR.pln"`) ||
		!strings.Contains(w.Body.String(), "<DestinationID>SBBR</DestinationID>") {
		t.Fatalf("%d %v %s", w.Code, w.Header(), w.Body)
	}

	// a name that is not found would be missing from the file: nothing is exported
	if w = post(s.handleRoutePLN, `{"route":"SBSP ZZZZZ SBBR"}`); w.Code != 422 || !strings.Contains(w.Body.String(), "ZZZZZ") {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	// both ends must be airports
	if w = post(s.handleRouteIVAO, `{"route":"SBSP"}`); w.Code != 422 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if w = post(s.handleRouteIVAO, `{"route":""}`); w.Code != 400 {
		t.Fatalf("%d", w.Code)
	}

	// loading: with no link to the simulator, an error; with it, the file is written and handed over
	if w = post(s.handleRoutePLNLoad, `{"route":"SBSP SBBR"}`); w.Code != 503 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var loaded string
	s.SetPlanLoader(func(file string) error { loaded = file; return nil })
	if w = post(s.handleRoutePLNLoad, `{"route":"SBSP SBBR"}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if loaded != filepath.Join(dir, "flightplans", "SBSP-SBBR.pln") {
		t.Fatalf("loaded %q", loaded)
	}
	if b, err := os.ReadFile(loaded); err != nil || !strings.Contains(string(b), "<DepartureID>SBSP</DepartureID>") {
		t.Fatalf("%v %s", err, b)
	}
}
