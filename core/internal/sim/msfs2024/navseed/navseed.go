// Package navseed reads the identifiers of waypoints, VORs and NDBs from the navigation packages
// the simulator keeps on the user's disk (MSFS 2024 StreamedPackages). Only the identifier and its
// approximate position are taken: the rest of what is known about a place (region, frequency,
// airways) always comes from the simulator itself, through SimConnect.
//
// The files are only read, never written, and nothing from them is stored beyond the identifiers.
package navseed

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/bgl"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/msfs2024/fsarchive"
)

// recordFix is the BGL record that holds a waypoint, VOR or NDB of the navigation packages.
const recordFix = 0x0108

// sectionBlobs is the section whose records the structure reader cannot tell apart: they hold runs
// of fix records of the airways (see scanBlob).
const sectionBlobs = 0x22

// the packages of the simulator that carry navigation data
var packages = []string{"fs24-fs-base-nav", "fs24-fs-base-genericairports"}

// Locate returns the archives that exist on this computer. Roots are tried in this order: the
// OPENCHARTS_MSFS_PACKAGES variable, the Steam/direct install under %APPDATA%, the Store install.
func Locate() []string {
	var roots []string
	if v := os.Getenv("OPENCHARTS_MSFS_PACKAGES"); v != "" {
		roots = append(roots, v)
	}
	if v := os.Getenv("APPDATA"); v != "" {
		roots = append(roots, filepath.Join(v, "Microsoft Flight Simulator 2024", "Packages", "StreamedPackages"))
	}
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		roots = append(roots, filepath.Join(v, "Packages", "Microsoft.Limitless_8wekyb3d8bbwe", "LocalCache", "Packages", "StreamedPackages"))
	}
	var out []string
	for _, root := range roots {
		for _, p := range packages {
			matches, _ := filepath.Glob(filepath.Join(root, p, "content", "*.fsarchive"))
			out = append(out, matches...)
		}
		if len(out) > 0 {
			break
		}
	}
	return out
}

const base38 = " ?0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

func unpack38(v uint64) string {
	var out []byte
	for v != 0 {
		out = append(out, base38[v%38])
		v /= 38
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return strings.TrimSpace(string(out))
}

// walk calls fn with the data of every fix record of the archives. progress, when set, is told how
// many files were read out of how many; a false return from it stops the reading.
func walk(paths []string, progress func(done, total int) bool, fn func(b []byte)) error {
	if len(paths) == 0 {
		return errors.New("no navigation package of the simulator was found on this computer")
	}
	for _, path := range paths {
		a, err := fsarchive.Open(path)
		if err != nil {
			return err
		}
		var files []fsarchive.Entry
		for _, e := range a.Entries {
			if strings.HasSuffix(strings.ToLower(e.Name()), ".bgl") {
				files = append(files, e)
			}
		}
		for i, e := range files {
			if progress != nil && !progress(i, len(files)) {
				a.Close()
				return errors.New("stopped")
			}
			data, err := a.Read(e)
			if err != nil {
				continue
			}
			f, err := bgl.Parse(data)
			if err != nil {
				continue // a file of another kind is not an error
			}
			for _, s := range f.Sections {
				for _, sub := range s.Subsections {
					for _, r := range sub.Records {
						switch {
						case r.ID == recordFix:
							fn(f.Bytes(r))
						case s.Type == sectionBlobs:
							scanBlob(f.Bytes(r), fn)
						}
					}
				}
			}
		}
		a.Close()
	}
	return nil
}

// Read reads the identifiers of the given archives. progress, when set, is told how many files
// were read out of how many; a false return from it stops the reading.
func Read(paths []string, progress func(done, total int) bool) ([]navdb.SeedIdent, error) {
	seen := map[string]navdb.SeedIdent{}
	err := walk(paths, progress, func(b []byte) {
		f, ok := parseFix(b)
		if !ok {
			return
		}
		// a name can be several fixes (a VOR and a waypoint of the same name), each of its own
		// class: Classes holds one bit per class found
		bit := 1 << f.Class
		if s, ok := seen[f.Ident]; ok {
			s.Classes |= bit
			seen[f.Ident] = s
			return
		}
		seen[f.Ident] = navdb.SeedIdent{Ident: f.Ident, Classes: bit, Lat: f.Lat, Lon: f.Lon}
	})
	if err != nil {
		return nil, err
	}
	out := make([]navdb.SeedIdent, 0, len(seen))
	for _, s := range seen {
		out = append(out, s)
	}
	return out, nil
}

// ReadFixes reads every fix of the archives with its region and the airways through it.
func ReadFixes(paths []string, progress func(done, total int) bool) ([]FileFix, error) {
	var out []FileFix
	err := walk(paths, progress, func(b []byte) {
		if f, ok := parseFix(b); ok {
			out = append(out, f)
		}
	})
	return out, err
}
