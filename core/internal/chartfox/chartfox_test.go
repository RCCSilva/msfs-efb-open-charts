package chartfox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fixedToken string

func (f fixedToken) Token(context.Context) (string, error) { return string(f), nil }

func TestKind(t *testing.T) {
	// Arrange
	cases := map[string]string{"Approach": "IAC", "SID": "SID", "STAR": "STAR", "GroundLayout": "ADC", "Unknown": "", "Whatever": ""}

	for typeKey, want := range cases {
		// Act
		got := Kind(typeKey)

		// Assert
		assert.Equal(t, want, got, typeKey)
	}
}

func TestGeoRefRoundTrip(t *testing.T) {
	// Arrange: the numbers of the documentation's example
	g := GeoRef{TX: 1489.83, TY: 1729.55, K: 3.9, Angle: 264.1, PageRotation: 90, Page: 1}

	// Act
	cx, cy := g.ChartPoint(2000, 3000, 1000)
	x, y := g.World(cx, cy, 1000)

	// Assert
	assert.InDelta(t, 2000, x, 1e-6)
	assert.InDelta(t, 3000, y, 1e-6)
}

func TestGeoRefOriginIsTranslation(t *testing.T) {
	// Arrange
	g := GeoRef{TX: 100, TY: 200, K: 2, Angle: 0}

	// Act
	x, y := g.World(0, 0, 1)

	// Assert
	assert.Equal(t, 100.0, x)
	assert.Equal(t, 200.0, y)
}

func TestDetailNeverKeepsTheSourceURL(t *testing.T) {
	// Arrange: a detail as the API sends it, with the publisher's link and its key inside
	raw := `{"id":"c1","airport_icao":"SBSP","name":"SBSP Aerodrome Chart","type_key":"GroundLayout",
	  "source_url":"https://publisher.example/download?apikey=SECRET","source":{"display_name":"Decea","copyright_statement_short":"&#169; Decea"},
	  "files":[{"type":0,"url":"https://cdn.example/c1.pdf"}],"georefs":[]}`

	// Act
	var d Detail
	err := json.Unmarshal([]byte(raw), &d)
	back, _ := json.Marshal(d)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Decea", d.Source.DisplayName)
	assert.NotContains(t, string(back), "SECRET")
	assert.NotContains(t, string(back), "publisher.example")
}

func TestClientListsAllPagesAndSendsTheToken(t *testing.T) {
	// Arrange
	var auth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = append(auth, r.Header.Get("Authorization"))
		switch r.URL.Query().Get("page") {
		case "1":
			fmt.Fprint(w, `{"data":[{"id":"a","type_key":"SID"}],"meta":{"current_page":1,"last_page":2}}`)
		default:
			fmt.Fprint(w, `{"data":[{"id":"b","type_key":"STAR"}],"meta":{"current_page":2,"last_page":2}}`)
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client(), fixedToken("tok"))

	// Act
	charts, err := c.Charts(context.Background(), "sbsp")

	// Assert
	require.NoError(t, err)
	require.Len(t, charts, 2)
	assert.Equal(t, "b", charts[1].ID)
	assert.Equal(t, []string{"Bearer tok", "Bearer tok"}, auth)
}

func TestClientMapsStatusCodes(t *testing.T) {
	// Arrange
	status := http.StatusUnauthorized
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) }))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client(), fixedToken("tok"))

	// Act
	_, errUnauthorized := c.Airport(context.Background(), "SBSP")
	status = http.StatusNotFound
	_, errNotFound := c.Airport(context.Background(), "ZZZZ")
	status = http.StatusInternalServerError
	_, errServer := c.Airport(context.Background(), "SBSP")

	// Assert
	assert.ErrorIs(t, errUnauthorized, ErrUnauthorized)
	assert.ErrorIs(t, errNotFound, ErrNotFound)
	assert.Error(t, errServer)
}

func TestDownloadDoesNotSendTheToken(t *testing.T) {
	// Arrange
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Authorization")
		fmt.Fprint(w, "%PDF-1.7")
	}))
	defer srv.Close()
	c := NewClient(srv.URL, srv.Client(), fixedToken("tok"))

	// Act
	b, err := c.Download(context.Background(), srv.URL+"/c1.pdf")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "%PDF-1.7", string(b))
	assert.Empty(t, got)
}

func TestDownloadRefusesPlainHTTPElsewhere(t *testing.T) {
	// Arrange
	c := NewClient(BaseURL, http.DefaultClient, fixedToken("tok"))

	// Act
	_, err := c.Download(context.Background(), "http://elsewhere.example/c.pdf")

	// Assert
	assert.Error(t, err)
}
