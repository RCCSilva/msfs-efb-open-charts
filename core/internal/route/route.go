// Package route reads a route written the way pilots write it ("SBSJ BOBI3A DAGUB UZ6 ABCDE SBGR")
// and finds every part of it in the navigation data: airports, fixes, VORs, NDBs, airways and
// procedures.
//
// It plans nothing: the route is the user's. Each name is looked up where the data lives (the
// provider decides: the database first, the simulator when the database does not know it), and
// whatever is not found is reported as a problem, never guessed.
package route

import (
	"errors"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// Provider is where names are looked up.
type Provider interface {
	// Airport returns the airport, or (nil, nil) when it does not exist.
	Airport(icao string) (*navdata.Airport, error)
	// Fixes returns every waypoint, VOR and NDB with that identifier, with the airways through
	// them. Empty means that nothing by that name is known anywhere.
	Fixes(ident string) ([]navdata.Fix, error)
	// FixAt returns one fix with its airways, or (nil, nil) when it is not known.
	FixAt(ref navdata.FixRef) (*navdata.Fix, error)
}

// Point is one place of the resolved route, in flying order.
type Point struct {
	Token  string  `json:"token"`            // what the user wrote
	Kind   string  `json:"kind"`             // airport, waypoint, vor, ndb, coordinate
	Ident  string  `json:"ident"`            // identifier of the place
	Region string  `json:"region,omitempty"` // region code, for fixes
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	// Via is the airway or the procedure that leads here from the previous point; empty for a
	// direct leg.
	Via string `json:"via,omitempty"`
	// Nth is which occurrence of this name in the route it is (0 for the first), the key of a choice.
	Nth int `json:"nth"`
	// Candidates lists every place the name can mean when there is more than one (the point is the
	// one used). Chosen tells that the user picked it, instead of the closest being taken.
	Candidates []Candidate `json:"candidates,omitempty"`
	Chosen     bool        `json:"chosen,omitempty"`
}

// Choice is the user's pick between places that share a name. It refers to the Nth occurrence of
// the name in the route, so editing other parts of the text does not move it.
type Choice struct {
	Token  string `json:"token"`
	Nth    int    `json:"nth"`
	Region string `json:"region"`
	Kind   string `json:"kind"` // waypoint, vor, ndb
}

// Candidate is one of several places a name can mean.
type Candidate struct {
	Ident  string  `json:"ident"`
	Region string  `json:"region"`
	Kind   string  `json:"kind"`
	Name   string  `json:"name,omitempty"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	// DistKm is how far it is from the previous point of the route (0 when there is none).
	DistKm float64 `json:"distKm,omitempty"`
}

// Problem is a part of the route that could not be resolved, or was resolved with a doubt.
type Problem struct {
	Token      string      `json:"token"`
	Message    string      `json:"message"`
	Candidates []Candidate `json:"candidates,omitempty"`
}

// Result is a resolved route.
type Result struct {
	Points   []Point   `json:"points"`
	Problems []Problem `json:"problems"`
}

var (
	levelGroup  = regexp.MustCompile(`^[NMK]\d{3,4}[FAMS]\d{3,4}$`)
	airwayName  = regexp.MustCompile(`^[A-Z]{1,2}\d{1,3}[A-Z]?$`)
	icaoAirport = regexp.MustCompile(`^[A-Z0-9]{4}$`)
	// 4530N07030W, 45N070W, 4530N/07030W is already split by the slash rule
	coordLong  = regexp.MustCompile(`^(\d{2})(\d{2})([NS])(\d{3})(\d{2})([EW])$`)
	coordShort = regexp.MustCompile(`^(\d{2})([NS])(\d{3})([EW])$`)
)

// Tokens splits a route into names: whitespace separated, upper case, "/speed-level" suffixes and
// DCT removed, level groups (N0450F350) dropped.
func Tokens(text string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ToUpper(text)) {
		if i := strings.IndexByte(w, '/'); i > 0 {
			w = w[:i]
		}
		w = strings.Trim(w, ",;")
		switch {
		case w == "", w == "DCT", w == "SID", w == "STAR", w == "VFR", w == "IFR":
			continue
		case levelGroup.MatchString(w):
			continue
		}
		out = append(out, w)
	}
	return out
}

func kindName(k byte) string {
	switch k {
	case navdata.FixVOR:
		return "vor"
	case navdata.FixNDB:
		return "ndb"
	}
	return "waypoint"
}

func parseCoordinate(tok string) (lat, lon float64, ok bool) {
	if m := coordLong.FindStringSubmatch(tok); m != nil {
		d, _ := strconv.Atoi(m[1])
		mi, _ := strconv.Atoi(m[2])
		lat = float64(d) + float64(mi)/60
		if m[3] == "S" {
			lat = -lat
		}
		d, _ = strconv.Atoi(m[4])
		mi, _ = strconv.Atoi(m[5])
		lon = float64(d) + float64(mi)/60
		if m[6] == "W" {
			lon = -lon
		}
		return lat, lon, true
	}
	if m := coordShort.FindStringSubmatch(tok); m != nil {
		d, _ := strconv.Atoi(m[1])
		lat = float64(d)
		if m[2] == "S" {
			lat = -lat
		}
		d, _ = strconv.Atoi(m[3])
		lon = float64(d)
		if m[4] == "W" {
			lon = -lon
		}
		return lat, lon, true
	}
	return 0, 0, false
}

func distance(aLat, aLon, bLat, bLon float64) float64 {
	rad := math.Pi / 180
	dLat := (bLat - aLat) * rad
	dLon := (bLon - aLon) * rad * math.Cos((aLat+bLat)/2*rad)
	return math.Hypot(dLat, dLon) * 6371
}

type resolver struct {
	p        Provider
	res      Result
	airports map[string]*navdata.Airport
	last     *Point // the last resolved point, for choosing between homonyms
	choices  []Choice
	seen     map[string]int // occurrences of each name met so far
	nth      int            // occurrence number of the name being resolved
}

// Resolve finds every name of the route.
func Resolve(text string, p Provider) (*Result, error) { return ResolveWith(text, p, nil) }

// ResolveWith is Resolve with the user's picks between places that share a name. A name with
// several places gets the closest to the previous point, unless a choice says otherwise.
func ResolveWith(text string, p Provider, choices []Choice) (*Result, error) {
	toks := Tokens(text)
	r := &resolver{p: p, res: Result{Points: []Point{}, Problems: []Problem{}}, airports: map[string]*navdata.Airport{},
		choices: choices, seen: map[string]int{}}
	if len(toks) == 0 {
		return &r.res, nil
	}
	for i := 0; i < len(toks); i++ {
		tok := toks[i]
		consumed, err := r.step(toks, i)
		if err != nil {
			return &r.res, err
		}
		i += consumed
		_ = tok
	}
	return &r.res, nil
}

func (r *resolver) add(pt Point) {
	r.res.Points = append(r.res.Points, pt)
	r.last = &r.res.Points[len(r.res.Points)-1]
}

func (r *resolver) problem(tok, msg string, c []Candidate) {
	r.res.Problems = append(r.res.Problems, Problem{Token: tok, Message: msg, Candidates: c})
}

func (r *resolver) airport(icao string) (*navdata.Airport, error) {
	if a, ok := r.airports[icao]; ok {
		return a, nil
	}
	a, err := r.p.Airport(icao)
	if err != nil {
		return nil, err
	}
	r.airports[icao] = a
	return a, nil
}

// the airport at the start of the route (the first resolved airport), for procedures
func (r *resolver) firstAirport() *navdata.Airport {
	for _, p := range r.res.Points {
		if p.Kind == "airport" {
			return r.airports[p.Ident]
		}
	}
	return nil
}

// step resolves toks[i] and returns how many following tokens it also used.
func (r *resolver) step(toks []string, i int) (int, error) {
	tok := toks[i]
	first, last := i == 0, i == len(toks)-1
	r.nth = r.seen[tok]
	r.seen[tok]++

	// an airport at either end
	if (first || last) && icaoAirport.MatchString(tok) {
		a, err := r.airport(tok)
		if err != nil {
			return 0, err
		}
		if a != nil {
			r.add(Point{Token: tok, Kind: "airport", Ident: a.ICAO, Region: a.Region, Lat: a.Lat, Lon: a.Lon})
			return 0, nil
		}
		if first || last {
			// not an airport: it may be a fix by chance (4 letters), try below
		}
	}

	// a coordinate
	if lat, lon, ok := parseCoordinate(tok); ok {
		r.add(Point{Token: tok, Kind: "coordinate", Ident: tok, Lat: lat, Lon: lon})
		return 0, nil
	}

	// a departure or arrival procedure of the airports already seen
	if used, ok, err := r.procedure(toks, i); err != nil {
		return 0, err
	} else if ok {
		return used, nil
	}

	// an airway between the previous point and the next fix
	if airwayName.MatchString(tok) && r.last != nil && i+1 < len(toks) {
		if r.expandAirway(tok, toks[i+1]) {
			return 1, nil
		}
	}

	// the fix a procedure or an airway just ended at, written again by the user: it is the same place
	if r.last != nil && r.last.Via != "" && r.last.Ident == tok {
		return 0, nil
	}

	// a fix, a VOR or an NDB
	fixes, err := r.p.Fixes(tok)
	if err != nil {
		return 0, err
	}
	if len(fixes) == 0 {
		// the last chance for a 4-letter name that is an airport in the middle of a route
		if icaoAirport.MatchString(tok) {
			if a, err := r.airport(tok); err != nil {
				return 0, err
			} else if a != nil {
				r.add(Point{Token: tok, Kind: "airport", Ident: a.ICAO, Region: a.Region, Lat: a.Lat, Lon: a.Lon})
				return 0, nil
			}
		}
		r.problem(tok, "not found: the simulator does not know this name", nil)
		return 0, nil
	}
	f := fixes[0]
	var cands []Candidate
	chosen := false
	if len(fixes) > 1 {
		f = r.nearest(fixes)
		for _, x := range fixes {
			c := Candidate{Ident: x.Ident, Region: x.Region, Kind: kindName(x.Kind), Name: x.Name, Lat: x.Lat, Lon: x.Lon}
			if r.last != nil {
				c.DistKm = math.Round(distance(r.last.Lat, r.last.Lon, x.Lat, x.Lon)*10) / 10
			}
			cands = append(cands, c)
		}
		sort.SliceStable(cands, func(a, b int) bool { return cands[a].DistKm < cands[b].DistKm })
		if pick, ok := r.pick(tok, fixes); ok {
			f, chosen = pick, true
		} else {
			r.problem(tok, "more than one place has this name: the one closest to the route so far was used", cands)
		}
	}
	r.add(Point{Token: tok, Kind: kindName(f.Kind), Ident: f.Ident, Region: f.Region, Lat: f.Lat, Lon: f.Lon,
		Nth: r.nth, Candidates: cands, Chosen: chosen})
	return 0, nil
}

// pick applies the user's choice for this occurrence of the name, when there is one and it still
// matches one of the places.
func (r *resolver) pick(tok string, fixes []navdata.Fix) (navdata.Fix, bool) {
	for _, c := range r.choices {
		if !strings.EqualFold(c.Token, tok) || c.Nth != r.nth {
			continue
		}
		for _, f := range fixes {
			if strings.EqualFold(f.Region, c.Region) && (c.Kind == "" || kindName(f.Kind) == c.Kind) {
				return f, true
			}
		}
	}
	return navdata.Fix{}, false
}

func (r *resolver) nearest(fixes []navdata.Fix) navdata.Fix {
	if r.last == nil {
		return fixes[0]
	}
	best, bestD := fixes[0], math.Inf(1)
	for _, f := range fixes {
		if d := distance(r.last.Lat, r.last.Lon, f.Lat, f.Lon); d < bestD {
			best, bestD = f, d
		}
	}
	return best
}

// procedure resolves a SID or STAR name of an airport already in the route, and uses the fix that
// follows it (the enroute transition of a SID, or the one before a STAR) when it names one.
func (r *resolver) procedure(toks []string, i int) (int, bool, error) {
	tok := toks[i]
	var pr *navdata.Procedure
	var ap *navdata.Airport
	for _, p := range r.res.Points {
		if p.Kind != "airport" {
			continue
		}
		a := r.airports[p.Ident]
		if a == nil {
			continue
		}
		for k := range a.Procedures {
			x := &a.Procedures[k]
			if x.Name == tok && (x.Kind == navdata.KindSID || x.Kind == navdata.KindSTAR) {
				pr, ap = x, a
			}
		}
	}
	if pr == nil && i+1 == len(toks) {
		return 0, false, nil
	}
	// a STAR of the destination, which is only the next token: look it up before the destination
	if pr == nil && i+1 < len(toks) && icaoAirport.MatchString(toks[len(toks)-1]) {
		a, err := r.airport(toks[len(toks)-1])
		if err != nil {
			return 0, false, err
		}
		if a != nil {
			for k := range a.Procedures {
				x := &a.Procedures[k]
				if x.Name == tok && (x.Kind == navdata.KindSID || x.Kind == navdata.KindSTAR) {
					pr, ap = x, a
				}
			}
		}
	}
	if pr == nil {
		return 0, false, nil
	}
	// the transition named by the neighbouring token
	neighbour := ""
	if pr.Kind == navdata.KindSID && i+1 < len(toks) {
		neighbour = toks[i+1]
	}
	if pr.Kind == navdata.KindSTAR && i > 0 {
		neighbour = toks[i-1]
	}
	var legs []navdata.Leg
	for _, t := range pr.Transitions {
		if t.Kind == navdata.TransCommon {
			legs = append(legs, t.Legs...)
		}
	}
	for _, t := range pr.Transitions {
		if t.Kind == navdata.TransEnroute && t.Name == neighbour && neighbour != "" {
			if pr.Kind == navdata.KindSID {
				legs = append(legs, t.Legs...)
			} else {
				legs = append(append([]navdata.Leg{}, t.Legs...), legs...)
			}
		}
	}
	if len(legs) == 0 {
		r.add(Point{Token: tok, Kind: "procedure", Ident: pr.Name, Lat: ap.Lat, Lon: ap.Lon, Via: pr.Kind})
		return 0, true, nil
	}
	added := 0
	for _, l := range legs {
		if l.Lat == nil || l.Lon == nil {
			continue
		}
		r.add(Point{Token: tok, Kind: "waypoint", Ident: l.FixICAO, Region: l.FixRegion, Lat: *l.Lat, Lon: *l.Lon, Via: pr.Name})
		added++
	}
	if added == 0 {
		r.problem(tok, "the procedure is known but its fixes have no position yet", nil)
	}
	return 0, true, nil
}

// expandAirway walks the airway from the last resolved point to the fix named by `to`. It returns
// false when the airway cannot be used between those two (the caller then treats the token as a
// plain name).
func (r *resolver) expandAirway(name, to string) bool {
	if r.last == nil || r.last.Kind == "airport" || r.last.Kind == "coordinate" || r.last.Kind == "procedure" {
		return false
	}
	start := navdata.FixRef{Ident: r.last.Ident, Region: r.last.Region, Kind: kindByte(r.last.Kind)}
	target, err := r.p.Fixes(to)
	if err != nil || len(target) == 0 {
		return false
	}
	isTarget := func(ref navdata.FixRef) bool {
		for _, t := range target {
			if t.Ident == ref.Ident && (t.Region == ref.Region || t.Region == "" || ref.Region == "") {
				return true
			}
		}
		return false
	}

	type node struct {
		ref        navdata.FixRef
		prev       int // index in nodes, -1 for the start
		lat, lon   float64
		airway     string
		expandedBy string
	}
	nodes := []node{{ref: start, prev: -1, lat: r.last.Lat, lon: r.last.Lon}}
	seen := map[navdata.FixRef]bool{start: true}
	for head := 0; head < len(nodes) && head < 600; head++ {
		cur := nodes[head]
		fx, err := r.p.FixAt(cur.ref)
		if err != nil || fx == nil {
			continue
		}
		for _, rt := range fx.Routes {
			if rt.Airway != name {
				continue
			}
			for _, nb := range []struct {
				ident, region string
				kind          byte
				lat, lon      float64
			}{
				{rt.NextIdent, rt.NextRegion, rt.NextKind, rt.NextLat, rt.NextLon},
				{rt.PrevIdent, rt.PrevRegion, rt.PrevKind, rt.PrevLat, rt.PrevLon},
			} {
				if nb.ident == "" {
					continue
				}
				ref := navdata.FixRef{Ident: nb.ident, Region: nb.region, Kind: nb.kind}
				if seen[ref] {
					continue
				}
				seen[ref] = true
				nodes = append(nodes, node{ref: ref, prev: head, lat: nb.lat, lon: nb.lon, airway: name})
				if isTarget(ref) {
					// the path, from the start to here
					var path []node
					for k := len(nodes) - 1; k > 0; k = nodes[k].prev {
						path = append(path, nodes[k])
					}
					for j := len(path) - 1; j >= 0; j-- {
						n := path[j]
						r.add(Point{Token: name, Kind: kindName(n.ref.Kind), Ident: n.ref.Ident, Region: n.ref.Region, Lat: n.lat, Lon: n.lon, Via: name})
					}
					return true
				}
			}
		}
	}
	r.problem(name, "the airway does not connect "+r.last.Ident+" to "+to, nil)
	return true // the airway token is consumed; the next token is resolved on its own
}

func kindByte(kind string) byte {
	switch kind {
	case "vor":
		return navdata.FixVOR
	case "ndb":
		return navdata.FixNDB
	}
	return navdata.FixWaypoint
}

var _ = errors.New
