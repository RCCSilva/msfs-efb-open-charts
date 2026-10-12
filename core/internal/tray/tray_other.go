//go:build !windows

package tray

// Config describes the tray entry (unused outside Windows).
type Config struct {
	URL    string
	MapURL string
	Status func() string
	Quit   func()
}

// Open does nothing outside Windows.
func Open(string) {}

// Run blocks forever: there is no tray outside Windows, the server keeps running.
func Run(Config) { select {} }

// Stop does nothing outside Windows.
func Stop() {}
