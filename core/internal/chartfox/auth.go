package chartfox

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// ClientID is the public OAuth client id of the project at ChartFox. It is not a secret (a public client, PKCE, nothing
// else to hide) and is set at build time (`-X ...chartfox.ClientID=...`, from CHARTFOX_CLIENT_ID in the ignored .env file
// at the repository root). The environment variable of the same name overrides it, for development.
var ClientID string

// Config describes the OAuth endpoints and what to ask for.
type Config struct {
	ClientID string
	AuthURL  string
	TokenURL string
	Redirect string // registered at ChartFox: the listener of a login answers here
	Scopes   []string
}

// DefaultConfig is the ChartFox configuration. Only the scopes the app needs: not `charts:view_source_url` (it points to
// third-party sites) and not the telemetry ones.
func DefaultConfig() Config {
	id := ClientID
	if v := os.Getenv("CHARTFOX_CLIENT_ID"); v != "" {
		id = v
	}
	return Config{
		ClientID: id,
		AuthURL:  BaseURL + "/oauth/authorize",
		TokenURL: BaseURL + "/oauth/token",
		Redirect: "http://127.0.0.1:43219/oauth/chartfox/callback",
		Scopes:   []string{"oauth:user:name", "airports:view", "charts:index", "charts:view", "charts:files", "charts:geos"},
	}
}

// Available says whether this build can connect to ChartFox (it knows its client id).
func (c Config) Available() bool { return c.ClientID != "" }

// Token is what the authorization server gives back.
type Token struct {
	Access  string    `json:"access"`
	Refresh string    `json:"refresh,omitempty"`
	Expires time.Time `json:"expires"`
	Scope   string    `json:"scope,omitempty"`
}

// Fresh reports whether the access token is good for at least another minute.
func (t Token) Fresh(now time.Time) bool {
	return t.Access != "" && now.Add(time.Minute).Before(t.Expires)
}

// PKCE makes a code verifier and its S256 challenge.
func PKCE() (verifier, challenge string) {
	verifier = randomString(48)
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)[:n]
}

// AuthorizeURL is the page the user opens to approve the connection.
func (c Config) AuthorizeURL(state, challenge string) string {
	q := url.Values{
		"response_type":         {"code"},
		"client_id":             {c.ClientID},
		"redirect_uri":          {c.Redirect},
		"scope":                 {strings.Join(c.Scopes, " ")},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}
	return c.AuthURL + "?" + q.Encode()
}

// Exchange trades the authorization code for a token.
func (c Config) Exchange(ctx context.Context, h Doer, code, verifier string) (Token, error) {
	return c.token(ctx, h, url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {c.ClientID},
		"redirect_uri":  {c.Redirect},
		"code":          {code},
		"code_verifier": {verifier},
	})
}

// Refresh trades a refresh token for a new token.
func (c Config) Refresh(ctx context.Context, h Doer, refresh string) (Token, error) {
	tok, err := c.token(ctx, h, url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.ClientID},
		"refresh_token": {refresh},
	})
	if err == nil && tok.Refresh == "" {
		tok.Refresh = refresh // the server may keep the same one
	}
	return tok, err
}

func (c Config) token(ctx context.Context, h Doer, form url.Values) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := h.Do(req)
	if err != nil {
		return Token{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		return Token{}, ErrUnauthorized // the code or refresh token is no good: connect again
	}
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("chartfox: token endpoint answered %d", resp.StatusCode)
	}
	var r struct {
		Access  string `json:"access_token"`
		Refresh string `json:"refresh_token"`
		Expires int    `json:"expires_in"`
		Scope   string `json:"scope"`
	}
	if err := json.Unmarshal(body, &r); err != nil || r.Access == "" {
		return Token{}, errors.New("chartfox: unreadable token answer")
	}
	return Token{Access: r.Access, Refresh: r.Refresh, Expires: time.Now().Add(time.Duration(r.Expires) * time.Second), Scope: r.Scope}, nil
}
