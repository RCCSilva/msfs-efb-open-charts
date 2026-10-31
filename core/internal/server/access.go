package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"rsc.io/qr"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
)

// Who may talk to the server.
//
//   - This computer (loopback) always can: the manager, the viewer inside the simulator.
//   - Other devices of the local network can only when the user turned that on. They must come
//     from a private address, address the server by IP, and carry the key from the QR code.
//     They never reach the system pages (installing the EFB app, EXE.xml, settings).
//
// The rules live here, in one place, and are covered by access_test.go.

const keyCookie = "oc_key"

// netState is the part of the server that can change while it runs.
type netState struct {
	mu             sync.RWMutex
	cfg            settings.Settings
	dataDir        string
	defaultDataDir string
	fixedAddr      bool // the address was given on the command line: settings do not change it

	httpMu  sync.Mutex
	httpSrv *http.Server
	ln      net.Listener
	handler http.Handler
}

// UseSettings gives the server its persisted settings. fixedAddr means the listen address came
// from the command line and the port/network options of the settings are not applied.
func (s *Server) UseSettings(dataDir, defaultDataDir string, cfg settings.Settings, fixedAddr bool) {
	s.net.mu.Lock()
	defer s.net.mu.Unlock()
	if !settings.ValidAircraft(cfg.Aircraft) {
		cfg.Aircraft = settings.AircraftPlane
	}
	if settings.AircraftHex(cfg.AircraftColor) == "" {
		cfg.AircraftColor = settings.DefaultAircraftColor
	}
	s.net.cfg, s.net.dataDir, s.net.defaultDataDir, s.net.fixedAddr = cfg, dataDir, defaultDataDir, fixedAddr
}

func (s *Server) settingsSnapshot() settings.Settings {
	s.net.mu.RLock()
	defer s.net.mu.RUnlock()
	return s.net.cfg
}

// port is the port the server listens on now, as a string ("" if unknown).
func (s *Server) port() string {
	s.net.mu.RLock()
	defer s.net.mu.RUnlock()
	_, p, err := net.SplitHostPort(s.addr)
	if err != nil {
		return ""
	}
	return p
}

// ------------------------------------------------------------------ listening

// Serve starts serving on ln in the background.
func (s *Server) Serve(ln net.Listener) {
	s.net.httpMu.Lock()
	defer s.net.httpMu.Unlock()
	s.swapLocked(ln)
}

func (s *Server) swapLocked(ln net.Listener) {
	if s.net.handler == nil {
		s.net.handler = s.Handler()
	}
	old := s.net.httpSrv
	srv := &http.Server{Handler: s.net.handler, ReadHeaderTimeout: 10 * time.Second}
	s.net.httpSrv, s.net.ln = srv, ln
	s.net.mu.Lock()
	s.addr = ln.Addr().String()
	s.net.mu.Unlock()
	go func() { _ = srv.Serve(ln) }()
	if old != nil {
		go func() { // requests in flight (the one that asked for the change) finish normally
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = old.Shutdown(ctx)
		}()
	}
}

// Rebind moves the server to another listen address without stopping the program. The old
// address is released first (Windows does not let 0.0.0.0:p and 127.0.0.1:p coexist); if the new
// one cannot be opened, the old one is opened again and the error is returned.
func (s *Server) Rebind(addr string) error {
	s.net.httpMu.Lock()
	defer s.net.httpMu.Unlock()
	s.net.mu.RLock()
	oldAddr := s.addr
	s.net.mu.RUnlock()
	if s.net.ln != nil {
		_ = s.net.ln.Close()
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if back, e2 := net.Listen("tcp", oldAddr); e2 == nil {
			s.swapLocked(back)
		}
		return err
	}
	s.swapLocked(ln)
	return nil
}

// Shutdown stops serving.
func (s *Server) Shutdown(ctx context.Context) error {
	s.net.httpMu.Lock()
	srv := s.net.httpSrv
	s.net.httpMu.Unlock()
	if srv == nil {
		return nil
	}
	return srv.Shutdown(ctx)
}

// ------------------------------------------------------------------ the guard

func remoteIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}

func isPrivateIP(ip net.IP) bool {
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

func hostAndPort(h string) (host, port string) {
	if hh, pp, err := net.SplitHostPort(h); err == nil {
		return strings.ToLower(hh), pp
	}
	return strings.ToLower(strings.Trim(h, "[]")), ""
}

func isLocalName(h string) bool { return h == "127.0.0.1" || h == "localhost" || h == "::1" }

// localOnlyPath: what a device of the network must never reach, even with the key. These change
// the computer (EFB install, EXE.xml, settings) or read arbitrary paths of it (package reader).
func localOnlyPath(p string) bool {
	switch {
	case strings.HasPrefix(p, "/api/setup"), strings.HasPrefix(p, "/api/settings"), strings.HasPrefix(p, "/api/lan/"), strings.HasPrefix(p, "/api/db/"), strings.HasPrefix(p, "/api/chartfox"), p == "/api/library/delete-all":
		return true
	case strings.HasPrefix(p, "/api/sources/") && strings.Contains(p, "/package"):
		return true
	case strings.HasPrefix(p, "/api/sources/") && strings.HasSuffix(p, "/favorite"):
		return true // the tabs of the manager are the computer's own preference
	}
	return false
}

func tokenEqual(a, b string) bool {
	return a != "" && b != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// isLocalRequest reports whether the request comes from this computer.
func isLocalRequest(r *http.Request) bool {
	ip := remoteIP(r)
	return ip != nil && ip.IsLoopback()
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg := s.settingsSnapshot()
		host, hport := hostAndPort(r.Host)
		if port := s.port(); hport != "" && port != "" && hport != port {
			http.Error(w, "forbidden host", http.StatusForbidden)
			return
		}

		// --- this computer
		if isLocalRequest(r) {
			if !isLocalName(host) {
				http.Error(w, "forbidden host", http.StatusForbidden) // DNS rebinding
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				if o := r.Header.Get("Origin"); o != "" &&
					!strings.HasPrefix(o, "http://127.0.0.1") && !strings.HasPrefix(o, "http://localhost") {
					http.Error(w, "forbidden origin", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		// --- another device
		ip := remoteIP(r)
		switch {
		case !cfg.LAN:
			http.Error(w, "access from other devices is off", http.StatusForbidden)
			return
		case !isPrivateIP(ip):
			http.Error(w, "only devices of the local network may connect", http.StatusForbidden)
			return
		case net.ParseIP(host) == nil:
			http.Error(w, "use the IP address of the computer", http.StatusForbidden)
			return
		case localOnlyPath(r.URL.Path):
			http.Error(w, "this page is only available on the computer", http.StatusForbidden)
			return
		}
		if k := r.URL.Query().Get("key"); k != "" {
			if !tokenEqual(k, cfg.Token) {
				unauthorized(w)
				return
			}
			http.SetCookie(w, &http.Cookie{Name: keyCookie, Value: k, Path: "/", HttpOnly: true,
				SameSite: http.SameSiteLaxMode, MaxAge: 365 * 24 * 3600})
			if r.Method == http.MethodGet {
				clean := *r.URL
				q := clean.Query()
				q.Del("key")
				clean.RawQuery = q.Encode()
				http.Redirect(w, r, clean.String(), http.StatusSeeOther)
				return
			}
		} else if c, err := r.Cookie(keyCookie); err != nil || !tokenEqual(c.Value, cfg.Token) {
			unauthorized(w)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && o != "http://"+r.Host {
				http.Error(w, "forbidden origin", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = fmt.Fprint(w, `<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width">
<title>Open Charts</title><body style="font:16px system-ui;margin:2em;max-width:30em">
<h2>Open Charts</h2>
<p>Leia o QR code mostrado no computador (Open Charts &rarr; Configura&ccedil;&atilde;o &rarr; Rede local).</p>
<p>Scan the QR code shown on the computer (Open Charts &rarr; Setup &rarr; Local network).</p></body>`)
}

// ------------------------------------------------------------------ the local network

// LANAddress is an address of this computer that devices of the network can use.
type LANAddress struct {
	IP      string `json:"ip"`
	Name    string `json:"name"`
	Virtual bool   `json:"virtual"` // looks like a VM / VPN / container adapter
	URL     string `json:"url"`
	QR      string `json:"qr"`
}

var virtualNames = []string{"vethernet", "virtualbox", "vmware", "vbox", "hyper-v", "wsl", "docker", "loopback", "bluetooth", "tailscale", "zerotier", "tap-", "vpn"}

func lanAddresses(port string) []LANAddress {
	var out []LANAddress
	ifs, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipn.IP.To4()
			if ip == nil || !ip.IsPrivate() {
				continue
			}
			low := strings.ToLower(ifc.Name)
			virt := false
			for _, v := range virtualNames {
				if strings.Contains(low, v) {
					virt = true
				}
			}
			out = append(out, LANAddress{IP: ip.String(), Name: ifc.Name, Virtual: virt,
				URL: "http://" + ip.String() + ":" + port + "/manager/",
				QR:  "/api/lan/qr.svg?ip=" + url.QueryEscape(ip.String())})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return !out[i].Virtual && out[j].Virtual })
	return out
}

// ------------------------------------------------------------------ handlers

func (s *Server) registerAccessRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", s.handlePutSettings)
	mux.HandleFunc("POST /api/lan/token", s.handleRotateToken)
	mux.HandleFunc("GET /api/lan/qr.svg", s.handleQR)
}

func (s *Server) settingsView() map[string]any {
	cfg := s.settingsSnapshot()
	s.net.mu.RLock()
	fixed := s.net.fixedAddr
	s.net.mu.RUnlock()
	port := s.port()
	addrs := []LANAddress{}
	if cfg.LAN {
		addrs = lanAddresses(port)
	}
	return map[string]any{
		"port": cfg.Port, "lan": cfg.LAN, "defaultPort": settings.DefaultPort, "fixed": fixed, "detail": cfg.Detail, "largeCharts": cfg.LargeCharts, "basemap": cfg.Basemap,
		"aircraft": cfg.Aircraft, "aircraftColor": cfg.AircraftColor, "aircraftColors": settings.AircraftColors,
		"listenAddr": s.listenAddr(), "localURL": "http://127.0.0.1:" + port + "/", "addresses": addrs,
	}
}

func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.settingsView())
}

// handlePutSettings changes the port and/or the network access. It opens the new address first,
// and only then saves: a port that is taken leaves everything as it was.
func (s *Server) handlePutSettings(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Port    *int    `json:"port"`
		LAN     *bool   `json:"lan"`
		Detail  *string `json:"detail"`
		Basemap *string `json:"basemap"`
		// how sharp the big-area charts are drawn
		LargeCharts *string `json:"largeCharts"`
		// the symbol of the aircraft
		Aircraft      *string `json:"aircraft"`
		AircraftColor *string `json:"aircraftColor"`
	}
	if err := readJSON(r, &body); err != nil {
		writeErr(w, 400, err)
		return
	}
	s.net.mu.RLock()
	fixed, dir := s.net.fixedAddr, s.net.dataDir
	s.net.mu.RUnlock()
	// how sharp big-area charts are drawn does not depend on the listen address
	if body.LargeCharts != nil && body.Port == nil && body.LAN == nil && body.Detail == nil && body.Basemap == nil && body.Aircraft == nil && body.AircraftColor == nil {
		if !settings.ValidLargeCharts(*body.LargeCharts) {
			writeErr(w, 400, errors.New("largeCharts must be standard, balanced, sharp or maximum"))
			return
		}
		s.net.mu.Lock()
		s.net.cfg.LargeCharts = *body.LargeCharts
		cfg := s.net.cfg
		s.net.mu.Unlock()
		if err := cfg.Save(dir); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, s.settingsView())
		return
	}
	// the aircraft symbol does not depend on the listen address either
	if (body.Aircraft != nil || body.AircraftColor != nil) && body.Port == nil && body.LAN == nil && body.Detail == nil && body.Basemap == nil {
		if body.Aircraft != nil && !settings.ValidAircraft(*body.Aircraft) {
			writeErr(w, 400, errors.New("aircraft must be plane or helicopter"))
			return
		}
		if body.AircraftColor != nil && settings.AircraftHex(*body.AircraftColor) == "" {
			writeErr(w, 400, errors.New("unknown aircraft colour"))
			return
		}
		s.net.mu.Lock()
		if body.Aircraft != nil {
			s.net.cfg.Aircraft = *body.Aircraft
		}
		if body.AircraftColor != nil {
			s.net.cfg.AircraftColor = *body.AircraftColor
		}
		cfg := s.net.cfg
		s.net.mu.Unlock()
		if err := cfg.Save(dir); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, s.settingsView())
		return
	}
	// the base map does not depend on the listen address
	if body.Basemap != nil && body.Port == nil && body.LAN == nil && body.Detail == nil {
		if !settings.ValidBasemap(*body.Basemap) {
			writeErr(w, 400, errors.New("basemap must be standard, light, dark or none"))
			return
		}
		s.net.mu.Lock()
		s.net.cfg.Basemap = *body.Basemap
		cfg := s.net.cfg
		s.net.mu.Unlock()
		if err := cfg.Save(dir); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, s.settingsView())
		return
	}
	// the chart detail level does not depend on the listen address
	if body.Detail != nil && body.Port == nil && body.LAN == nil {
		if *body.Detail != settings.DetailNormal && *body.Detail != settings.DetailHigh {
			writeErr(w, 400, errors.New("detail must be normal or high"))
			return
		}
		s.net.mu.Lock()
		s.net.cfg.Detail = *body.Detail
		cfg := s.net.cfg
		s.net.mu.Unlock()
		if err := cfg.Save(dir); err != nil {
			writeErr(w, 500, err)
			return
		}
		writeJSON(w, 200, s.settingsView())
		return
	}
	if fixed {
		writeErr(w, 409, errors.New("the address was set with -addr; remove it to use these options"))
		return
	}
	cur := s.settingsSnapshot()
	next := cur
	if body.Port != nil {
		if err := settings.ValidatePort(*body.Port); err != nil {
			writeErr(w, 400, err)
			return
		}
		next.Port = *body.Port
	}
	if body.LAN != nil {
		next.LAN = *body.LAN
	}
	if next.Port != cur.Port || next.LAN != cur.LAN {
		if err := s.Rebind(next.ListenAddr()); err != nil {
			writeErr(w, 409, fmt.Errorf("could not listen on port %d: %v", next.Port, err))
			return
		}
		s.net.mu.Lock()
		s.net.cfg = next
		s.net.mu.Unlock()
		if err := next.Save(dir); err != nil {
			writeErr(w, 500, err)
			return
		}
	}
	writeJSON(w, 200, s.settingsView())
}

// handleRotateToken makes a new key: devices that had the old one must scan the QR code again.
func (s *Server) handleRotateToken(w http.ResponseWriter, r *http.Request) {
	s.net.mu.Lock()
	s.net.cfg.Token = settings.NewToken()
	cfg, dir := s.net.cfg, s.net.dataDir
	s.net.mu.Unlock()
	if err := cfg.Save(dir); err != nil {
		writeErr(w, 500, err)
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

// handleQR draws the QR code of the address (with the key) for one of this computer's addresses.
func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	cfg := s.settingsSnapshot()
	if !cfg.LAN {
		writeErr(w, 404, errors.New("network access is off"))
		return
	}
	ip := r.URL.Query().Get("ip")
	found := false
	for _, a := range lanAddresses(s.port()) {
		if a.IP == ip {
			found = true
		}
	}
	if !found {
		writeErr(w, 404, errors.New("unknown address"))
		return
	}
	target := "http://" + ip + ":" + s.port() + "/manager/?key=" + url.QueryEscape(cfg.Token)
	svg, err := qrSVG(target)
	if err != nil {
		writeErr(w, 500, err)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(svg))
}

// qrSVG renders text as a QR code (error correction M) with a 4-module quiet zone.
func qrSVG(text string) (string, error) {
	code, err := qr.Encode(text, qr.M)
	if err != nil {
		return "", err
	}
	const quiet = 4
	n := code.Size
	var b strings.Builder
	total := n + 2*quiet
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, total, total)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, total, total)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if code.Black(x, y) {
				b.WriteString("M" + strconv.Itoa(x+quiet) + " " + strconv.Itoa(y+quiet) + "h1v1h-1z")
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String(), nil
}

// listenAddr is the address the server listens on now.
func (s *Server) listenAddr() string {
	s.net.mu.RLock()
	defer s.net.mu.RUnlock()
	return s.addr
}

// LocalURL is the address of the manager on this computer.
func (s *Server) LocalURL() string { return "http://127.0.0.1:" + s.port() + "/" }

// nilCtx is a background context for shutting down in tests and at exit.
func nilCtx() context.Context { return context.Background() }
