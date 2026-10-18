package navdb

import "math"

// How wide a view may be, in degrees on its larger side, for each thing to be shown: further out they
// are too many to mean anything. They are about the zooms 5 (airports), 6 (airways) and 7 (fixes) of
// the map of the manager.
const (
	MaxSpanAirports = 35.0
	MaxSpanSegments = 20.0
	MaxSpanFixes    = 8.0
)

// InAreaView is InArea for what a map shows: a map can be moved past the antimeridian, so that its
// west and east edges are longitudes beyond -180 and 180 (the world repeats every 360 degrees). The
// rectangle is cut into the stretches of the real world that it covers, each is asked, and what is
// found is returned in the longitudes of the view: where a place was found one world to the east, it
// is at its longitude plus 360. So a place shows wherever the map is looking at it, and a stretch of
// airway that crosses the antimeridian is drawn the short way.
func (d *DB) InAreaView(minLat, minLon, maxLat, maxLon float64, maxFixes, maxSegments int, withFixes, withSegments bool) (Area, error) {
	out := Area{Airports: []AreaAirport{}, Fixes: []FixRow{}, Segments: []AreaSegment{}}
	span := math.Max(maxLat-minLat, maxLon-minLon)
	out.AirportsHidden = span > MaxSpanAirports
	out.FixesHidden = withFixes && span > MaxSpanFixes
	out.SegmentsHidden = withSegments && span > MaxSpanSegments
	withFixes = withFixes && !out.FixesHidden
	withSegments = withSegments && !out.SegmentsHidden
	for _, r := range worldStretches(minLon, maxLon) {
		a, err := d.InArea(minLat, r.lo, maxLat, r.hi, maxFixes, maxSegments, withFixes, withSegments)
		if err != nil {
			return out, err
		}
		if out.AirportsHidden {
			a.Airports = nil
		}
		for _, x := range a.Airports {
			x.Lon += r.shift
			out.Airports = append(out.Airports, x)
		}
		for _, x := range a.Fixes {
			x.Lon += r.shift
			out.Fixes = append(out.Fixes, x)
		}
		for _, s := range a.Segments {
			s.Lon += r.shift
			s.ToLon += r.shift
			switch d := s.ToLon - s.Lon; {
			case d > 180:
				s.ToLon -= 360
			case d < -180:
				s.ToLon += 360
			}
			out.Segments = append(out.Segments, s)
		}
		out.FixesCut = out.FixesCut || a.FixesCut
		out.SegmentsCut = out.SegmentsCut || a.SegmentsCut
	}
	if len(out.Fixes) > maxFixes {
		out.Fixes, out.FixesCut = out.Fixes[:maxFixes], true
	}
	if len(out.Segments) > maxSegments {
		out.Segments, out.SegmentsCut = out.Segments[:maxSegments], true
	}
	return out, nil
}

// stretch is a part of the real world (longitudes from lo to hi, in -180..180) and how much to add to
// them to put them where the view has them.
type stretch struct{ lo, hi, shift float64 }

// worldStretches cuts the longitudes of a view (which may go past -180 and 180) into the stretches of
// the real world it covers. A view of a whole world or more gives the world once, as the view's
// middle copy has it.
func worldStretches(minLon, maxLon float64) []stretch {
	if maxLon-minLon >= 360 {
		shift := 360 * math.Floor(((minLon+maxLon)/2+180)/360)
		return []stretch{{-180, 180, shift}}
	}
	k := math.Floor((minLon + 180) / 360)
	shift := 360 * k
	w, e := minLon-shift, maxLon-shift
	if e <= 180 {
		return []stretch{{w, e, shift}}
	}
	return []stretch{{w, 180, shift}, {-180, e - 360, shift + 360}}
}
