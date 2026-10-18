// Package navlookup answers the questions about places that reading a route raises: the navigation
// database first and, for what the database has never been asked about, the simulator, once for each
// name. What the simulator knows is stored, so the next question is answered from the database.
package navlookup

import (
	"errors"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

// Store is what the lookup reads from and writes to the navigation database.
type Store interface {
	Airport(icao string) (*navdata.Airport, error)
	SaveAirport(a *navdata.Airport) error
	// UncheckedIdents are the names the airport's procedures cite that were never asked to the simulator.
	UncheckedIdents(icao string) ([]string, error)
	IdentChecked(ident string) (bool, error)
	FixesByIdent(ident string) ([]navdata.Fix, error)
	UserFixes(ident string) ([]navdata.Fix, error)
	FixAt(ref navdata.FixRef) (*navdata.Fix, error)
	SaveFixes(fixes []navdata.Fix) error
	MarkIdentChecked(ident string, found int) error
}

// Source is what the lookup asks the simulator.
type Source interface {
	Airport(icao string) (*navdata.Airport, error)
	// LookupFixes asks for waypoints, VORs and NDBs as a whole: every place with that identifier.
	LookupFixes(idents []string) ([]navdata.Fix, error)
	// LookupFixKinds asks only for the kind given with each identifier.
	LookupFixKinds(asks []navdata.FixRef) ([]navdata.Fix, error)
}

// KindHints says which kinds of fix to ask the simulator for names that are known from its files. A
// name it does not know is absent.
type KindHints interface {
	SeedKinds(idents []string) (map[string][]byte, error)
}

// Lookup answers from a Store and, for what it does not have, from a Source (nil: there is no
// simulator to ask).
type Lookup struct {
	store Store
	src   Source
	hints KindHints                                  // nil: every name is asked as all three kinds
	timer func(name string, d time.Duration, n int) // nil: nothing is measured
}

// WithKindHints makes the Lookup ask each name only for the kinds the hints give (when they give
// any), which takes fewer questions to the simulator.
func (l *Lookup) WithKindHints(h KindHints) *Lookup {
	l.hints = h
	return l
}

// ask puts the names to the simulator: with hints, each as the kinds they give; any other as all three.
func (l *Lookup) ask(idents []string) ([]navdata.Fix, error) {
	if l.hints == nil {
		return l.src.LookupFixes(idents)
	}
	known, err := l.hints.SeedKinds(idents)
	if err != nil {
		return l.src.LookupFixes(idents) // no hint is better than no answer
	}
	var asks []navdata.FixRef
	for _, id := range idents {
		kinds := known[strings.ToUpper(id)]
		if len(kinds) == 0 {
			kinds = []byte{navdata.FixWaypoint, navdata.FixVOR, navdata.FixNDB}
		}
		for _, k := range kinds {
			asks = append(asks, navdata.FixRef{Ident: id, Kind: k})
		}
	}
	return l.src.LookupFixKinds(asks)
}

// WithTimer makes the Lookup report how long its calls to the simulator and to the database take.
func (l *Lookup) WithTimer(f func(name string, d time.Duration, n int)) *Lookup {
	l.timer = f
	return l
}

// timed reports one run of a step that handled n items.
func (l *Lookup) timed(name string, n int, f func()) {
	if l.timer == nil {
		f()
		return
	}
	start := time.Now()
	f()
	l.timer(name, time.Since(start), n)
}

// New builds a Lookup.
func New(store Store, src Source) *Lookup { return &Lookup{store: store, src: src} }

// Airport returns the airport with its legs placed. An unknown airport is (nil, nil).
func (l *Lookup) Airport(icao string) (*navdata.Airport, error) {
	a, err := l.store.Airport(icao)
	if err == nil {
		return l.ResolveFixes(a), nil
	}
	if !errors.Is(err, navdb.ErrNotFound) {
		return nil, err
	}
	if l.src == nil {
		return nil, nil // without a link to the simulator an unknown airport is just unknown
	}
	got, err := l.src.Airport(icao)
	if errors.Is(err, navdata.ErrUnknown) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := l.store.SaveAirport(got); err != nil {
		return nil, err
	}
	stored, err := l.store.Airport(icao)
	if err != nil {
		return nil, err
	}
	return l.ResolveFixes(stored), nil
}

// Fixes returns every waypoint, VOR and NDB with that identifier, and the user's own points.
func (l *Lookup) Fixes(ident string) ([]navdata.Fix, error) {
	checked, err := l.store.IdentChecked(ident)
	if err != nil {
		return nil, err
	}
	if !checked {
		// the simulator may not be reachable: then what is stored is the answer, and the name stays
		// unchecked so that the next time asks again
		_ = l.LoadIdents([]string{ident})
	}
	list, err := l.store.FixesByIdent(ident)
	if err != nil {
		return nil, err
	}
	// the points the user made are places like any other
	mine, err := l.store.UserFixes(ident)
	if err != nil {
		return nil, err
	}
	return append(list, mine...), nil
}

// FixAt returns one fix, asking the simulator about its name first when the database never has.
func (l *Lookup) FixAt(ref navdata.FixRef) (*navdata.Fix, error) {
	if strings.EqualFold(ref.Region, navdb.UserRegion) {
		mine, err := l.store.UserFixes(ref.Ident)
		if err != nil || len(mine) == 0 {
			return nil, err
		}
		return &mine[0], nil
	}
	if checked, err := l.store.IdentChecked(ref.Ident); err == nil && !checked {
		_ = l.LoadIdents([]string{ref.Ident})
	}
	return l.store.FixAt(ref)
}

// ResolveFixes asks the simulator for the fixes the procedures of an airport cite and the database
// does not know yet, stores them, and returns the airport again with every leg placed. Without a
// link to the simulator (or when it does not know a fix) the legs simply stay without a position.
func (l *Lookup) ResolveFixes(a *navdata.Airport) *navdata.Airport {
	idents, err := l.store.UncheckedIdents(a.ICAO)
	if err != nil || len(idents) == 0 {
		return a
	}
	if err := l.LoadIdents(idents); err != nil {
		return a
	}
	if again, err := l.store.Airport(a.ICAO); err == nil {
		return again
	}
	return a
}

// LoadIdents asks the simulator for identifiers as a whole (every place of that name, any region
// and kind), stores what it knows and marks them checked. This is the only way fixes enter the
// database, so that "is in the database" always means "all of them are".
func (l *Lookup) LoadIdents(idents []string) error {
	_, err := l.LoadIdentsCount(idents, true)
	return err
}

// LoadIdentsCount is LoadIdents that also says how many places the simulator knew. With markEmpty
// false, an answer with nothing in it is not recorded (the caller doubts the simulator).
func (l *Lookup) LoadIdentsCount(idents []string, markEmpty bool) (int, error) {
	if l.src == nil || len(idents) == 0 {
		return 0, nil
	}
	var found []navdata.Fix
	var err error
	l.timed("sim.lookupFixes", len(idents), func() { found, err = l.ask(idents) })
	if err != nil {
		return 0, err // not asked: nothing is marked, the next time asks again
	}
	if len(found) == 0 && !markEmpty {
		return 0, nil
	}
	var saveErr error
	l.timed("db.saveFixes", len(found), func() { saveErr = l.store.SaveFixes(found) })
	if saveErr != nil {
		return 0, saveErr
	}
	count := map[string]int{}
	for _, f := range found {
		count[strings.ToUpper(f.Ident)]++
	}
	var markErr error
	l.timed("db.markChecked", len(idents), func() {
		for _, id := range idents {
			if markErr = l.store.MarkIdentChecked(id, count[strings.ToUpper(id)]); markErr != nil {
				return
			}
		}
	})
	if markErr != nil {
		return 0, markErr
	}
	return len(found), nil
}
