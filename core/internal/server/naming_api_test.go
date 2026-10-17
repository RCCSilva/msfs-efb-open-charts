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

func send(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.RemoteAddr = "127.0.0.1:50000"
	req.Host = "127.0.0.1:43210"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

const goodPattern = `{"patterns":["{icao}_{type}_{title}_{date:YYYYMMDD}"]}`

func TestAWrongPatternIsRefusedAndNothingIsSaved(t *testing.T) {
	// Arrange
	s, h := newAccessServer(t, false)

	// Act
	r := send(h, "PUT", "/api/naming/patterns", `{"patterns":["{icao}_{type}","{airport}_{title}"]}`)

	// Assert
	assert.Equal(t, 400, r.Code, r.Body.String())
	assert.Empty(t, s.settingsSnapshot().NamePatterns)
}

func TestPatternsAreSavedAndListed(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, false)

	// Act
	put := send(h, "PUT", "/api/naming/patterns", goodPattern)
	list := send(h, "GET", "/api/naming/patterns", "")

	// Assert
	require.Equal(t, 200, put.Code, put.Body.String())
	var got struct {
		Patterns []string `json:"patterns"`
	}
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &got))
	assert.Len(t, got.Patterns, 1)
}

func TestAFileOfTheUserIsReadByThePattern(t *testing.T) {
	// Arrange
	s, h := newAccessServer(t, false)
	require.Equal(t, 200, send(h, "PUT", "/api/naming/patterns", goodPattern).Code)

	// Act
	id := s.identify("manual", "SBSP_IAC_ILS-X-RWY-17R_20260806", "SBSP_IAC_ILS-X-RWY-17R_20260806.pdf", "BaseMap_REA")

	// Assert
	assert.Equal(t, "SBSP", id.ICAO)
	assert.Equal(t, "IAC", id.Type)
	assert.Equal(t, "ILS X RWY 17R", id.Title)
	assert.Equal(t, "2026-08-06", id.Validity)
	assert.True(t, id.Confident, "sure enough to import without asking")
}

func TestANameThatDoesNotFitIsLeftToTheUsualRules(t *testing.T) {
	// Arrange
	s, h := newAccessServer(t, false)
	require.Equal(t, 200, send(h, "PUT", "/api/naming/patterns", goodPattern).Code)

	// Act
	id := s.identify("manual", "scan 0042", "scan 0042.pdf", "")

	// Assert
	assert.Empty(t, id.ICAO, "no ICAO is invented")
	assert.False(t, id.Confident)
}

func TestThePatternsBelongToMyPDFsOnly(t *testing.T) {
	// Arrange
	s, h := newAccessServer(t, false)
	require.Equal(t, 200, send(h, "PUT", "/api/naming/patterns", goodPattern).Code)

	// Act
	id := s.identify("decea-br", "SBSP_IAC_ILS-X-RWY-17R_20260806", "x.pdf", "")

	// Assert
	assert.NotEqual(t, "pattern", id.Recognizer, "the files of a provider keep the rules of the provider")
}

func TestThePatternTestEndpointReadsASample(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, false)

	// Act
	r := send(h, "POST", "/api/naming/test", `{"pattern":"{type}-{icao}","sample":"SID-SBGR.pdf"}`)

	// Assert
	var got struct {
		OK   bool   `json:"ok"`
		ICAO string `json:"icao"`
		Type string `json:"type"`
	}
	require.NoError(t, json.Unmarshal(r.Body.Bytes(), &got), r.Body.String())
	assert.True(t, got.OK)
	assert.Equal(t, "SBGR", got.ICAO)
	assert.Equal(t, "SID", got.Type)
}
