package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

// A route is checked when it is saved and kept with its places: listing it gives them back, and
// sending it somewhere by its id does not resolve the text again.
func TestSavedRouteIsKeptResolved(t *testing.T) {
	s, _ := exportServer(t)

	w := post(s.handleRouteSave, `{"text":"SBSP ZZZZZ SBBR"}`)
	if w.Code != 422 || !strings.Contains(w.Body.String(), "ZZZZZ") {
		t.Fatalf("a route with a name that is not found is not saved: %d %s", w.Code, w.Body)
	}
	if w = post(s.handleRouteSave, `{"text":"SBSP SBBR"}`); w.Code != 200 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	var saved struct {
		ID     int64
		Name   string
		Result struct{ Points []struct{ Ident string } }
	}
	if err := json.Unmarshal(w.Body.Bytes(), &saved); err != nil || saved.ID == 0 || saved.Name != "SBSP > SBBR" || len(saved.Result.Points) != 2 {
		t.Fatalf("%+v %v %s", saved, err, w.Body)
	}

	lw := httptest.NewRecorder()
	s.handleRoutes(lw, httptest.NewRequest("GET", "/api/routes", nil))
	var list struct {
		Routes []struct {
			Result struct{ Points []struct{ Ident string } }
		}
	}
	if err := json.Unmarshal(lw.Body.Bytes(), &list); err != nil || len(list.Routes) != 1 || len(list.Routes[0].Result.Points) != 2 {
		t.Fatalf("%v %s", err, lw.Body)
	}

	// the airports are gone from the database: the saved route still exports, from what was kept
	if err := s.navDB().DeleteAirport("SBSP"); err != nil {
		t.Fatal(err)
	}
	if err := s.navDB().DeleteAirport("SBBR"); err != nil {
		t.Fatal(err)
	}
	w = post(s.handleRouteIVAO, `{"id":`+strconv.FormatInt(saved.ID, 10)+`}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"departure":"SBSP"`) {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
}
