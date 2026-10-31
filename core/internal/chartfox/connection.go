package chartfox

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TokenStore keeps the token between runs.
type TokenStore interface {
	Load() (Token, bool)
	Save(Token) error
	Delete() error
}

// FileStore keeps the token in a file of the data folder, readable only by the user.
type FileStore struct{ Path string }

// Load reads the token; false when there is none or it is unreadable.
func (f FileStore) Load() (Token, bool) {
	b, err := os.ReadFile(f.Path)
	if err != nil {
		return Token{}, false
	}
	var t Token
	if json.Unmarshal(b, &t) != nil || t.Access == "" {
		return Token{}, false
	}
	return t, true
}

// Save writes the token.
func (f FileStore) Save(t Token) error {
	b, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(f.Path, b, 0o600)
}

// Delete removes the token.
func (f FileStore) Delete() error {
	if err := os.Remove(f.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Connection is the account the user connected: it keeps the token fresh and is the TokenSource of the Client.
type Connection struct {
	cfg   Config
	http  Doer
	store TokenStore
	now   func() time.Time

	mu sync.Mutex
}

// NewConnection builds a connection that reads and refreshes the token kept in store.
func NewConnection(cfg Config, h Doer, store TokenStore) *Connection {
	return &Connection{cfg: cfg, http: h, store: store, now: time.Now}
}

// Connected says whether a token is stored (it may still need a refresh).
func (c *Connection) Connected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.store.Load()
	return ok
}

// User is the name of the connected account ("" when unknown or not connected).
func (c *Connection) User() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, _ := c.store.Load()
	return t.User
}

// Set stores the token of a new login.
func (c *Connection) Set(t Token) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.store.Save(t)
}

// Disconnect forgets the token.
func (c *Connection) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.store.Delete()
}

// Token gives a usable access token, refreshing it when it is about to expire. When the refresh is refused the stored
// token is dropped and ErrUnauthorized is returned: the user has to connect again.
func (c *Connection) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.store.Load()
	if !ok {
		return "", ErrUnauthorized
	}
	if t.Fresh(c.now()) {
		return t.Access, nil
	}
	if t.Refresh == "" {
		_ = c.store.Delete()
		return "", ErrUnauthorized
	}
	nt, err := c.cfg.Refresh(ctx, c.http, t.Refresh)
	if errors.Is(err, ErrUnauthorized) {
		_ = c.store.Delete()
	}
	if err != nil {
		return "", err
	}
	if err := c.store.Save(nt); err != nil {
		return "", err
	}
	return nt.Access, nil
}
