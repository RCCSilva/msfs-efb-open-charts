// Package navseed reads the identifiers of waypoints, VORs and NDBs from the navigation packages
// the simulator keeps on the user's disk (MSFS 2024 StreamedPackages). Only the identifier and its
// approximate position are taken: the rest of what is known about a place (region, frequency,
// airways) always comes from the simulator itself, through SimConnect.
//
// The files are only read, never written, and nothing from them is stored beyond the identifiers.
package navseed

import (
	"encoding/binary"
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

// Read reads the identifiers of the given archives. progress, when set, is told how many files
// were read out of how many; a false return from it stops the reading.
func Read(paths []string, progress func(done, total int) bool) ([]navdb.SeedIdent, error) {
	if len(paths) == 0 {
		return nil, errors.New("no navigation package of the simulator was found on this computer")
	}
	seen := map[string]navdb.SeedIdent{}
	for _, path := range paths {
		a, err := fsarchive.Open(path)
		if err != nil {
			return nil, err
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
				return nil, errors.New("stopped")
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
						if r.ID != recordFix {
							continue
						}
						b := f.Bytes(r)
						if len(b) < 30 {
							continue
						}
						le := binary.LittleEndian
						lonU, latU := le.Uint32(b[2:6]), le.Uint32(b[6:10])
						id1 := le.Uint64(b[14:22])
						ident := unpack38(id1 >> 6)
						if ident == "" {
							continue
						}
						if _, ok := seen[ident]; !ok {
							seen[ident] = navdb.SeedIdent{
								Ident: ident, Kind: int(id1 & 0x3f),
								Lat: 90 - float64(latU)*(180.0/0x20000000),
								Lon: float64(lonU)*(360.0/0x30000000) - 180,
							}
						}
					}
				}
			}
		}
		a.Close()
	}
	out := make([]navdb.SeedIdent, 0, len(seen))
	for _, s := range seen {
		out = append(out, s)
	}
	return out, nil
}
