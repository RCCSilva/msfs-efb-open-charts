package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

const (
	testToken = "tok-0123456789abcdef"
	lanHost   = "192.168.0.10:43210"
	lanDevice = "192.168.0.50"
)

func newAccessServer(t *testing.T, lan bool) (*Server, http.Handler) {
	t.Helper()
	lib, err := store.Open(t.TempDir())
	require.NoError(t, err)
	s := New(lib, nil, sim.Provider{}, "127.0.0.1:43210")
	s.UseSettings(t.TempDir(), "", settings.Settings{Port: 43210, LAN: lan, Token: testToken}, false)
	return s, s.Handler()
}

type reqOpt struct {
	from, host, method, path, origin, cookie string
}

func do(h http.Handler, o reqOpt) *httptest.ResponseRecorder {
	if o.method == "" {
		o.method = "GET"
	}
	if o.path == "" {
		o.path = "/api/health"
	}
	req := httptest.NewRequest(o.method, o.path, strings.NewReader(""))
	req.RemoteAddr = net.JoinHostPort(o.from, "50000")
	req.Host = o.host
	if o.origin != "" {
		req.Header.Set("Origin", o.origin)
	}
	if o.cookie != "" {
		req.Header.Set("Cookie", keyCookie+"="+o.cookie)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestThisComputerIsAlwaysAllowed(t *testing.T) {
	cases := map[string]reqOpt{
		"loopback, own host":  {from: "127.0.0.1", host: "127.0.0.1:43210"},
		"loopback, localhost": {from: "127.0.0.1", host: "localhost:43210"},
		"IPv6 loopback":       {from: "::1", host: "[::1]:43210"},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			_, h := newAccessServer(t, false)

			// Act
			r := do(h, o)

			// Assert
			assert.Equal(t, 200, r.Code)
		})
	}
}

func TestThisComputerIsRefusedWhenTheRequestIsNotForIt(t *testing.T) {
	cases := map[string]reqOpt{
		// DNS rebinding: a name that is not ours pointing at 127.0.0.1
		"a foreign host": {from: "127.0.0.1", host: "evil.example:43210"},
		"the wrong port": {from: "127.0.0.1", host: "127.0.0.1:8080"},
		// a web page in the browser of the user must not be able to drive the API
		"a foreign origin": {from: "127.0.0.1", host: "127.0.0.1:43210", method: "POST", path: "/api/groups", origin: "https://evil.example"},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			_, h := newAccessServer(t, false)

			// Act
			r := do(h, o)

			// Assert
			assert.Equal(t, 403, r.Code)
		})
	}
}

func TestOtherDevicesAreRefusedWhenNetworkAccessIsOff(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, false)

	// Act
	r := do(h, reqOpt{from: lanDevice, host: lanHost, cookie: testToken})

	// Assert
	assert.Equal(t, 403, r.Code, "even with the key")
}

func TestOtherDevicesNeedTheKey(t *testing.T) {
	cases := map[string]reqOpt{
		"no key":            {from: lanDevice, host: lanHost},
		"a wrong cookie":    {from: lanDevice, host: lanHost, cookie: "nope"},
		"a wrong query key": {from: lanDevice, host: lanHost, path: "/manager/?key=wrong"},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			_, h := newAccessServer(t, true)

			// Act
			r := do(h, o)

			// Assert
			assert.Equal(t, 401, r.Code)
		})
	}
}

func TestTheQRLinkSetsTheCookieAndDropsTheKeyFromTheURL(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, true)

	// Act
	r := do(h, reqOpt{from: lanDevice, host: lanHost, path: "/manager/?key=" + testToken + "&lang=pt"})

	// Assert
	require.Equal(t, http.StatusSeeOther, r.Code)
	loc := r.Header().Get("Location")
	assert.NotContains(t, loc, "key=")
	assert.Contains(t, loc, "lang=pt")
	assert.Contains(t, r.Header().Get("Set-Cookie"), keyCookie+"="+testToken)
}

func TestOtherDevicesWithTheKeyAreServed(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, true)

	// Act
	r := do(h, reqOpt{from: lanDevice, host: lanHost, cookie: testToken})

	// Assert
	assert.Equal(t, 200, r.Code)
}

func TestOtherDevicesAreRefusedOutsideThePrivateAddress(t *testing.T) {
	cases := map[string]reqOpt{
		"a public address": {from: "8.8.8.8", host: lanHost, cookie: testToken},
		// DNS rebinding: devices use the IP
		"a host name": {from: lanDevice, host: "evil.example:43210", cookie: testToken},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			_, h := newAccessServer(t, true)

			// Act
			r := do(h, o)

			// Assert
			assert.Equal(t, 403, r.Code)
		})
	}
}

func TestOtherDevicesNeverReachSystemPages(t *testing.T) {
	for _, p := range []string{
		"/api/setup", "/api/setup/efb", "/api/settings", "/api/lan/token", "/api/lan/qr.svg?ip=192.168.0.10",
		"/api/sources/decea-br/package?path=C:/x.zip", "/api/sources/decea-br/package/import",
	} {
		t.Run(p, func(t *testing.T) {
			// Arrange
			_, h := newAccessServer(t, true)

			// Act
			r := do(h, reqOpt{from: lanDevice, host: lanHost, cookie: testToken, path: p})

			// Assert
			assert.Equal(t, 403, r.Code)
		})
	}
}

func TestSystemPagesWorkOnThisComputer(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, true)

	// Act
	r := do(h, reqOpt{from: "127.0.0.1", host: "127.0.0.1:43210", path: "/api/settings"})

	// Assert
	assert.Equal(t, 200, r.Code)
}

func TestOtherDevicesMayUseTheNormalPages(t *testing.T) {
	for _, p := range []string{"/api/charts", "/viewer.html"} {
		t.Run(p, func(t *testing.T) {
			// Arrange
			_, h := newAccessServer(t, true)

			// Act
			r := do(h, reqOpt{from: lanDevice, host: lanHost, cookie: testToken, path: p})

			// Assert
			assert.Equal(t, 200, r.Code)
		})
	}
}

func TestAWriteFromAPageOfAnotherSiteIsRefused(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, true)

	// Act
	r := do(h, reqOpt{from: lanDevice, host: lanHost, cookie: testToken, method: "PUT", path: "/api/groups/current", origin: "http://evil.example"})

	// Assert
	assert.Equal(t, 403, r.Code)
}

func TestAWriteFromOurOwnPageIsAccepted(t *testing.T) {
	// Arrange
	_, h := newAccessServer(t, true)

	// Act
	r := do(h, reqOpt{from: lanDevice, host: lanHost, cookie: testToken, method: "PUT", path: "/api/groups/current", origin: "http://" + lanHost})

	// Assert
	assert.NotContains(t, []int{401, 403}, r.Code)
}

func TestHealthTellsWhoIsAsking(t *testing.T) {
	cases := map[string]struct {
		o    reqOpt
		want []string
	}{
		"this computer":  {reqOpt{from: "127.0.0.1", host: "127.0.0.1:43210"}, []string{`"local":true`, `"app":"open-charts"`}},
		"another device": {reqOpt{from: lanDevice, host: lanHost, cookie: testToken}, []string{`"local":false`}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			_, h := newAccessServer(t, true)

			// Act
			body := do(h, c.o).Body.String()

			// Assert
			for _, w := range c.want {
				assert.Contains(t, body, w)
			}
		})
	}
}

func answers(addr string) bool {
	c := http.Client{Timeout: 700 * time.Millisecond}
	resp, err := c.Get("http://" + addr + "/api/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// listening starts the server on a free address and stops it with the test.
func listening(t *testing.T) (*Server, string) {
	t.Helper()
	s, _ := newAccessServer(t, false)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s.Serve(ln)
	t.Cleanup(func() { s.Shutdown(nilCtx()) })
	return s, ln.Addr().String()
}

// Changing the port opens the new one and releases the old one, in the same process.
func TestRebindMovesThePort(t *testing.T) {
	// Arrange
	s, first := listening(t)
	require.True(t, answers(first), "the server does not answer on its first address")
	second := freeAddr(t)

	// Act
	err := s.Rebind(second)

	// Assert
	require.NoError(t, err)
	assert.True(t, answers(second), "the server does not answer on the new address")
	time.Sleep(150 * time.Millisecond)
	assert.False(t, answers(first), "the old address must be released")
}

func TestRebindOntoABusyPortLeavesTheServerWhereItWas(t *testing.T) {
	// Arrange
	s, addr := listening(t)
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer busy.Close()

	// Act
	err = s.Rebind(busy.Addr().String())

	// Assert
	assert.Error(t, err)
	assert.True(t, answers(addr), "after a failed rebind the server must keep answering where it was")
}

func TestQRCodeIsDrawn(t *testing.T) {
	// Arrange
	link := "http://192.168.0.10:43210/manager/?key=abc"

	// Act
	svg, err := qrSVG(link)

	// Assert
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(svg, "<svg"))
	assert.Contains(t, svg, "<path")
	assert.True(t, strings.HasSuffix(svg, "</svg>"))
}
