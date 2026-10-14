// opencharts: local chart manager and in-sim viewer server.
//
//	opencharts [-data DIR] [-addr 127.0.0.1:PORT] [-fixed -lat N -lon N -hdg N]
//
// Serves the viewer for the MSFS EFB iframe, the manager UI, the chart API and the library.
// The aircraft position comes from SimConnect; -fixed (with -lat/-lon/-hdg) uses a fixed test
// position instead. The port and the access from other devices of the local network are options
// of the manager (settings.json); -addr overrides them for development.
// "opencharts setup ..." is used by the installer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/server"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/setup"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/simconnect"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/tray"
	"github.com/RCCSilva/msfs-efb-open-charts/core/web"
)

func defaultDataDir() string {
	if d, err := os.UserCacheDir(); err == nil { // %LOCALAPPDATA% on Windows
		return filepath.Join(d, "OpenCharts")
	}
	return "opencharts-data"
}

// portOfInstalledEFB returns the port the installed EFB app is configured for, or 0.
func portOfInstalledEFB() int {
	exe, err := os.Executable()
	if err != nil {
		return 0
	}
	for _, in := range setup.Detect(exe) {
		raw := setup.InstalledServerURL(in.Community)
		if raw == "" {
			continue
		}
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		if p, err := strconv.Atoi(u.Port()); err == nil && settings.ValidatePort(p) == nil {
			return p
		}
	}
	return 0
}

// alreadyRunning asks whatever listens on the port whether it is Open Charts.
func alreadyRunning(port string) bool {
	c := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := c.Get("http://127.0.0.1:" + port + "/api/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var h struct {
		App string `json:"app"`
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&h) == nil && h.App == "open-charts"
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		os.Exit(runSetupCommand(os.Args[2:]))
	}
	data := flag.String("data", defaultDataDir(), "data directory (library, groups, tile cache, settings)")
	addrFlag := flag.String("addr", "", "listen address, overriding the settings (loopback only), for example 127.0.0.1:8080")
	lat := flag.Float64("lat", -23.5505, "fixed aircraft latitude")
	lon := flag.Float64("lon", -46.6333, "fixed aircraft longitude")
	hdg := flag.Float64("hdg", 45, "fixed aircraft heading (true)")
	fixed := flag.Bool("fixed", false, "use the fixed position above instead of SimConnect")
	dllPath := flag.String("simconnect-dll", "", "path to SimConnect.dll (default: search the MSFS SDK folders)")
	webDir := flag.String("web", "", "serve the UI from this folder (with sim/ and manager/) instead of the embedded copy")
	noTray := flag.Bool("no-tray", false, "do not show the notification-area icon")
	openAtStart := flag.Bool("open", false, "open the manager in the browser at start")
	noImport := flag.Bool("no-import", false, "do not start the PDF importer (faster start)")
	flag.Parse()

	// Log to a file too: with the tray there is usually no console window.
	_ = os.MkdirAll(*data, 0o755)
	if lf, err := os.OpenFile(filepath.Join(*data, "opencharts.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		log.SetOutput(io.MultiWriter(lf, os.Stderr))
		defer lf.Close()
	}

	if *webDir != "" {
		web.SetDir(*webDir)
		log.Printf("serving the UI from %s", *webDir)
	}

	lib, err := store.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("library: %s (%d charts)", *data, lib.Count())

	// The PDF renderer is heavy (~300 MB): it is created on the first import and released
	// when idle (see internal/server/renderer.go).
	var factory server.RendererFactory
	if !*noImport {
		factory = func() (*importer.Renderer, error) {
			t0 := time.Now()
			r, err := importer.NewRenderer()
			if err == nil {
				log.Printf("importer ready in %v", time.Since(t0).Round(time.Millisecond))
			}
			return r, err
		}
	}

	fixedPos := func() server.Position {
		return server.Position{Valid: true, Connected: true, Lat: *lat, Lon: *lon, HeadingTrue: *hdg,
			Source: "fixed", UpdatedAt: time.Now()}
	}
	pos := fixedPos
	var simDLL string // the SimConnect library, when there is a simulator link at all
	if !*fixed {
		dll, err := simconnect.FindDLL(*dllPath, *data)
		if err != nil {
			log.Printf("simconnect: %v; using the fixed position", err)
		} else {
			simDLL = dll
			sc := simconnect.New(context.Background(), dll, log.Printf)
			pos = func() server.Position {
				st, detail := sc.Status()
				p := server.Position{Source: "simconnect", Connected: st == simconnect.StateOpen || st == simconnect.StateData, Detail: detail}
				if s, ok := sc.Latest(); ok && time.Since(s.At) < 5*time.Second {
					p.Valid, p.Lat, p.Lon, p.HeadingTrue = true, s.Lat, s.Lon, s.HeadingTrue
					p.GroundSpeed, p.AltitudeFt, p.UpdatedAt = s.GroundSpeed, s.AltitudeFt, s.At
				}
				return p
			}
		}
	}

	// ---- where to listen: the settings, unless -addr says otherwise
	cfg := settings.Load(*data)
	if _, err := os.Stat(filepath.Join(*data, settings.FileName)); err != nil {
		// First run with settings. Someone who installed an earlier version has the EFB app
		// pointing at its port (8080): keep that port, so nothing breaks until they choose to move.
		if p := portOfInstalledEFB(); p != 0 {
			log.Printf("keeping port %d, the one the installed EFB app uses", p)
			cfg.Port = p
		}
		_ = cfg.Save(*data) // also keeps the generated key
	}
	fixedAddr := *addrFlag != ""
	listenAddr := cfg.ListenAddr()
	if fixedAddr {
		host, _, err := net.SplitHostPort(*addrFlag)
		if err != nil || (host != "127.0.0.1" && host != "localhost" && host != "::1") {
			log.Fatalf("refusing to listen on %q with -addr: use a loopback address (the network option is in the manager)", *addrFlag)
		}
		listenAddr = *addrFlag
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		_, port, _ := net.SplitHostPort(listenAddr)
		if alreadyRunning(port) {
			log.Printf("Open Charts is already running on port %s: opening its manager", port)
			tray.Open("http://127.0.0.1:" + port + "/")
			return
		}
		if fixedAddr {
			log.Fatalf("cannot listen on %s: %v", listenAddr, err)
		}
		// Another program holds our port: take the next free one and remember it. The EFB app
		// is told the new address from the manager (Setup).
		ln = nil
		for p := cfg.Port + 1; p < cfg.Port+50 && p <= 65535; p++ {
			c2 := cfg
			c2.Port = p
			if l, e := net.Listen("tcp", c2.ListenAddr()); e == nil {
				log.Printf("port %d is in use by another program: using %d", cfg.Port, p)
				ln, cfg = l, c2
				_ = cfg.Save(*data)
				break
			}
		}
		if ln == nil {
			log.Fatalf("cannot listen on %s: %v", listenAddr, err)
		}
	}

	srv := server.New(lib, factory, pos, ln.Addr().String())
	srv.UseSettings(*data, defaultDataDir(), cfg, fixedAddr)

	// What the simulator knows about airports and procedures: asked on demand, kept in SQLite.
	if nav, err := navdb.Open(filepath.Join(*data, "navdata.db")); err != nil {
		log.Printf("navigation database: %v (airport data will be unavailable)", err)
	} else {
		defer nav.Close()
		var fetch server.NavFetcher
		var fixes server.FixFetcher
		var lookup server.IdentFetcher
		if simDLL != "" {
			fetch = func(icao string) (*navdata.Airport, error) {
				return simconnect.FetchAirport(simDLL, icao, 30*time.Second)
			}
			fixes = func(refs []navdata.FixRef) ([]navdata.Fix, error) {
				return simconnect.FetchFixes(simDLL, refs, 60*time.Second)
			}
			lookup = func(idents []string) ([]navdata.Fix, error) {
				return simconnect.LookupFixes(simDLL, idents, 30*time.Second)
			}
		}
		srv.SetNav(nav, fetch, fixes, lookup)
		if simDLL != "" {
			srv.SetAirportLister(func() ([]server.AirportRef, error) {
				list, err := simconnect.ListAirports(simDLL, 90*time.Second)
				out := make([]server.AirportRef, 0, len(list))
				for _, a := range list {
					out = append(out, server.AirportRef{ICAO: a.ICAO, Region: a.Region, Lat: a.Lat, Lon: a.Lon})
				}
				return out, err
			})
		}
	}
	defer srv.Close()
	srv.Serve(ln)
	log.Printf("listening on %s (network access: %v)", ln.Addr(), cfg.LAN && !fixedAddr)
	if *openAtStart {
		tray.Open(srv.LocalURL())
	}

	if *noTray {
		select {} // run until the process is stopped
	}
	status := func() string {
		p := pos()
		switch {
		case p.Source == "fixed":
			return "Fixed position (no simulator)"
		case p.Valid:
			return "MSFS: receiving position"
		case p.Connected:
			return "MSFS: connected"
		}
		return "MSFS: waiting for the simulator"
	}
	tray.Run(tray.Config{
		URL:    srv.LocalURL,
		MapURL: func() string { return srv.LocalURL() + "viewer.html" },
		Status: status,
		Quit: func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = srv.Shutdown(ctx)
		},
	})
	log.Printf("bye")
}
