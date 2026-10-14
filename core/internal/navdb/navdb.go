// Package navdb stores the navigation data of the simulator (package navdata) in a SQLite file:
// airports, runways, frequencies and procedures with their legs.
//
// It uses modernc.org/sqlite, a Go translation of SQLite, so the program stays free of cgo. The file
// is a cache: everything in it can be fetched again from the simulator, so a schema change is
// handled by starting over (see the version in the meta table).
package navdb

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// schemaVersion changes whenever the tables below do.
const schemaVersion = 3

const schema = `
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS airport (
  icao TEXT PRIMARY KEY, region TEXT, name TEXT, lat REAL, lon REAL, alt_m REAL,
  trans_alt_m REAL, trans_level REAL, fetched_at TEXT, source TEXT
);
CREATE INDEX IF NOT EXISTS airport_pos ON airport (lat, lon);
CREATE TABLE IF NOT EXISTS runway (
  id INTEGER PRIMARY KEY, airport TEXT NOT NULL REFERENCES airport(icao) ON DELETE CASCADE,
  primary_name TEXT, secondary_name TEXT, lat REAL, lon REAL, alt_m REAL,
  heading REAL, length_m REAL, width_m REAL, surface INTEGER
);
CREATE INDEX IF NOT EXISTS runway_airport ON runway (airport);
CREATE TABLE IF NOT EXISTS frequency (
  id INTEGER PRIMARY KEY, airport TEXT NOT NULL REFERENCES airport(icao) ON DELETE CASCADE,
  type INTEGER, hz INTEGER, name TEXT
);
CREATE INDEX IF NOT EXISTS frequency_airport ON frequency (airport);
CREATE TABLE IF NOT EXISTS procedure (
  id INTEGER PRIMARY KEY, airport TEXT NOT NULL REFERENCES airport(icao) ON DELETE CASCADE,
  kind TEXT, name TEXT, runway TEXT, approach_type INTEGER, suffix INTEGER, faf TEXT
);
CREATE INDEX IF NOT EXISTS procedure_airport ON procedure (airport);
CREATE TABLE IF NOT EXISTS transition (
  id INTEGER PRIMARY KEY, procedure_id INTEGER NOT NULL REFERENCES procedure(id) ON DELETE CASCADE,
  seq INTEGER, kind TEXT, name TEXT, runway TEXT
);
CREATE INDEX IF NOT EXISTS transition_proc ON transition (procedure_id);
CREATE TABLE IF NOT EXISTS leg (
  id INTEGER PRIMARY KEY, transition_id INTEGER NOT NULL REFERENCES transition(id) ON DELETE CASCADE,
  seq INTEGER, type INTEGER, fix_type TEXT, fix TEXT, fix_region TEXT, fly_over INTEGER,
  turn_left INTEGER, turn_right INTEGER, course REAL, distance REAL, alt_desc INTEGER,
  alt1_m REAL, alt2_m REAL, speed REAL, vertical REAL
);
CREATE INDEX IF NOT EXISTS leg_transition ON leg (transition_id);
CREATE INDEX IF NOT EXISTS leg_fix ON leg (fix, fix_region);
CREATE TABLE IF NOT EXISTS fix (
  ident TEXT NOT NULL, region TEXT NOT NULL, kind TEXT NOT NULL, lat REAL, lon REAL, alt_m REAL,
  freq_hz INTEGER, name TEXT, fetched_at TEXT, PRIMARY KEY (ident, region, kind)
);
CREATE INDEX IF NOT EXISTS fix_pos ON fix (lat, lon);
CREATE TABLE IF NOT EXISTS route (
  id INTEGER PRIMARY KEY, ident TEXT NOT NULL, region TEXT NOT NULL, kind TEXT NOT NULL,
  airway TEXT, type INTEGER,
  next_ident TEXT, next_region TEXT, next_kind TEXT, next_lat REAL, next_lon REAL,
  prev_ident TEXT, prev_region TEXT, prev_kind TEXT, prev_lat REAL, prev_lon REAL
);
CREATE INDEX IF NOT EXISTS route_fix ON route (ident, region);
CREATE INDEX IF NOT EXISTS route_airway ON route (airway);
CREATE TABLE IF NOT EXISTS ident_check (ident TEXT PRIMARY KEY, found INTEGER NOT NULL, checked_at TEXT);
`

// DB is the navigation database.
type DB struct {
	db *sql.DB
}

// Open opens (creating it when needed) the database file. A file with another schema version is
// emptied: it only holds data that can be fetched again.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	// one writer at a time is what SQLite does anyway; this keeps the pool simple
	db.SetMaxOpenConns(4)
	d := &DB{db: db}
	if err := d.init(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) init() error {
	if _, err := d.db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	var v string
	err := d.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema'`).Scan(&v)
	switch {
	case err == nil && v == fmt.Sprint(schemaVersion):
	case err == nil || errors.Is(err, sql.ErrNoRows):
		if err == nil { // another version: start over
			for _, t := range []string{"ident_check", "route", "fix", "leg", "transition", "procedure", "frequency", "runway", "airport"} {
				if _, e := d.db.Exec("DROP TABLE IF EXISTS " + t); e != nil {
					return e
				}
			}
		}
		if _, e := d.db.Exec(schema); e != nil {
			return e
		}
		if _, e := d.db.Exec(`INSERT OR REPLACE INTO meta (key, value) VALUES ('schema', ?)`, fmt.Sprint(schemaVersion)); e != nil {
			return e
		}
	default:
		return err
	}
	if _, err = d.db.Exec(schema); err != nil {
		return err
	}
	if _, err = d.db.Exec(extraSchema); err != nil {
		return err
	}
	return d.migrateUserPoints()
}

// Close closes the database.
func (d *DB) Close() error { return d.db.Close() }

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// SaveAirport stores an airport with everything under it, replacing what was there.
func (d *DB) SaveAirport(a *navdata.Airport) error {
	icao := strings.ToUpper(strings.TrimSpace(a.ICAO))
	if icao == "" {
		return errors.New("airport without an ICAO")
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM airport WHERE icao = ?`, icao); err != nil {
		return err
	}
	fetched := a.FetchedAt
	if fetched.IsZero() {
		fetched = time.Now()
	}
	if _, err := tx.Exec(`INSERT INTO airport (icao, region, name, lat, lon, alt_m, trans_alt_m, trans_level, fetched_at, source)
		VALUES (?,?,?,?,?,?,?,?,?,?)`, icao, a.Region, a.Name, a.Lat, a.Lon, a.AltitudeM, a.TransAltM, a.TransLevel,
		fetched.UTC().Format(time.RFC3339), a.Source); err != nil {
		return err
	}
	for _, r := range a.Runways {
		if _, err := tx.Exec(`INSERT INTO runway (airport, primary_name, secondary_name, lat, lon, alt_m, heading, length_m, width_m, surface)
			VALUES (?,?,?,?,?,?,?,?,?,?)`, icao, r.PrimaryName, r.SecondaryNam, r.Lat, r.Lon, r.AltitudeM, r.HeadingTrue, r.LengthM, r.WidthM, r.Surface); err != nil {
			return err
		}
	}
	for _, f := range a.Frequencies {
		if _, err := tx.Exec(`INSERT INTO frequency (airport, type, hz, name) VALUES (?,?,?,?)`, icao, f.Type, f.Hz, f.Name); err != nil {
			return err
		}
	}
	for _, p := range a.Procedures {
		res, err := tx.Exec(`INSERT INTO procedure (airport, kind, name, runway, approach_type, suffix, faf) VALUES (?,?,?,?,?,?,?)`,
			icao, p.Kind, p.Name, p.Runway, p.ApproachType, p.Suffix, p.FAF)
		if err != nil {
			return err
		}
		pid, _ := res.LastInsertId()
		for ti, t := range p.Transitions {
			res, err := tx.Exec(`INSERT INTO transition (procedure_id, seq, kind, name, runway) VALUES (?,?,?,?,?)`, pid, ti, t.Kind, t.Name, t.Runway)
			if err != nil {
				return err
			}
			tid, _ := res.LastInsertId()
			for li, l := range t.Legs {
				if _, err := tx.Exec(`INSERT INTO leg (transition_id, seq, type, fix_type, fix, fix_region, fly_over, turn_left, turn_right,
					course, distance, alt_desc, alt1_m, alt2_m, speed, vertical) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
					tid, li, l.Type, l.FixType, l.FixICAO, l.FixRegion, boolInt(l.FlyOver), boolInt(l.TurnLeft), boolInt(l.TurnRight),
					l.Course, l.DistanceNm, l.AltDesc, l.Altitude1M, l.Altitude2M, l.SpeedLimit, l.VerticalDeg); err != nil {
					return err
				}
			}
		}
	}
	return tx.Commit()
}

// ErrNotFound is returned for an airport that is not in the database.
var ErrNotFound = errors.New("airport not in the navigation database")

// Airport loads an airport with everything under it.
func (d *DB) Airport(icao string) (*navdata.Airport, error) {
	icao = strings.ToUpper(strings.TrimSpace(icao))
	a := &navdata.Airport{}
	var fetched string
	err := d.db.QueryRow(`SELECT icao, region, name, lat, lon, alt_m, trans_alt_m, trans_level, fetched_at, source FROM airport WHERE icao = ?`, icao).
		Scan(&a.ICAO, &a.Region, &a.Name, &a.Lat, &a.Lon, &a.AltitudeM, &a.TransAltM, &a.TransLevel, &fetched, &a.Source)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.FetchedAt, _ = time.Parse(time.RFC3339, fetched)

	rows, err := d.db.Query(`SELECT primary_name, secondary_name, lat, lon, alt_m, heading, length_m, width_m, surface FROM runway WHERE airport = ? ORDER BY id`, icao)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var r navdata.Runway
		if err := rows.Scan(&r.PrimaryName, &r.SecondaryNam, &r.Lat, &r.Lon, &r.AltitudeM, &r.HeadingTrue, &r.LengthM, &r.WidthM, &r.Surface); err != nil {
			rows.Close()
			return nil, err
		}
		a.Runways = append(a.Runways, r)
	}
	rows.Close()

	rows, err = d.db.Query(`SELECT type, hz, name FROM frequency WHERE airport = ? ORDER BY id`, icao)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var f navdata.Frequency
		if err := rows.Scan(&f.Type, &f.Hz, &f.Name); err != nil {
			rows.Close()
			return nil, err
		}
		a.Frequencies = append(a.Frequencies, f)
	}
	rows.Close()

	prows, err := d.db.Query(`SELECT id, kind, name, runway, approach_type, suffix, faf FROM procedure WHERE airport = ? ORDER BY id`, icao)
	if err != nil {
		return nil, err
	}
	type pid struct {
		id int64
		p  navdata.Procedure
	}
	var procs []pid
	for prows.Next() {
		var x pid
		if err := prows.Scan(&x.id, &x.p.Kind, &x.p.Name, &x.p.Runway, &x.p.ApproachType, &x.p.Suffix, &x.p.FAF); err != nil {
			prows.Close()
			return nil, err
		}
		procs = append(procs, x)
	}
	prows.Close()
	for _, x := range procs {
		trows, err := d.db.Query(`SELECT id, kind, name, runway FROM transition WHERE procedure_id = ? ORDER BY seq`, x.id)
		if err != nil {
			return nil, err
		}
		type tid struct {
			id int64
			t  navdata.Transition
		}
		var trs []tid
		for trows.Next() {
			var y tid
			if err := trows.Scan(&y.id, &y.t.Kind, &y.t.Name, &y.t.Runway); err != nil {
				trows.Close()
				return nil, err
			}
			trs = append(trs, y)
		}
		trows.Close()
		for _, y := range trs {
			lrows, err := d.db.Query(`SELECT type, fix_type, fix, fix_region, fly_over, turn_left, turn_right, course, distance, alt_desc, alt1_m, alt2_m, speed, vertical
				FROM leg WHERE transition_id = ? ORDER BY seq`, y.id)
			if err != nil {
				return nil, err
			}
			for lrows.Next() {
				var l navdata.Leg
				var fo, tl, tr int
				if err := lrows.Scan(&l.Type, &l.FixType, &l.FixICAO, &l.FixRegion, &fo, &tl, &tr, &l.Course, &l.DistanceNm, &l.AltDesc,
					&l.Altitude1M, &l.Altitude2M, &l.SpeedLimit, &l.VerticalDeg); err != nil {
					lrows.Close()
					return nil, err
				}
				l.FlyOver, l.TurnLeft, l.TurnRight = fo != 0, tl != 0, tr != 0
				y.t.Legs = append(y.t.Legs, l)
			}
			lrows.Close()
			x.p.Transitions = append(x.p.Transitions, y.t)
		}
		a.Procedures = append(a.Procedures, x.p)
	}
	if err := d.placeLegs(a); err != nil {
		return nil, err
	}
	return a, nil
}

// Summary is one line of the list of stored airports.
type Summary struct {
	ICAO      string    `json:"icao"`
	Name      string    `json:"name"`
	FetchedAt time.Time `json:"fetchedAt"`
}

// Airports lists the stored airports whose ICAO starts with the prefix.
func (d *DB) Airports(prefix string) ([]Summary, error) {
	rows, err := d.db.Query(`SELECT icao, name, fetched_at FROM airport WHERE icao LIKE ? ORDER BY icao`, strings.ToUpper(prefix)+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Summary
	for rows.Next() {
		var s Summary
		var f string
		if err := rows.Scan(&s.ICAO, &s.Name, &f); err != nil {
			return nil, err
		}
		s.FetchedAt, _ = time.Parse(time.RFC3339, f)
		out = append(out, s)
	}
	return out, rows.Err()
}
