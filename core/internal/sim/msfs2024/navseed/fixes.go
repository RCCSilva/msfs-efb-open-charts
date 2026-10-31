package navseed

import (
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// fixKey tells one fix from another: the files repeat a fix many times, and a name can be several
// fixes in several regions and classes.
type fixKey struct {
	ident, region string
	class         int
}

func (f FileFix) key() fixKey { return fixKey{f.Ident, f.Region, f.Class} }

// ReadFixes reads every fix of the archives, each once, with its region and the airways through it
// (the airways of its repeats are joined). progress is as in Read.
func ReadFixes(paths []string, progress func(done, total int) bool) ([]FileFix, error) {
	byKey := map[fixKey]int{}
	var out []FileFix
	err := walk(paths, progress, func(b []byte, l layout) {
		f, ok := parseFix(b, l)
		if !ok {
			return
		}
		i, seen := byKey[f.key()]
		if !seen {
			byKey[f.key()] = len(out)
			out = append(out, f)
			return
		}
		have := out[i].Airways
	next:
		for _, a := range f.Airways {
			for _, h := range have {
				if h.Name == a.Name && h.Prev.Ident == a.Prev.Ident && h.Next.Ident == a.Next.Ident {
					continue next
				}
			}
			have = append(have, a)
		}
		out[i].Airways = have
	})
	return out, err
}

// kindOfClass is the kind of fix a class of the files stands for (see navdb.KindsToAsk).
func kindOfClass(class int) byte {
	switch class {
	case 2:
		return navdata.FixVOR
	case 3:
		return navdata.FixNDB
	}
	return navdata.FixWaypoint
}

// Waypoints reads the waypoints of the archives as the database stores them, with the airways through
// them. Those whose region is not a code of two letters are left out (see validRegion): the simulator
// is asked for them. A neighbour on an airway that the files have no record of (they hold only part of the fixes)
// is kept by name, with no position. VORs and NDBs are not taken: the files do not have their names
// and frequencies.
func Waypoints(paths []string, progress func(done, total int) bool) ([]navdata.Fix, error) {
	files, err := ReadFixes(paths, progress)
	if err != nil {
		return nil, err
	}
	return waypoints(files), nil
}

// waypoints is Waypoints for fixes already read.
func waypoints(files []FileFix) []navdata.Fix {
	at := make(map[fixKey]FileFix, len(files))
	for _, f := range files {
		at[f.key()] = f
	}
	var out []navdata.Fix
	for _, f := range files {
		if kindOfClass(f.Class) != navdata.FixWaypoint || !validRegion(f.Region) {
			continue
		}
		fix := navdata.Fix{Ident: f.Ident, Region: f.Region, Kind: navdata.FixWaypoint, Lat: f.Lat, Lon: f.Lon}
		for _, a := range f.Airways {
			r := navdata.Route{Airway: a.Name, Type: a.Type}
			// a neighbour the files have no record of is kept by name, at 0, 0: the simulator is
			// asked for it later and its position is filled in (navdb.FillRoutePositions)
			if a.HasNext {
				r.NextIdent, r.NextRegion, r.NextKind = a.Next.Ident, a.Next.Region, kindOfClass(a.Next.Class)
				if n, ok := at[a.Next.key()]; ok {
					r.NextLat, r.NextLon = n.Lat, n.Lon
				}
			}
			if a.HasPrev {
				r.PrevIdent, r.PrevRegion, r.PrevKind = a.Prev.Ident, a.Prev.Region, kindOfClass(a.Prev.Class)
				if p, ok := at[a.Prev.key()]; ok {
					r.PrevLat, r.PrevLon = p.Lat, p.Lon
				}
			}
			if r.NextIdent != "" || r.PrevIdent != "" {
				fix.Routes = append(fix.Routes, r)
			}
		}
		out = append(out, fix)
	}
	return out
}
