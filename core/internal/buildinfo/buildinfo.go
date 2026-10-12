// Package buildinfo tells which build of the program is running.
//
// A release build (installer\build.ps1) shows just the version, "0.1.1". Any other build, such as
// "go build" while developing, shows the version with the commit it was built from, "0.1.1-8fa9b5b",
// plus "-dirty" when the working tree had uncommitted changes. The commit comes from the build
// information Go stamps into the executable, so nothing has to be edited at each commit.
package buildinfo

import "runtime/debug"

// Release is set to "1" at link time by the installer build:
//
//	go build -ldflags "-X github.com/RCCSilva/msfs-efb-open-charts/core/internal/buildinfo.Release=1"
var Release string

// Info describes a build.
type Info struct {
	Version  string `json:"version"`  // "0.1.1"
	Display  string `json:"display"`  // "0.1.1" (release) or "0.1.1-8fa9b5b[-dirty]" (development)
	Revision string `json:"revision"` // short commit hash, "" when unknown
	Dirty    bool   `json:"dirty"`    // uncommitted changes when it was built
	Release  bool   `json:"release"`
}

// Get returns the build information for the given base version.
func Get(version string) Info {
	info := Info{Version: version, Display: version, Release: Release == "1"}
	if info.Release {
		return info
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Revision = s.Value
			case "vcs.modified":
				info.Dirty = s.Value == "true"
			}
		}
	}
	if len(info.Revision) > 7 {
		info.Revision = info.Revision[:7]
	}
	suffix := info.Revision
	if suffix == "" {
		suffix = "dev"
	}
	if info.Dirty {
		suffix += "-dirty"
	}
	info.Display = version + "-" + suffix
	return info
}

// Display is shorthand for Get(version).Display.
func Display(version string) string { return Get(version).Display }
