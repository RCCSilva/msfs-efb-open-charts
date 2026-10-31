package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store/storemock"
)

// libServer is a server over a mocked library: the handlers are tested without any chart files.
func libServer(t *testing.T) (*Server, *storemock.Library) {
	t.Helper()
	lib := &storemock.Library{}
	lib.On("Dir").Return(t.TempDir()).Maybe()
	return New(lib, nil, sim.Provider{}, "127.0.0.1:43215"), lib
}

func charts(ids ...string) []*store.Chart {
	var out []*store.Chart
	for _, id := range ids {
		out = append(out, &store.Chart{ID: id})
	}
	return out
}

func getReq(h http.HandlerFunc, target string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h(w, httptest.NewRequest("GET", target, nil))
	return w
}

func TestLibraryPagesTheMatchingCharts(t *testing.T) {
	// Arrange
	s, lib := libServer(t)
	lib.On("Search", store.Query{Text: "sbsp"}).Return(charts("a", "b", "c"))
	lib.On("Types").Return([]string{"IAP"})
	lib.On("Count").Return(10)

	// Act
	w := getReq(s.handleLibrary, "/api/library?q=sbsp&offset=1&limit=1")

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	var out struct {
		Total, All, Offset, Limit int
		Items                     []store.Chart
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Equal(t, 3, out.Total)
	assert.Equal(t, 10, out.All)
	assert.Equal(t, 1, out.Offset)
	require.Len(t, out.Items, 1)
	assert.Equal(t, "b", out.Items[0].ID)
}

func TestLibraryWithoutICAOKeepsOnlyTheChartsWithNone(t *testing.T) {
	// Arrange
	s, lib := libServer(t)
	visual := &store.Chart{ID: "v"}
	lib.On("Search", store.Query{}).Return([]*store.Chart{{ID: "a", ICAO: "SBSP"}, visual})
	lib.On("Types").Return([]string(nil))
	lib.On("Count").Return(2)

	// Act
	w := getReq(s.handleLibrary, "/api/library?icao=-&ids=1")

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.JSONEq(t, `{"total":1,"all":2,"types":null,"ids":["v"]}`, w.Body.String())
}

func TestDeleteManyRefusesAnEmptySelection(t *testing.T) {
	// Arrange
	s, lib := libServer(t)

	// Act
	w := post(s.handleDeleteMany, `{"ids":[]}`)

	// Assert
	assert.Equal(t, 400, w.Code)
	lib.AssertNotCalled(t, "DeleteMany", mock.Anything)
}

func TestDeleteManyAnswersWithTheGroupsThatRemain(t *testing.T) {
	// Arrange
	s, lib := libServer(t)
	lib.On("DeleteMany", []string{"a", "b"}).Return(2, nil)
	lib.On("Groups").Return(store.GroupsState{})

	// Act
	w := post(s.handleDeleteMany, `{"ids":["a","b"]}`)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"deleted":2`)
	lib.AssertExpectations(t)
}

func TestDeleteManyReportsWhyItFailed(t *testing.T) {
	// Arrange
	s, lib := libServer(t)
	lib.On("DeleteMany", []string{"a"}).Return(0, errors.New("disk full"))

	// Act
	w := post(s.handleDeleteMany, `{"ids":["a"]}`)

	// Assert
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "disk full")
}

func TestPinOfAMissingGroupIsNotFound(t *testing.T) {
	// Arrange
	s, lib := libServer(t)
	lib.On("Pin", "g1", "c1").Return(os.ErrNotExist)
	req := httptest.NewRequest("POST", "/", strings.NewReader(``))
	req.SetPathValue("id", "g1")
	req.SetPathValue("chart", "c1")
	w := httptest.NewRecorder()

	// Act
	s.handlePin(w, req)

	// Assert
	assert.Equal(t, 404, w.Code)
}

func TestSetCurrentAnswersWithTheGroups(t *testing.T) {
	// Arrange
	s, lib := libServer(t)
	lib.On("SetCurrent", "g1").Return(nil)
	lib.On("Groups").Return(store.GroupsState{Current: "g1"})

	// Act
	w := post(s.handleSetCurrent, `{"id":"g1"}`)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "g1")
}

func TestDeleteAllAnswersWithHowManyWereRemoved(t *testing.T) {
	// Arrange
	s, lib := libServer(t)
	lib.On("DeleteAll").Return(3, nil)
	lib.On("Groups").Return(store.GroupsState{})

	// Act
	w := post(s.handleDeleteAll, ``)

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), `"deleted":3`)
	lib.AssertExpectations(t)
}

func TestDeleteAllReportsWhyItFailed(t *testing.T) {
	// Arrange
	s, lib := libServer(t)
	lib.On("DeleteAll").Return(1, errors.New("disk full"))

	// Act
	w := post(s.handleDeleteAll, ``)

	// Assert
	assert.Equal(t, 500, w.Code)
	assert.Contains(t, w.Body.String(), "disk full")
}

func TestDeleteAllIsOnlyForTheComputerItself(t *testing.T) {
	// Arrange / Act / Assert
	assert.True(t, localOnlyPath("/api/library/delete-all"))
	assert.False(t, localOnlyPath("/api/library/delete"))
}
