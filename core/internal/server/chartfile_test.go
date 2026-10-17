package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// chartServer is a server over a real library with one chart, "c1"; it returns the way to call it.
func chartServer(t *testing.T) func(method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	lib, err := store.Open(t.TempDir())
	require.NoError(t, err)
	meta := `{"id":"c1","name":"SBSJ · ADC","source":"manual","icao":"SBSJ","type":"ADC","title":"",
	  "image":{"file":"chart.png","width":10,"height":10,"dpi":72},"georef":{"errorMeters":1,"metersPerPixel":2},
	  "originalFile":"sbsj.pdf","addedAt":"2026-10-12T00:00:00Z"}`
	_, err = lib.Add("c1", []byte("png"), []byte(meta), []byte("%PDF-1.4"))
	require.NoError(t, err)
	srv := New(lib, nil, sim.Provider{}, "127.0.0.1:8080").Handler()
	return func(method, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = "127.0.0.1:8080"
		req.RemoteAddr = "127.0.0.1:50000" // this computer
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}
}

// What the browser reads of a chart must be revalidated, so that an edit shows at once.
func TestChartFilesAreRevalidated(t *testing.T) {
	for _, file := range []string{"chart.json", "chart.png"} {
		t.Run(file, func(t *testing.T) {
			// Arrange
			do := chartServer(t)

			// Act
			r := do("GET", "/charts/c1/"+file, "")

			// Assert
			require.Equal(t, http.StatusOK, r.Code)
			assert.Equal(t, "no-cache", r.Header().Get("Cache-Control"))
		})
	}
}

func TestAnEditShowsInTheNextRead(t *testing.T) {
	// Arrange
	do := chartServer(t)

	// Act
	patch := do("PATCH", "/api/charts/c1", `{"icao":"SBSJ","type":"IAC","title":"RNP A RWY 15"}`)

	// Assert
	require.Equal(t, http.StatusOK, patch.Code, patch.Body.String())
	assert.Contains(t, do("GET", "/charts/c1/chart.json", "").Body.String(), "SBSJ · IAC · RNP A RWY 15")
}

func TestAnInvalidEditIsRefusedAndChangesNothing(t *testing.T) {
	// Arrange
	do := chartServer(t)
	require.Equal(t, http.StatusOK, do("PATCH", "/api/charts/c1", `{"icao":"SBSJ","type":"IAC","title":"RNP A RWY 15"}`).Code)

	// Act
	bad := do("PATCH", "/api/charts/c1", `{"validity":"31/12/2026"}`)

	// Assert
	assert.Equal(t, http.StatusBadRequest, bad.Code)
	assert.Contains(t, do("GET", "/api/charts/c1", "").Body.String(), "RNP A RWY 15")
}
