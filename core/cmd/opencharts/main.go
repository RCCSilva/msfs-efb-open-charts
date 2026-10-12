// opencharts: local chart manager and in-sim viewer server.
//
//	opencharts [-data DIR] [-addr 127.0.0.1:8080] [-lat N -lon N -hdg N]
//
// Serves the viewer for the MSFS EFB iframe, the manager UI, the chart API and the library.
// The aircraft position comes from SimConnect; -fixed (with -lat/-lon/-hdg) uses a fixed test
// position instead. "opencharts setup ..." is used by the installer.
package main

import (
	"context"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/importer"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/server"
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "setup" {
		os.Exit(runSetupCommand(os.Args[2:]))
	}
	data := flag.String("data", defaultDataDir(), "data directory (library, flights, tile cache)")
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (keep it on 127.0.0.1)")
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

	host, _, err := net.SplitHostPort(*addr)
	if err != nil || (host != "127.0.0.1" && host != "localhost" && host != "::1") {
		log.Fatalf("refusing to listen on %q: use a loopback address", *addr)
	}

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
	if !*fixed {
		dll, err := simconnect.FindDLL(*dllPath, *data)
		if err != nil {
			log.Printf("simconnect: %v; using the fixed position", err)
		} else {
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

	managerURL := "http://" + *addr + "/"
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		// Most likely Open Charts is already running: just bring up its manager.
		log.Printf("cannot listen on %s (%v): already running? opening %s", *addr, err, managerURL)
		tray.Open(managerURL)
		return
	}
	srv := server.New(lib, factory, pos, *addr)
	defer srv.Close()
	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	log.Printf("listening on %s", managerURL)
	if *openAtStart {
		tray.Open(managerURL)
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
	tray.Run(tray.Config{URL: managerURL, MapURL: managerURL + "viewer.html", Status: status, Quit: func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}})
	log.Printf("bye")
}
