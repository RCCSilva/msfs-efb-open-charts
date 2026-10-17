package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func readViewerState(t *testing.T, h http.Handler) map[string]any {
	t.Helper()
	r := do(h, reqOpt{from: "127.0.0.1", host: "127.0.0.1:43210", path: "/api/viewer-state"})
	var m map[string]any
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &m))
	return m
}

func TestAircraftSymbolDefaultsToAYellowPlane(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, false)

	// Act
	st := readViewerState(t, h)

	// Assert
	assert.Equal(t, "plane", st["aircraft"])
	assert.Equal(t, "yellow", st["aircraftColor"])
	assert.NotEmpty(t, st["aircraftHex"])
}

func TestAircraftSymbolCanBeChanged(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, false)

	// Act
	r := putSettings(h, `{"aircraft":"helicopter","aircraftColor":"blue"}`)

	// Assert
	require.Equal(t, 200, r.Code, r.Body.String())
	st := readViewerState(t, h)
	assert.Equal(t, "helicopter", st["aircraft"])
	assert.Equal(t, "blue", st["aircraftColor"])
}

func TestAircraftSymbolRefusesUnknownValuesAndKeepsTheCurrentOne(t *testing.T) {
	for _, bad := range []string{`{"aircraft":"rocket"}`, `{"aircraftColor":"plaid"}`} {
		t.Run(bad, func(t *testing.T) {
			// Arrange
			_, h := newAccessServer(t, false)
			require.Equal(t, 200, putSettings(h, `{"aircraft":"helicopter","aircraftColor":"blue"}`).Code)

			// Act
			r := putSettings(h, bad)

			// Assert
			assert.Equal(t, 400, r.Code)
			st := readViewerState(t, h)
			assert.Equal(t, "helicopter", st["aircraft"], "a refused change altered the symbol")
			assert.Equal(t, "blue", st["aircraftColor"])
		})
	}
}
