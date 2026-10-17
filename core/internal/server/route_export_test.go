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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/simmock"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

func exportServer(t *testing.T, provider sim.Provider) (*Server, string) {
	t.Helper()
	lib, err := store.Open(t.TempDir())
	require.NoError(t, err)
	db, err := navdb.Open(filepath.Join(t.TempDir(), "n.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	for _, a := range []*navdata.Airport{
		{ICAO: "SBSP", Name: "Congonhas", Lat: -23.6261, Lon: -46.6564},
		{ICAO: "SBBR", Name: "Brasilia", Lat: -15.8711, Lon: -47.9186},
	} {
		require.NoError(t, db.SaveAirport(a))
	}
	s := New(lib, nil, provider, "127.0.0.1:43212")
	s.SetNav(db)
	dir := t.TempDir()
	s.UseSettings(dir, dir, settings.Settings{}, true)
	return s, dir
}

func post(h http.HandlerFunc, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("POST", "/", strings.NewReader(body)))
	return w
}

func TestRouteExportToIVAO(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})

	// Act
	w := post(s.handleRouteIVAO, `{"route":"SBSP SBBR"}`)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	var out struct{ URL, Departure, Arrival, Route string }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, "SBSP", out.Departure)
	assert.Equal(t, "SBBR", out.Arrival)
	assert.Empty(t, out.Route)
	u, err := url.Parse(out.URL)
	require.NoError(t, err)
	raw, err := base64.StdEncoding.DecodeString(u.Query().Get("flightPlan"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"departureId":"SBSP"`)
}

func TestRouteExportToPLN(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})

	// Act
	w := post(s.handleRoutePLN, `{"route":"SBSP SBBR"}`)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Header().Get("Content-Disposition"), `filename="SBSP-SBBR.pln"`)
	assert.Contains(t, w.Body.String(), "<DestinationID>SBBR</DestinationID>")
}

func TestRouteExportRefusesWhatWouldBeIncomplete(t *testing.T) {
	cases := []struct {
		name, body string
		want       int
		contains   string
	}{
		{"a name that is not found would be missing from the file", `{"route":"SBSP ZZZZZ SBBR"}`, 422, "ZZZZZ"},
		{"both ends must be airports", `{"route":"SBSP"}`, 422, ""},
		{"an empty route", `{"route":""}`, 400, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			s, _ := exportServer(t, sim.Provider{})

			// Act
			w := post(s.handleRoutePLN, c.body)

			// Assert
			assert.Equal(t, c.want, w.Code, w.Body.String())
			assert.Contains(t, w.Body.String(), c.contains)
		})
	}
}

func TestPlanLoadNeedsALinkToTheSimulator(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})

	// Act
	w := post(s.handleRoutePLNLoad, `{"route":"SBSP SBBR"}`)

	// Assert
	assert.Equal(t, 503, w.Code, w.Body.String())
}

func TestPlanLoadWritesTheFileAndHandsItOver(t *testing.T) {
	// Arrange
	plans := new(simmock.Plans)
	s, dir := exportServer(t, sim.Provider{Plans: plans})
	file := filepath.Join(dir, "flightplans", "SBSP-SBBR.pln")
	plans.On("Load", file).Return(nil)

	// Act
	w := post(s.handleRoutePLNLoad, `{"route":"SBSP SBBR"}`)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	plans.AssertExpectations(t)
	written, err := os.ReadFile(file)
	require.NoError(t, err)
	assert.Contains(t, string(written), "<DepartureID>SBSP</DepartureID>")
}
