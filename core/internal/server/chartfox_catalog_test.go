package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/chartfox"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/chartfox/chartfoxmock"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store/storemock"
)

const (
	idSID    = "11111111-aaaa-bbbb-cccc-000000000001"
	idGround = "11111111-aaaa-bbbb-cccc-000000000002"
)

// chartfoxServer is a server over a mocked library, a mocked account and a mocked catalogue.
func chartfoxServer(t *testing.T) (*Server, *storemock.Library, *chartfoxmock.Catalog) {
	t.Helper()
	s, lib := libServer(t)
	cat := new(chartfoxmock.Catalog)
	s.UseChartFox(new(chartfoxmock.Connector), cat)
	return s, lib, cat
}

func postJSON(h http.HandlerFunc, target, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("POST", target, strings.NewReader(body)))
	return w
}

func airportReq(h http.HandlerFunc, icao string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", "/api/chartfox/airports/"+icao, nil)
	r.SetPathValue("icao", icao)
	w := httptest.NewRecorder()
	h(w, r)
	return w
}

func TestChartFoxAirportListsChartsAndMarksTheOnesAlreadyImported(t *testing.T) {
	// Arrange
	s, lib, cat := chartfoxServer(t)
	cat.On("Airport", mock.Anything, "SBSP").Return(chartfox.Airport{Ident: "SBSP", Name: "Congonhas Airport"}, nil).Once()
	cat.On("Charts", mock.Anything, "SBSP").Return([]chartfox.Chart{
		{ID: idSID, Name: "RNAV SEDLO 1A", TypeKey: "SID"},
		{ID: idGround, Name: "SBSP Aerodrome Chart", TypeKey: "GroundLayout"},
	}, nil).Once()
	lib.On("FindByOriginal", chartfoxFile(idSID)).Return(&store.Chart{ID: "x"}, true)
	lib.On("FindByOriginal", chartfoxFile(idGround)).Return(nil, false)

	// Act
	w := airportReq(s.handleChartFoxAirport, "sbsp")

	// Assert
	require.Equal(t, 200, w.Code)
	var got struct {
		Airport chartfox.Airport
		Charts  []chartfoxChart
		Credit  string
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Charts, 2)
	assert.Equal(t, "SID", got.Charts[0].Type)
	assert.True(t, got.Charts[0].Imported)
	assert.Equal(t, "ADC", got.Charts[1].Type)
	assert.False(t, got.Charts[1].Imported)
	assert.Equal(t, chartfox.Credit, got.Credit)
}

func TestChartFoxAirportReusesTheListForAWhile(t *testing.T) {
	// Arrange
	s, lib, cat := chartfoxServer(t)
	cat.On("Airport", mock.Anything, "SBSP").Return(chartfox.Airport{}, nil).Once() // a second call would panic
	cat.On("Charts", mock.Anything, "SBSP").Return([]chartfox.Chart{}, nil).Once()
	lib.On("FindByOriginal", mock.Anything).Return(nil, false).Maybe()

	// Act
	first := airportReq(s.handleChartFoxAirport, "SBSP")
	second := airportReq(s.handleChartFoxAirport, "SBSP")

	// Assert
	assert.Equal(t, 200, first.Code)
	assert.Equal(t, 200, second.Code)
	cat.AssertExpectations(t)
}

func TestChartFoxAirportErrors(t *testing.T) {
	// Arrange
	s, _, cat := chartfoxServer(t)
	cat.On("Airport", mock.Anything, "ZZZZ").Return(chartfox.Airport{}, chartfox.ErrNotFound).Once()
	cat.On("Airport", mock.Anything, "SBGR").Return(chartfox.Airport{}, chartfox.ErrUnauthorized).Once()
	cat.On("Airport", mock.Anything, "SBBR").Return(chartfox.Airport{}, errors.New("boom")).Once()

	// Act
	badCode := airportReq(s.handleChartFoxAirport, "../etc")
	notFound := airportReq(s.handleChartFoxAirport, "ZZZZ")
	unauthorized := airportReq(s.handleChartFoxAirport, "SBGR")
	failed := airportReq(s.handleChartFoxAirport, "SBBR")

	// Assert
	assert.Equal(t, 400, badCode.Code)
	assert.Equal(t, 404, notFound.Code)
	assert.Equal(t, 401, unauthorized.Code)
	assert.Equal(t, 502, failed.Code)
}

func TestChartFoxImportBringsTheChartWithTheMetadataOfChartFox(t *testing.T) {
	// Arrange
	s, lib, cat := chartfoxServer(t)
	lib.On("FindByOriginal", chartfoxFile(idSID)).Return(nil, false)
	cat.On("Chart", mock.Anything, idSID).Return(chartfox.Detail{
		Chart: chartfox.Chart{ID: idSID, AirportICAO: "SBSP", Name: "RNAV SEDLO 1A", TypeKey: "SID"},
		Files: []chartfox.File{{URL: "https://cdn.example/a.pdf"}},
	}, nil)
	cat.On("Download", mock.Anything, "https://cdn.example/a.pdf").Return([]byte("%PDF-1.7"), nil)
	var gotForm importForm
	var gotFile string
	s.importChart = func(data []byte, filename string, f importForm) (*store.Chart, error) {
		gotForm, gotFile = f, filename
		return &store.Chart{ID: "lib-1"}, nil
	}

	// Act
	w := postJSON(s.handleChartFoxImport, "/api/chartfox/import", `{"ids":["`+idSID+`"]}`)

	// Assert
	require.Equal(t, 200, w.Code)
	assert.JSONEq(t, `{"results":[{"id":"`+idSID+`","status":"ok","chartId":"lib-1"}]}`, w.Body.String())
	assert.Equal(t, chartfoxFile(idSID), gotFile)
	assert.Equal(t, "chartfox", gotForm.Source)
	assert.Equal(t, "SBSP", gotForm.ICAO)
	assert.Equal(t, "SID", gotForm.Type)
	assert.Equal(t, "RNAV SEDLO 1A", gotForm.Title)
	assert.Equal(t, "SBSP · SID · RNAV SEDLO 1A", gotForm.Name)
}

func TestChartFoxImportSkipsWhatIsAlreadyInTheLibraryWithoutAskingChartFox(t *testing.T) {
	// Arrange
	s, lib, _ := chartfoxServer(t) // the catalogue mock has nothing set up: any call would panic
	lib.On("FindByOriginal", chartfoxFile(idSID)).Return(&store.Chart{ID: "lib-1"}, true)

	// Act
	w := postJSON(s.handleChartFoxImport, "/api/chartfox/import", `{"ids":["`+idSID+`"]}`)

	// Assert
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"already"`)
}

func TestChartFoxImportReportsEachChartAndStopsWhenTheTokenIsRefused(t *testing.T) {
	// Arrange
	s, lib, cat := chartfoxServer(t)
	lib.On("FindByOriginal", mock.Anything).Return(nil, false)
	cat.On("Chart", mock.Anything, idSID).Return(chartfox.Detail{}, chartfox.ErrUnauthorized).Once()

	// Act
	w := postJSON(s.handleChartFoxImport, "/api/chartfox/import", `{"ids":["`+idSID+`","`+idGround+`"]}`)

	// Assert: the second chart was not even tried (the catalogue has nothing set up for it)
	require.Equal(t, 200, w.Code)
	var got struct{ Results []chartfoxResult }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Results, 1)
	assert.Equal(t, errNotConnected, got.Results[0].Message)
}

func TestChartFoxImportRefusesWhatIsNotAChartID(t *testing.T) {
	// Arrange
	s, _, _ := chartfoxServer(t)
	many := `["` + strings.Repeat(idSID+`","`, maxChartFoxBatch) + idSID + `"]`

	// Act
	empty := postJSON(s.handleChartFoxImport, "/api/chartfox/import", `{"ids":[]}`)
	bad := postJSON(s.handleChartFoxImport, "/api/chartfox/import", `{"ids":["../../x"]}`)
	tooMany := postJSON(s.handleChartFoxImport, "/api/chartfox/import", `{"ids":`+many+`}`)

	// Assert
	assert.Equal(t, 400, empty.Code)
	assert.Equal(t, 400, bad.Code)
	assert.Equal(t, 400, tooMany.Code)
}

func TestChartFoxDisconnectRemovesTheChartsBroughtFromChartFox(t *testing.T) {
	// Arrange
	s, lib, _ := chartfoxServer(t)
	acc := new(chartfoxmock.Connector)
	acc.On("Disconnect").Return(nil)
	acc.On("Status").Return(chartfox.Status{Available: true})
	s.UseChartFox(acc, new(chartfoxmock.Catalog))
	lib.On("Search", store.Query{Source: "chartfox"}).Return(charts("a", "b"))
	lib.On("DeleteMany", []string{"a", "b"}).Return(2, nil)

	// Act
	w := postJSON(s.handleChartFoxDisconnect, "/api/chartfox/disconnect", "")

	// Assert
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"removed":2`)
	lib.AssertExpectations(t)
}
