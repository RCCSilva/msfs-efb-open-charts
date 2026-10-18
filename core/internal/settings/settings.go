// Package settings holds the few options of the application that survive restarts: the port the
// server listens on, whether other devices of the local network may connect, and the key those
// devices must present. They live in settings.json inside the data folder.
package settings

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultPort is what Open Charts listens on unless told otherwise. It is deliberately not one of
// the busy development ports (3000, 5000, 8000, 8080...) that other programs of the user may hold.
const DefaultPort = 43210

// FileName of the settings inside the data folder.
const FileName = "settings.json"

// Detail levels of imported charts: the size, in pixels, of the longer side of the chart image.
const (
	DetailNormal = "normal" // 3200 px: light, fast to import and to load in the simulator
	DetailHigh   = "high"   // 4800 px: sharper when zoomed in, bigger files, slower
)

// How sharp the charts that cover a lot of ground (the WACs, the en-route charts) are drawn. A
// chart of this kind has so many metres in a pixel that it is blurred when the map is zoomed in;
// drawing it sharper makes a pyramid of many more tiles (see internal/importer/detail.go).
const (
	LargeStandard = "standard" // the automatic resolution: light and fast, but blurred when zoomed in
	LargeBalanced = "balanced" // sharp to map zoom 11: 70 metres per pixel at the finest level (the default)
	LargeSharp    = "sharp"    // sharp to map zoom 12: 35 metres per pixel at the finest level
	LargeMaximum  = "maximum"  // sharp to map zoom 13: 17.5 metres per pixel, for the few that want it
)

// ValidLargeCharts reports whether s is one of the levels above.
func ValidLargeCharts(s string) bool {
	return s == LargeStandard || s == LargeBalanced || s == LargeSharp || s == LargeMaximum
}

// DetailMPerPx is the ground resolution asked for at the finest level of a large chart, in metres
// per pixel, for a level (0: leave the automatic resolution).
func DetailMPerPx(level string) float64 {
	switch level {
	case LargeBalanced:
		return 70
	case LargeSharp:
		return 35
	case LargeMaximum:
		return 17.5
	}
	return 0
}

// Base maps drawn under the charts (see internal/server/tilestyle.go).
const (
	BasemapStandard = "standard" // the default: the regular OpenStreetMap tiles
	BasemapLight    = "light"    // a quiet grey-scale version of OpenStreetMap (lighter to download)
	BasemapDark     = "dark"
	BasemapNone     = "none"
)

// The aircraft symbol drawn on the map: a shape and a colour.
const (
	AircraftPlane        = "plane"
	AircraftHelicopter   = "helicopter"
	DefaultAircraftColor = "yellow"
)

// AircraftColor is one of the colours offered for the symbol.
type AircraftColor struct {
	ID  string `json:"id"`
	Hex string `json:"hex"`
}

// AircraftColors is the list the manager offers (the first is the default).
var AircraftColors = []AircraftColor{
	{"yellow", "#ffd21f"}, {"white", "#ffffff"}, {"orange", "#ff8c1a"}, {"red", "#e53935"},
	{"magenta", "#d81b9a"}, {"blue", "#2f80ed"}, {"cyan", "#19b8d8"}, {"green", "#34c759"}, {"black", "#1a1a1a"},
}

// ValidAircraft reports whether shape is a known aircraft symbol.
func ValidAircraft(shape string) bool { return shape == AircraftPlane || shape == AircraftHelicopter }

// AircraftHex returns the colour value of an aircraft colour id ("" if unknown).
func AircraftHex(id string) string {
	for _, c := range AircraftColors {
		if c.ID == id {
			return c.Hex
		}
	}
	return ""
}

// MaxNamePatterns is how many file-name patterns a user may keep.
const MaxNamePatterns = 10

// ValidBasemap reports whether s is a known base map style.
func ValidBasemap(s string) bool {
	return s == BasemapStandard || s == BasemapLight || s == BasemapDark || s == BasemapNone
}

// LongEdgePx is the longer side of the image for a detail level (unknown values mean normal).
func LongEdgePx(detail string) int {
	if detail == DetailHigh {
		return 4800
	}
	return 3200
}

// Settings are the persisted options.
type Settings struct {
	Port  int    `json:"port"`
	LAN   bool   `json:"lan"`   // accept connections from other devices of the local network
	Token string `json:"token"` // the key those devices must present (in the QR code link)
	// Detail is the level of detail of charts imported from now on (DetailNormal or DetailHigh).
	Detail string `json:"detail"`
	// LargeCharts is how sharp the big-area charts are drawn, for charts imported from now on.
	LargeCharts string `json:"largeCharts"`
	// Basemap is what the viewer draws under the chart (BasemapStandard by default).
	Basemap string `json:"basemap"`
	// Aircraft and AircraftColor are the symbol shown for the aircraft position (a yellow plane by default).
	Aircraft      string `json:"aircraft"`
	AircraftColor string `json:"aircraftColor"`
	// NamePatterns are file-name templates of the user's own PDFs ("{icao}_{type}_{title}_{date}"),
	// tried in order for the "My PDFs" source (see package naming).
	NamePatterns []string `json:"namePatterns,omitempty"`
}

// Default returns the settings of a fresh installation (a new key is generated).
func Default() Settings {
	return Settings{Port: DefaultPort, LAN: false, Token: NewToken(), Detail: DetailNormal, LargeCharts: LargeBalanced, Basemap: BasemapStandard,
		Aircraft: AircraftPlane, AircraftColor: DefaultAircraftColor}
}

// NewToken returns a random key (96 bits, hex).
func NewToken() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the system random source failing is not recoverable
	}
	return hex.EncodeToString(b)
}

// ValidatePort accepts ports a program may use without special rights.
func ValidatePort(p int) error {
	if p < 1024 || p > 65535 {
		return fmt.Errorf("port must be between 1024 and 65535")
	}
	return nil
}

// Load reads the settings of a data folder. A missing or unreadable file gives the defaults, and
// an empty key is replaced by a new one (and saved), so a caller never has to handle an error.
func Load(dir string) Settings {
	s := Default()
	if b, err := os.ReadFile(filepath.Join(dir, FileName)); err == nil {
		var f Settings
		if json.Unmarshal(b, &f) == nil {
			if ValidatePort(f.Port) == nil {
				s.Port = f.Port
			}
			s.LAN = f.LAN
			if f.Detail == DetailHigh {
				s.Detail = DetailHigh
			}
			if ValidLargeCharts(f.LargeCharts) {
				s.LargeCharts = f.LargeCharts
			}
			if ValidBasemap(f.Basemap) {
				s.Basemap = f.Basemap
			}
			if ValidAircraft(f.Aircraft) {
				s.Aircraft = f.Aircraft
			}
			if AircraftHex(f.AircraftColor) != "" {
				s.AircraftColor = f.AircraftColor
			}
			for _, p := range f.NamePatterns {
				if p = strings.TrimSpace(p); p != "" && len(s.NamePatterns) < MaxNamePatterns {
					s.NamePatterns = append(s.NamePatterns, p)
				}
			}
			if f.Token != "" {
				s.Token = f.Token
			}
		}
	}
	return s
}

// Save writes the settings atomically.
func (s Settings) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, FileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ListenAddr is the address to listen on: only this computer, or every network interface when
// the local network is allowed.
func (s Settings) ListenAddr() string {
	host := "127.0.0.1"
	if s.LAN {
		host = "0.0.0.0"
	}
	return host + ":" + strconv.Itoa(s.Port)
}
