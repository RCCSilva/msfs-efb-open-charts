package chartfox

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tokenServer answers /oauth/token and records the forms it received.
func tokenServer(t *testing.T, status int, body string) (*httptest.Server, *[]url.Values) {
	t.Helper()
	var forms []url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		forms = append(forms, r.PostForm)
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &forms
}

func testConfig(tokenURL, redirect string) Config {
	return Config{ClientID: "cid", AuthURL: "https://auth.example/authorize", TokenURL: tokenURL, Redirect: redirect, Scopes: []string{"a:b", "c:d"}}
}

func TestPKCEChallengeIsTheS256OfTheVerifier(t *testing.T) {
	// Act
	verifier, challenge := PKCE()

	// Assert
	sum := sha256.Sum256([]byte(verifier))
	assert.Equal(t, base64.RawURLEncoding.EncodeToString(sum[:]), challenge)
	assert.GreaterOrEqual(t, len(verifier), 43)
}

func TestAuthorizeURLAsksForPKCEAndOnlyTheScopesOfTheConfig(t *testing.T) {
	// Arrange
	cfg := testConfig("", "http://127.0.0.1:1/cb")

	// Act
	u, err := url.Parse(cfg.AuthorizeURL("st", "ch"))

	// Assert
	require.NoError(t, err)
	q := u.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "cid", q.Get("client_id"))
	assert.Equal(t, "a:b c:d", q.Get("scope"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Equal(t, "ch", q.Get("code_challenge"))
	assert.Equal(t, "st", q.Get("state"))
	assert.Empty(t, q.Get("client_secret"))
}

func TestDefaultConfigAsksNoSourceURLNorTelemetry(t *testing.T) {
	// Act
	cfg := DefaultConfig()

	// Assert
	assert.NotContains(t, cfg.Scopes, "charts:view_source_url")
	assert.NotContains(t, cfg.Scopes, "oauth:telemetry:view")
	assert.Equal(t, "http://127.0.0.1:43219/oauth/chartfox/callback", cfg.Redirect)
}

func TestDefaultConfigTakesTheClientIDFromTheEnvironment(t *testing.T) {
	// Arrange
	t.Setenv("CHARTFOX_CLIENT_ID", "from-env")

	// Act
	cfg := DefaultConfig()

	// Assert
	assert.Equal(t, "from-env", cfg.ClientID)
	assert.True(t, cfg.Available())
}

func TestExchangeSendsTheVerifierAndReadsTheToken(t *testing.T) {
	// Arrange
	srv, forms := tokenServer(t, 200, `{"access_token":"A","refresh_token":"R","expires_in":3600,"scope":"x"}`)
	cfg := testConfig(srv.URL, "http://127.0.0.1:1/cb")

	// Act
	tok, err := cfg.Exchange(context.Background(), srv.Client(), "the-code", "the-verifier")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "A", tok.Access)
	assert.Equal(t, "R", tok.Refresh)
	assert.WithinDuration(t, time.Now().Add(time.Hour), tok.Expires, time.Minute)
	require.Len(t, *forms, 1)
	f := (*forms)[0]
	assert.Equal(t, "authorization_code", f.Get("grant_type"))
	assert.Equal(t, "the-code", f.Get("code"))
	assert.Equal(t, "the-verifier", f.Get("code_verifier"))
	assert.Empty(t, f.Get("client_secret"))
}

func TestExchangeRefusedIsUnauthorized(t *testing.T) {
	// Arrange
	srv, _ := tokenServer(t, 400, `{"error":"invalid_grant"}`)
	cfg := testConfig(srv.URL, "http://127.0.0.1:1/cb")

	// Act
	_, err := cfg.Exchange(context.Background(), srv.Client(), "bad", "v")

	// Assert
	assert.ErrorIs(t, err, ErrUnauthorized)
}

// login runs Login against a listener of its own and plays the browser with visit.
func login(t *testing.T, tokenURL string, ctx context.Context, visit func(authURL, redirect string)) (Token, error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	redirect := "http://" + ln.Addr().String() + "/oauth/chartfox/callback"
	cfg := testConfig(tokenURL, redirect)
	return Login(ctx, cfg, http.DefaultClient, ln, func(authURL string) error {
		go visit(authURL, redirect)
		return nil
	})
}

func TestLoginTradesTheCodeForAToken(t *testing.T) {
	// Arrange
	srv, forms := tokenServer(t, 200, `{"access_token":"A","expires_in":60}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act
	tok, err := login(t, srv.URL, ctx, func(authURL, redirect string) {
		state := mustQuery(authURL, "state")
		resp, e := http.Get(redirect + "?code=abc&state=" + url.QueryEscape(state))
		if e == nil {
			resp.Body.Close()
		}
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "A", tok.Access)
	assert.Equal(t, "abc", (*forms)[0].Get("code"))
	assert.NotEmpty(t, (*forms)[0].Get("code_verifier"))
}

func TestLoginIgnoresAWrongStateAndKeepsWaiting(t *testing.T) {
	// Arrange
	srv, _ := tokenServer(t, 200, `{"access_token":"A","expires_in":60}`)
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	// Act
	_, err := login(t, srv.URL, ctx, func(authURL, redirect string) {
		resp, e := http.Get(redirect + "?code=abc&state=forged")
		if e == nil {
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
			resp.Body.Close()
		}
	})

	// Assert: the forged request did not end the login
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestLoginDenied(t *testing.T) {
	// Arrange
	srv, _ := tokenServer(t, 200, `{"access_token":"A","expires_in":60}`)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Act
	_, err := login(t, srv.URL, ctx, func(authURL, redirect string) {
		state := mustQuery(authURL, "state")
		resp, e := http.Get(redirect + "?error=access_denied&state=" + url.QueryEscape(state))
		if e == nil {
			resp.Body.Close()
		}
	})

	// Assert
	assert.ErrorIs(t, err, ErrDenied)
}

func TestLoginGivesUpWhenNobodyAnswers(t *testing.T) {
	// Arrange
	srv, _ := tokenServer(t, 200, `{}`)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	// Act
	_, err := login(t, srv.URL, ctx, func(string, string) {})

	// Assert
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func mustQuery(raw, key string) string {
	u, _ := url.Parse(raw)
	return u.Query().Get(key)
}

func TestFileStoreRoundTripAndDelete(t *testing.T) {
	// Arrange
	s := FileStore{Path: filepath.Join(t.TempDir(), "sub", "chartfox.json")}
	want := Token{Access: "A", Refresh: "R", Expires: time.Now().Add(time.Hour).Round(time.Second)}

	// Act
	require.NoError(t, s.Save(want))
	got, ok := s.Load()
	require.NoError(t, s.Delete())
	_, okAfter := s.Load()

	// Assert
	assert.True(t, ok)
	assert.True(t, want.Expires.Equal(got.Expires))
	assert.Equal(t, "R", got.Refresh)
	assert.False(t, okAfter)
	assert.NoError(t, s.Delete(), "deleting what is not there is fine")
}

func TestConnectionRefreshesAnExpiredToken(t *testing.T) {
	// Arrange
	srv, forms := tokenServer(t, 200, `{"access_token":"NEW","expires_in":3600}`)
	store := FileStore{Path: filepath.Join(t.TempDir(), "t.json")}
	require.NoError(t, store.Save(Token{Access: "OLD", Refresh: "R", Expires: time.Now().Add(-time.Hour)}))
	c := NewConnection(testConfig(srv.URL, ""), srv.Client(), store)

	// Act
	got, err := c.Token(context.Background())
	again, _ := c.Token(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "NEW", got)
	assert.Equal(t, "NEW", again)
	assert.Len(t, *forms, 1, "the second call used the stored token")
	assert.Equal(t, "R", (*forms)[0].Get("refresh_token"))
	saved, _ := store.Load()
	assert.Equal(t, "R", saved.Refresh, "the old refresh token is kept when the server sends none")
}

func TestConnectionDropsTheTokenWhenTheRefreshIsRefused(t *testing.T) {
	// Arrange
	srv, _ := tokenServer(t, 400, `{"error":"invalid_grant"}`)
	store := FileStore{Path: filepath.Join(t.TempDir(), "t.json")}
	require.NoError(t, store.Save(Token{Access: "OLD", Refresh: "R", Expires: time.Now().Add(-time.Hour)}))
	c := NewConnection(testConfig(srv.URL, ""), srv.Client(), store)

	// Act
	_, err := c.Token(context.Background())

	// Assert
	assert.ErrorIs(t, err, ErrUnauthorized)
	assert.False(t, c.Connected())
}

func TestConnectionWithoutATokenIsUnauthorized(t *testing.T) {
	// Arrange
	c := NewConnection(testConfig("", ""), http.DefaultClient, FileStore{Path: filepath.Join(t.TempDir(), "none.json")})

	// Act
	_, err := c.Token(context.Background())

	// Assert
	assert.ErrorIs(t, err, ErrUnauthorized)
	assert.False(t, c.Connected())
}

func TestAccountConnectsAndShowsTheUser(t *testing.T) {
	// Arrange
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/token":
			fmt.Fprint(w, `{"access_token":"A","expires_in":3600}`)
		case "/v2/user":
			fmt.Fprint(w, `{"full_name":"Pat Pilot"}`)
		}
	}))
	defer api.Close()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close() // the account listens on this port itself
	cfg := testConfig(api.URL+"/oauth/token", fmt.Sprintf("http://127.0.0.1:%d/oauth/chartfox/callback", port))
	opened := make(chan string, 1)
	acc := NewAccount(cfg, rewriteHost(api), FileStore{Path: filepath.Join(t.TempDir(), "t.json")}, func(authURL string) error {
		opened <- authURL
		return nil
	})

	// Act
	require.NoError(t, acc.Connect())
	authURL := <-opened
	resp, err := http.Get(cfg.Redirect + "?code=abc&state=" + url.QueryEscape(mustQuery(authURL, "state")))
	require.NoError(t, err)
	resp.Body.Close()
	require.Eventually(t, func() bool { return acc.Status().Connected }, 3*time.Second, 10*time.Millisecond)

	// Assert
	st := acc.Status()
	assert.True(t, st.Available)
	assert.False(t, st.Connecting)
	assert.Equal(t, "Pat Pilot", st.User)
	require.NoError(t, acc.Disconnect())
	assert.False(t, acc.Status().Connected)
}

func TestAccountWithoutAClientIDCannotConnect(t *testing.T) {
	// Arrange
	acc := NewAccount(Config{}, http.DefaultClient, FileStore{Path: filepath.Join(t.TempDir(), "t.json")}, func(string) error { return nil })

	// Act
	err := acc.Connect()

	// Assert
	assert.Error(t, err)
	assert.False(t, acc.Status().Available)
}

// rewriteHost sends the calls the account makes to BaseURL to a test server instead.
func rewriteHost(srv *httptest.Server) Doer { return hostRewriter{srv} }

type hostRewriter struct{ srv *httptest.Server }

func (h hostRewriter) Do(r *http.Request) (*http.Response, error) {
	u, _ := url.Parse(h.srv.URL)
	r.URL.Scheme, r.URL.Host = u.Scheme, u.Host
	return h.srv.Client().Do(r)
}
