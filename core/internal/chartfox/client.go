package chartfox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// BaseURL is the ChartFox API.
const BaseURL = "https://api.chartfox.org"

// maxFile caps a downloaded chart (the largest ones seen are well under 10 MB).
const maxFile = 64 << 20

// maxPages stops a list that never ends.
const maxPages = 20

// ErrUnauthorized is returned when the token was refused: the user has to connect the account again.
var ErrUnauthorized = errors.New("chartfox: not authorized")

// ErrNotFound is returned for an airport or chart ChartFox does not have.
var ErrNotFound = errors.New("chartfox: not found")

// Doer is what the client needs from an HTTP client.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// TokenSource gives the access token of the connected user.
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// Client calls the ChartFox API on behalf of the connected user.
type Client struct {
	base   string
	http   Doer
	tokens TokenSource
}

// NewClient builds a client. base is BaseURL, or a test server's.
func NewClient(base string, httpClient Doer, tokens TokenSource) *Client {
	return &Client{base: strings.TrimRight(base, "/"), http: httpClient, tokens: tokens}
}

// Airport looks an airport up by ICAO code.
func (c *Client) Airport(ctx context.Context, icao string) (Airport, error) {
	var a Airport
	err := c.getJSON(ctx, "/v2/airports/"+url.PathEscape(strings.ToUpper(icao)), &a)
	return a, err
}

// Charts lists every chart of an airport (all pages).
func (c *Client) Charts(ctx context.Context, icao string) ([]Chart, error) {
	var all []Chart
	for p := 1; p <= maxPages; p++ {
		var pg page[Chart]
		path := fmt.Sprintf("/v2/airports/%s/charts?page=%d", url.PathEscape(strings.ToUpper(icao)), p)
		if err := c.getJSON(ctx, path, &pg); err != nil {
			return nil, err
		}
		all = append(all, pg.Data...)
		if pg.Meta.CurrentPage >= pg.Meta.LastPage {
			break
		}
	}
	return all, nil
}

// Chart gives the detail of one chart.
func (c *Client) Chart(ctx context.Context, id string) (Detail, error) {
	var d Detail
	err := c.getJSON(ctx, "/v2/charts/"+url.PathEscape(id), &d)
	return d, err
}

// Download fetches the PDF of a chart from the file URL of its detail. The files are public on ChartFox's own CDN, so
// the token is not sent there.
func (c *Client) Download(ctx context.Context, fileURL string) ([]byte, error) {
	u, err := url.Parse(fileURL)
	if err != nil || u.Scheme != "https" && !c.isBase(u) {
		return nil, fmt.Errorf("chartfox: refusing file address %q", fileURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chartfox: file answered %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxFile+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxFile {
		return nil, errors.New("chartfox: file too big")
	}
	return b, nil
}

// isBase reports whether u is the API's own host (lets tests use a plain-http server).
func (c *Client) isBase(u *url.URL) bool {
	b, err := url.Parse(c.base)
	return err == nil && b.Host == u.Host
}

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	token, err := c.tokens.Token(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	default:
		return fmt.Errorf("chartfox: %s answered %d", strings.SplitN(path, "?", 2)[0], resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out)
}
