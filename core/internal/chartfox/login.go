package chartfox

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"net/url"
)

// ErrDenied is returned when the user refused the connection at ChartFox.
var ErrDenied = errors.New("chartfox: authorization denied")

// Login runs the authorization code flow with PKCE: it hands the authorization page to open (which shows it to the
// user), waits on ln for ChartFox to send the browser back to the redirect address and trades the code for a token.
// It gives up when ctx ends. ln is the listener of the redirect address (see Config.Redirect), so a test can pass its own.
func Login(ctx context.Context, cfg Config, h Doer, ln net.Listener, open func(authURL string) error) (Token, error) {
	redirect, err := url.Parse(cfg.Redirect)
	if err != nil {
		return Token{}, err
	}
	verifier, challenge := PKCE()
	state := randomString(24)

	type result struct {
		code string
		err  error
	}
	done := make(chan result, 1)
	send := func(r result) {
		select {
		case done <- r:
		default: // the first answer wins
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc(redirect.Path, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if e := q.Get("error"); e != "" {
			fmt.Fprint(w, closePage("ChartFox was not connected", "You can close this tab and go back to Open Charts."))
			send(result{err: ErrDenied})
			return
		}
		fmt.Fprint(w, closePage("ChartFox connected", "You can close this tab and go back to Open Charts."))
		send(result{code: q.Get("code")})
	})
	srv := &http.Server{Handler: mux}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	if err := open(cfg.AuthorizeURL(state, challenge)); err != nil {
		return Token{}, err
	}
	select {
	case r := <-done:
		if r.err != nil {
			return Token{}, r.err
		}
		if r.code == "" {
			return Token{}, errors.New("chartfox: no authorization code received")
		}
		return cfg.Exchange(ctx, h, r.code, verifier)
	case <-ctx.Done():
		return Token{}, ctx.Err()
	}
}

func closePage(title, text string) string {
	return "<!doctype html><meta charset=utf-8><title>" + html.EscapeString(title) + "</title>" +
		"<body style=\"font-family:system-ui;margin:3em\"><h2>" + html.EscapeString(title) + "</h2><p>" + html.EscapeString(text) + "</p></body>"
}
