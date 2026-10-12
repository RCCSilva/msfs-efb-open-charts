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
}

// Default returns the settings of a fresh installation (a new key is generated).
func Default() Settings {
	return Settings{Port: DefaultPort, LAN: false, Token: NewToken(), Detail: DetailNormal}
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
