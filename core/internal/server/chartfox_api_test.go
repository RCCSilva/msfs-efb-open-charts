package server

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/chartfox"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/chartfox/chartfoxmock"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
)

func TestChartFoxStatusWithoutAnAccountIsNotAvailable(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{})

	// Act
	w := httptest.NewRecorder()
	s.handleChartFoxStatus(w, httptest.NewRequest("GET", "/api/chartfox", nil))

	// Assert
	require.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{"available":false,"connected":false,"connecting":false,"search":false}`, w.Body.String())
}

func TestChartFoxConnectStartsALogin(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{})
	acc := new(chartfoxmock.Connector)
	acc.On("Connect").Return(nil).Once()
	acc.On("Status").Return(chartfox.Status{Available: true, Connecting: true}).Once()
	s.UseChartFox(acc, nil)

	// Act
	w := httptest.NewRecorder()
	s.handleChartFoxConnect(w, httptest.NewRequest("POST", "/api/chartfox/connect", nil))

	// Assert
	require.Equal(t, 202, w.Code)
	assert.Contains(t, w.Body.String(), `"connecting":true`)
	acc.AssertExpectations(t)
}

func TestChartFoxConnectReportsWhyItCouldNotStart(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{})
	acc := new(chartfoxmock.Connector)
	acc.On("Connect").Return(errors.New("port 43219 is busy")).Once()
	s.UseChartFox(acc, nil)

	// Act
	w := httptest.NewRecorder()
	s.handleChartFoxConnect(w, httptest.NewRequest("POST", "/api/chartfox/connect", nil))

	// Assert
	require.Equal(t, 409, w.Code)
	assert.Contains(t, w.Body.String(), "port 43219 is busy")
}

func TestChartFoxDisconnect(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{})
	acc := new(chartfoxmock.Connector)
	acc.On("Disconnect").Return(nil).Once()
	acc.On("Status").Return(chartfox.Status{Available: true}).Once()
	s.UseChartFox(acc, nil)

	// Act
	w := httptest.NewRecorder()
	s.handleChartFoxDisconnect(w, httptest.NewRequest("POST", "/api/chartfox/disconnect", nil))

	// Assert
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"connected":false`)
	acc.AssertExpectations(t)
}

func TestChartFoxEndpointsAreOnlyForThisComputer(t *testing.T) {
	// Act and Assert
	assert.True(t, localOnlyPath("/api/chartfox"))
	assert.True(t, localOnlyPath("/api/chartfox/connect"))
	assert.True(t, localOnlyPath("/api/chartfox/disconnect"))
}

func TestChartFoxSearchSwitchIsKeptAndShownInTheStatus(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{})
	dir := t.TempDir()
	s.UseSettings(dir, dir, settings.Default(), true)
	acc := new(chartfoxmock.Connector)
	acc.On("Status").Return(chartfox.Status{Available: true, Connected: true})
	s.UseChartFox(acc, nil)

	// Act
	w := httptest.NewRecorder()
	s.handleChartFoxSearchSwitch(w, httptest.NewRequest("PUT", "/api/chartfox/search", strings.NewReader(`{"on":true}`)))

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"search":true`)
	assert.True(t, settings.Load(dir).ChartFoxSearch, "it is saved")
	st := httptest.NewRecorder()
	s.handleChartFoxStatus(st, httptest.NewRequest("GET", "/api/chartfox", nil))
	assert.Contains(t, st.Body.String(), `"search":true`)
}

func TestChartFoxSearchSwitchRefusesAnythingElse(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{})
	dir := t.TempDir()
	s.UseSettings(dir, dir, settings.Default(), true)

	// Act
	w := httptest.NewRecorder()
	s.handleChartFoxSearchSwitch(w, httptest.NewRequest("PUT", "/api/chartfox/search", strings.NewReader(`nope`)))

	// Assert
	assert.Equal(t, 400, w.Code)
}
