package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
)

type savedView struct {
	ID     int64
	Name   string
	Result struct{ Points []struct{ Ident string } }
}

func saveRoute(t *testing.T, s *Server, body string) savedView {
	t.Helper()
	w := post(s.handleRouteSave, body)
	require.Equal(t, 200, w.Code, w.Body.String())
	var saved savedView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &saved))
	return saved
}

func TestARouteWithANameThatIsNotFoundIsNotSaved(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})

	// Act
	w := post(s.handleRouteSave, `{"text":"SBSP ZZZZZ SBBR"}`)

	// Assert
	assert.Equal(t, 422, w.Code)
	assert.Contains(t, w.Body.String(), "ZZZZZ")
	list := httptest.NewRecorder()
	s.handleRoutes(list, httptest.NewRequest("GET", "/api/routes", nil))
	assert.Contains(t, list.Body.String(), `"routes":[]`, "nothing was stored")
}

func TestASavedRouteIsNamedAndKeptWithItsPlaces(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})

	// Act
	saved := saveRoute(t, s, `{"text":"SBSP SBBR"}`)

	// Assert
	assert.NotZero(t, saved.ID)
	assert.Equal(t, "SBSP > SBBR", saved.Name)
	assert.Len(t, saved.Result.Points, 2)
}

func TestTheListOfSavedRoutesHasThePlacesKept(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})
	saveRoute(t, s, `{"text":"SBSP SBBR"}`)

	// Act
	w := httptest.NewRecorder()
	s.handleRoutes(w, httptest.NewRequest("GET", "/api/routes", nil))

	// Assert
	require.Equal(t, 200, w.Code)
	var list struct{ Routes []savedView }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	require.Len(t, list.Routes, 1)
	assert.Len(t, list.Routes[0].Result.Points, 2)
}

// The airports are gone from the database, and the saved route still exports from what was kept.
func TestASavedRouteIsExportedFromWhatWasKept(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})
	saved := saveRoute(t, s, `{"text":"SBSP SBBR"}`)
	require.NoError(t, s.navDB().DeleteAirport("SBSP"))
	require.NoError(t, s.navDB().DeleteAirport("SBBR"))

	// Act
	w := post(s.handleRouteIVAO, `{"id":`+strconv.FormatInt(saved.ID, 10)+`}`)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"departure":"SBSP"`)
}

func TestASavedRouteIsReplacedByItsID(t *testing.T) {
	// Arrange
	s, _ := exportServer(t, sim.Provider{})
	saved := saveRoute(t, s, `{"text":"SBSP SBBR"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest("PUT", "/api/routes/"+strconv.FormatInt(saved.ID, 10), strings.NewReader(`{"name":"back","text":"SBBR SBSP"}`))
	req.SetPathValue("id", strconv.FormatInt(saved.ID, 10))

	// Act
	s.handleRouteSave(w, req)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	var got savedView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, saved.ID, got.ID)
	assert.Equal(t, "back", got.Name)
}

func TestTheStatusOfWhatCannotBeDoneToASavedRoute(t *testing.T) {
	cases := []struct {
		name   string
		method string
		id     string
		body   string
		want   int
	}{
		{"replacing one that does not exist", "PUT", "999", `{"text":"SBSP SBBR"}`, http.StatusNotFound},
		{"replacing with an id that is not a number", "PUT", "abc", `{"text":"SBSP SBBR"}`, http.StatusBadRequest},
		{"saving an empty route", "POST", "", `{"text":"  "}`, http.StatusBadRequest},
		{"deleting one that does not exist", "DELETE", "999", ``, http.StatusNotFound},
		{"deleting with an id that is not a number", "DELETE", "abc", ``, http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			s, _ := exportServer(t, sim.Provider{})
			req := httptest.NewRequest(c.method, "/api/routes/"+c.id, strings.NewReader(c.body))
			if c.id != "" {
				req.SetPathValue("id", c.id)
			}
			w := httptest.NewRecorder()

			// Act
			if c.method == "DELETE" {
				s.handleRouteDelete(w, req)
			} else {
				s.handleRouteSave(w, req)
			}

			// Assert
			assert.Equal(t, c.want, w.Code, w.Body.String())
		})
	}
}
