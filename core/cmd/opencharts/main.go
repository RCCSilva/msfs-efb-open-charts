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
	"errors"
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

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/chartfox"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/logfile"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/server"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/simconnect"
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
func portOfInstalledEFB(app sim.AppInstaller) int {
	exe, err := os.Executable()
	if err != nil || app == nil {
		return 0
	}
	for _, in := range app.Detect(exe) {
		raw := app.InstalledServerURL(in.Target)
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
	if lf, err := logfile.Open(filepath.Join(*data, "opencharts.log"), logfile.DefaultMaxBytes); err == nil {
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

	// what the simulator gives the program: the position of the aircraft, navigation data...
	provider := sim.Provider{ID: "fixed", Name: "Fixed position", Position: sim.Fixed(*lat, *lon, *hdg)}
	if !*fixed {
		dll, err := simconnect.FindDLL(*dllPath, *data)
		if err != nil {
			log.Printf("simconnect: %v; using the fixed position", err)
		} else {
			provider = msfs2024.New(context.Background(), dll, log.Printf)
		}
	}

	// ---- where to listen: the settings, unless -addr says otherwise
	cfg := settings.Load(*data)
	if _, err := os.Stat(filepath.Join(*data, settings.FileName)); err != nil {
		// First run with settings. Someone who installed an earlier version has the EFB app
		// pointing at its port (8080): keep that port, so nothing breaks until they choose to move.
		if p := portOfInstalledEFB(provider.App); p != 0 {
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

	srv := server.New(lib, factory, provider, ln.Addr().String())
	srv.UseSettings(*data, defaultDataDir(), cfg, fixedAddr)
	srv.UseOpener(tray.Open)

	// The user's ChartFox account: the token stays in the data folder, the authorization page opens in the browser.
	cfAccount := chartfox.NewAccount(chartfox.DefaultConfig(), &http.Client{Timeout: 30 * time.Second},
		chartfox.FileStore{Path: filepath.Join(*data, "chartfox.json")}, func(authURL string) error {
			tray.Open(authURL)
			return nil
		})
	srv.UseChartFox(cfAccount, cfAccount.Client())

	// What the simulator knows about airports and procedures: asked on demand, kept in SQLite.
	navPath := filepath.Join(*data, "navdata.db")
	// The database is opened (its schema brought up to date, its structure checked) here, and again
	// when the user starts over or restores a copy. A file that cannot be used is not fatal: the
	// program runs without navigation data and the manager tells the user why.
	var navCur *navdb.DB
	openNav := func() error {
		nav, err := navdb.Open(navPath)
		if err != nil {
			kind := "failed"
			switch {
			case errors.Is(err, navdb.ErrCorrupt):
				kind = "corrupt"
			case errors.Is(err, navdb.ErrNewerSchema):
				kind = "newer"
			}
			srv.SetNavProblem(kind, err.Error())
			log.Printf("navigation database: %v (it is not in use)", err)
			return err
		}
		navCur = nav
		if m := nav.Migrated(); m.To != m.From {
			log.Printf("navigation database: schema %d -> %d (copy before: %q)", m.From, m.To, m.Backup)
		}
		srv.SetNav(nav)
		srv.SetNavProblem("", "")
		return nil
	}
	_ = openNav()
	srv.SetNavFile(server.NavFile{Path: navPath, Replace: func(change func() error) error {
		srv.SetNav(nil) // nothing uses the file while it is changed
		if navCur != nil {
			navCur.Close()
			navCur = nil
		}
		cerr := change()
		oerr := openNav()
		if cerr != nil {
			return cerr
		}
		return oerr
	}})
	defer func() {
		if navCur != nil {
			navCur.Close()
		}
	}()
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
		p := provider.Position.Position()
		switch {
		case p.Source == "fixed":
			return "Fixed position (no simulator)"
		case p.Valid:
			return provider.Short + ": receiving position"
		case p.Connected:
			return provider.Short + ": connected"
		}
		return provider.Short + ": waiting for the simulator"
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
