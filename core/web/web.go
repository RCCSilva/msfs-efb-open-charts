// Package web embeds the in-sim viewer (served to the MSFS EFB iframe and to any browser)
// and the manager UI.
package web

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed sim manager
var content embed.FS

var devDir string

// SetDir makes Sim and Manager read from a folder on disk (containing "sim" and "manager")
// instead of the embedded copy. Meant for working on the UI without rebuilding.
func SetDir(dir string) { devDir = dir }

func sub(name string) fs.FS {
	if devDir != "" {
		return os.DirFS(filepath.Join(devDir, name))
	}
	s, err := fs.Sub(content, name)
	if err != nil {
		panic(err)
	}
	return s
}

// Sim returns the in-sim viewer files rooted at "/".
func Sim() fs.FS { return sub("sim") }

// Manager returns the manager UI files rooted at "/".
func Manager() fs.FS { return sub("manager") }
