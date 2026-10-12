package server

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

const testToken = "tok-0123456789abcdef"

func newAccessServer(t *testing.T, lan bool) (*Server, http.Handler) {
	t.Helper()
	lib, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s := New(lib, nil, func() Position { return Position{} }, "127.0.0.1:43210")
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
	_, h := newAccessServer(t, false)
	if r := do(h, reqOpt{from: "127.0.0.1", host: "127.0.0.1:43210"}); r.Code != 200 {
		t.Fatalf("loopback, own host: %d", r.Code)
	}
	if r := do(h, reqOpt{from: "127.0.0.1", host: "localhost:43210"}); r.Code != 200 {
		t.Fatalf("loopback, localhost: %d", r.Code)
	}
	// DNS rebinding: a name that is not ours pointing at 127.0.0.1
	if r := do(h, reqOpt{from: "127.0.0.1", host: "evil.example:43210"}); r.Code != 403 {
		t.Fatalf("foreign host from loopback: %d, want 403", r.Code)
	}
	// wrong port
	if r := do(h, reqOpt{from: "127.0.0.1", host: "127.0.0.1:8080"}); r.Code != 403 {
		t.Fatalf("wrong port: %d, want 403", r.Code)
	}
	// a web page in the user's browser must not be able to drive the API
	if r := do(h, reqOpt{from: "127.0.0.1", host: "127.0.0.1:43210", method: "POST", path: "/api/groups", origin: "https://evil.example"}); r.Code != 403 {
		t.Fatalf("foreign origin: %d, want 403", r.Code)
	}
	if r := do(h, reqOpt{from: "::1", host: "[::1]:43210"}); r.Code != 200 {
		t.Fatalf("IPv6 loopback: %d", r.Code)
	}
}

func TestOtherDevicesAreRefusedWhenNetworkAccessIsOff(t *testing.T) {
	_, h := newAccessServer(t, false)
	r := do(h, reqOpt{from: "192.168.0.50", host: "192.168.0.10:43210", cookie: testToken})
	if r.Code != 403 {
		t.Fatalf("LAN off: %d, want 403 even with the key", r.Code)
	}
}

func TestOtherDevicesNeedThePrivateAddressAndTheKey(t *testing.T) {
	_, h := newAccessServer(t, true)
	host := "192.168.0.10:43210"

	// no key -> 401
	if r := do(h, reqOpt{from: "192.168.0.50", host: host}); r.Code != 401 {
		t.Fatalf("no key: %d, want 401", r.Code)
	}
	// wrong key -> 401
	if r := do(h, reqOpt{from: "192.168.0.50", host: host, cookie: "nope"}); r.Code != 401 {
		t.Fatalf("wrong cookie: %d, want 401", r.Code)
	}
	// the QR link: key in the query -> cookie set and the key removed from the URL
	r := do(h, reqOpt{from: "192.168.0.50", host: host, path: "/manager/?key=" + testToken + "&lang=pt"})
	if r.Code != http.StatusSeeOther {
		t.Fatalf("QR link: %d, want 303", r.Code)
	}
	if loc := r.Header().Get("Location"); strings.Contains(loc, "key=") || !strings.Contains(loc, "lang=pt") {
		t.Fatalf("redirect must drop the key and keep the rest: %q", loc)
	}
	if !strings.Contains(r.Header().Get("Set-Cookie"), keyCookie+"="+testToken) {
		t.Fatalf("cookie not set: %q", r.Header().Get("Set-Cookie"))
	}
	// with the cookie -> allowed
	if r := do(h, reqOpt{from: "192.168.0.50", host: host, cookie: testToken}); r.Code != 200 {
		t.Fatalf("with key: %d, want 200", r.Code)
	}
	// wrong key in the query -> 401
	if r := do(h, reqOpt{from: "192.168.0.50", host: host, path: "/manager/?key=wrong"}); r.Code != 401 {
		t.Fatalf("wrong query key: %d, want 401", r.Code)
	}
	// a public address is never served, key or not
	if r := do(h, reqOpt{from: "8.8.8.8", host: host, cookie: testToken}); r.Code != 403 {
		t.Fatalf("public address: %d, want 403", r.Code)
	}
	// a host name (DNS rebinding) is refused: devices use the IP
	if r := do(h, reqOpt{from: "192.168.0.50", host: "evil.example:43210", cookie: testToken}); r.Code != 403 {
		t.Fatalf("host name: %d, want 403", r.Code)
	}
}

func TestOtherDevicesNeverReachSystemPages(t *testing.T) {
	_, h := newAccessServer(t, true)
	host := "192.168.0.10:43210"
	for _, p := range []string{
		"/api/setup", "/api/setup/efb", "/api/settings", "/api/lan/token", "/api/lan/qr.svg?ip=192.168.0.10",
		"/api/sources/decea-br/package?path=C:/x.zip", "/api/sources/decea-br/package/import",
	} {
		if r := do(h, reqOpt{from: "192.168.0.50", host: host, cookie: testToken, path: p}); r.Code != 403 {
			t.Errorf("%s from another device: %d, want 403", p, r.Code)
		}
	}
	// and the same pages work on this computer
	if r := do(h, reqOpt{from: "127.0.0.1", host: "127.0.0.1:43210", path: "/api/settings"}); r.Code != 200 {
		t.Errorf("/api/settings on this computer: %d, want 200", r.Code)
	}
}

func TestOtherDevicesMayUseTheNormalApi(t *testing.T) {
	_, h := newAccessServer(t, true)
	host := "192.168.0.10:43210"
	if r := do(h, reqOpt{from: "192.168.0.50", host: host, cookie: testToken, path: "/api/charts"}); r.Code != 200 {
		t.Fatalf("/api/charts: %d", r.Code)
	}
	if r := do(h, reqOpt{from: "192.168.0.50", host: host, cookie: testToken, path: "/viewer.html"}); r.Code != 200 {
		t.Fatalf("/viewer.html: %d", r.Code)
	}
	// a write from a page of another site is refused; from our own page it is accepted
	bad := do(h, reqOpt{from: "192.168.0.50", host: host, cookie: testToken, method: "PUT", path: "/api/groups/current", origin: "http://evil.example"})
	if bad.Code != 403 {
		t.Fatalf("foreign origin write: %d, want 403", bad.Code)
	}
	ok := do(h, reqOpt{from: "192.168.0.50", host: host, cookie: testToken, method: "PUT", path: "/api/groups/current", origin: "http://" + host})
	if ok.Code == 403 || ok.Code == 401 {
		t.Fatalf("same-origin write blocked: %d", ok.Code)
	}
}

func TestHealthTellsWhoIsAsking(t *testing.T) {
	_, h := newAccessServer(t, true)
	if b := do(h, reqOpt{from: "127.0.0.1", host: "127.0.0.1:43210"}).Body.String(); !strings.Contains(b, `"local":true`) || !strings.Contains(b, `"app":"open-charts"`) {
		t.Fatalf("local health: %s", b)
	}
	if b := do(h, reqOpt{from: "192.168.0.50", host: "192.168.0.10:43210", cookie: testToken}).Body.String(); !strings.Contains(b, `"local":false`) {
		t.Fatalf("remote health: %s", b)
	}
}

// Changing the port opens the new one and releases the old one, in the same process.
func TestRebindMovesThePort(t *testing.T) {
	s, _ := newAccessServer(t, false)
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s.Serve(ln1)
	defer s.Shutdown(nilCtx())
	addr1 := ln1.Addr().String()

	probe := func(addr string) bool {
		c := http.Client{Timeout: 700 * time.Millisecond}
		resp, err := c.Get("http://" + addr + "/api/health")
		if err != nil {
			return false
		}
		resp.Body.Close()
		return resp.StatusCode == 200
	}
	if !probe(addr1) {
		t.Fatal("server does not answer on its first address")
	}

	// find a free port for the move
	tmp, _ := net.Listen("tcp", "127.0.0.1:0")
	addr2 := tmp.Addr().String()
	tmp.Close()
	if err := s.Rebind(addr2); err != nil {
		t.Fatal(err)
	}
	if !probe(addr2) {
		t.Fatal("server does not answer on the new address")
	}
	time.Sleep(150 * time.Millisecond)
	if probe(addr1) {
		t.Fatal("the old address must be released")
	}

	// a port that is taken leaves the server where it was
	busy, _ := net.Listen("tcp", "127.0.0.1:0")
	defer busy.Close()
	if err := s.Rebind(busy.Addr().String()); err == nil {
		t.Fatal("rebind onto a busy port must fail")
	}
	if !probe(addr2) {
		t.Fatal("after a failed rebind the server must keep answering where it was")
	}
}

func TestQRCodeIsDrawn(t *testing.T) {
	svg, err := qrSVG("http://192.168.0.10:43210/manager/?key=abc")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(svg, "<svg") || !strings.Contains(svg, "<path") || !strings.HasSuffix(svg, "</svg>") {
		t.Fatalf("not an svg: %.80s", svg)
	}
}
