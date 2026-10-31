package chartfox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"
)

// loginTimeout is how long a login waits for the user at the ChartFox page.
const loginTimeout = 5 * time.Minute

// Status is what the app shows about the ChartFox account.
type Status struct {
	Available  bool   `json:"available"`  // this build knows its client id
	Connected  bool   `json:"connected"`  // a token is stored
	Connecting bool   `json:"connecting"` // a login is waiting for the user
	User       string `json:"user,omitempty"`
	Error      string `json:"error,omitempty"` // why the last login failed
}

// Connector is what the server needs to link and unlink the account.
type Connector interface {
	Status() Status
	Connect() error
	Disconnect() error
}

// Account is the user's ChartFox account in the app: it runs the login and keeps the connection.
type Account struct {
	cfg  Config
	http Doer
	conn *Connection
	open func(string) error

	mu         sync.Mutex
	connecting bool
	lastErr    string
}

// NewAccount builds the account. open shows the authorization page to the user (their browser).
func NewAccount(cfg Config, h Doer, store TokenStore, open func(string) error) *Account {
	return &Account{cfg: cfg, http: h, conn: NewConnection(cfg, h, store), open: open}
}

// Client is a ChartFox client acting as the connected user.
func (a *Account) Client() *Client { return NewClient(BaseURL, a.http, a.conn) }

// Status reports the account.
func (a *Account) Status() Status {
	a.mu.Lock()
	defer a.mu.Unlock()
	return Status{Available: a.cfg.Available(), Connected: a.conn.Connected(), Connecting: a.connecting,
		User: a.conn.User(), Error: a.lastErr}
}

// Connect starts a login in the background; the page the user must open is shown through open. Calling it while a login
// is waiting does nothing.
func (a *Account) Connect() error {
	if !a.cfg.Available() {
		return errors.New("this version of Open Charts cannot connect to ChartFox")
	}
	redirect, err := url.Parse(a.cfg.Redirect)
	if err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.connecting {
		return nil
	}
	ln, err := net.Listen("tcp", redirect.Host)
	if err != nil {
		return fmt.Errorf("port %s is busy, close what uses it and try again", redirect.Port())
	}
	a.connecting, a.lastErr = true, ""
	go a.login(ln)
	return nil
}

func (a *Account) login(ln net.Listener) {
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
	defer cancel()
	tok, err := Login(ctx, a.cfg, a.http, ln, a.open)
	if err == nil {
		// the name is only to show; a failure here does not undo the login
		tok.User, _ = NewClient(BaseURL, a.http, staticToken(tok.Access)).User(ctx)
		err = a.conn.Set(tok)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.connecting = false
	switch {
	case err == nil || errors.Is(err, ErrDenied):
		a.lastErr = ""
	case errors.Is(err, context.DeadlineExceeded):
		a.lastErr = "the login was not finished in time"
	default:
		a.lastErr = err.Error()
	}
}

// Disconnect forgets the token. The charts already downloaded are the app's to remove (see the policy in AGENTS.md).
func (a *Account) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastErr = ""
	return a.conn.Disconnect()
}

type staticToken string

func (f staticToken) Token(context.Context) (string, error) { return string(f), nil }
