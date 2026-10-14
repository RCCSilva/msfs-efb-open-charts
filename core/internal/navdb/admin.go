package navdb

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// What is in this file serves the "Navigation" page of the manager: counting, listing, deleting what
// is stored, the points the user makes, and the identifiers read from the simulator's files.

// extraSchema is created whenever the database opens and is never dropped by a schema change:
// user_point is the user's own data, and seed_ident only grows.
const extraSchema = `
CREATE TABLE IF NOT EXISTS user_point (
  id INTEGER PRIMARY KEY AUTOINCREMENT, ident TEXT NOT NULL, name TEXT, lat REAL NOT NULL, lon REAL NOT NULL,
  note TEXT, created_at TEXT
);
CREATE INDEX IF NOT EXISTS user_point_ident ON user_point (ident);
CREATE TABLE IF NOT EXISTS seed_ident (ident TEXT PRIMARY KEY, kind INTEGER, lat REAL, lon REAL);
`

// UserRegion is the region the points made by the user are given in a route.
const UserRegion = "USR"

// Counts is what the database holds.
type Counts struct {
	Airports      int `json:"airports"`
	Runways       int `json:"runways"`
	Procedures    int `json:"procedures"`
	Waypoints     int `json:"waypoints"`
	VORs          int `json:"vors"`
	NDBs          int `json:"ndbs"`
	Airways       int `json:"airways"`
	Segments      int `json:"segments"`
	IdentsChecked int `json:"identsChecked"`
	IdentsEmpty   int `json:"identsEmpty"` // asked as a whole and the simulator knew nothing by that name
	UserPoints    int `json:"userPoints"`
	SeedIdents    int `json:"seedIdents"`
	SeedPending   int `json:"seedPending"` // read from the files and not asked to the simulator yet
}

// Counts counts what is stored.
func (d *DB) Counts() (Counts, error) {
	var c Counts
	one := func(dst *int, q string) error { return d.db.QueryRow(q).Scan(dst) }
	for _, it := range []struct {
		dst *int
		q   string
	}{
		{&c.Airports, `SELECT COUNT(*) FROM airport`},
		{&c.Runways, `SELECT COUNT(*) FROM runway`},
		{&c.Procedures, `SELECT COUNT(*) FROM procedure`},
		{&c.Waypoints, `SELECT COUNT(*) FROM fix WHERE kind = 'W'`},
		{&c.VORs, `SELECT COUNT(*) FROM fix WHERE kind = 'V'`},
		{&c.NDBs, `SELECT COUNT(*) FROM fix WHERE kind = 'N'`},
		{&c.Airways, `SELECT COUNT(DISTINCT airway) FROM route WHERE airway <> ''`},
		{&c.Segments, `SELECT COUNT(*) FROM route`},
		{&c.IdentsChecked, `SELECT COUNT(*) FROM ident_check`},
		{&c.IdentsEmpty, `SELECT COUNT(*) FROM ident_check WHERE found = 0`},
		{&c.UserPoints, `SELECT COUNT(*) FROM user_point`},
		{&c.SeedIdents, `SELECT COUNT(*) FROM seed_ident`},
		{&c.SeedPending, `SELECT COUNT(*) FROM seed_ident s WHERE NOT EXISTS (SELECT 1 FROM ident_check c WHERE c.ident = s.ident)`},
	} {
		if err := one(it.dst, it.q); err != nil {
			return c, err
		}
	}
	return c, nil
}

// FixRow is one line of the list of stored fixes.
type FixRow struct {
	Ident  string  `json:"ident"`
	Region string  `json:"region"`
	Kind   string  `json:"kind"` // W, V or N
	Name   string  `json:"name"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	FreqHz int     `json:"freqHz,omitempty"`
	Routes int     `json:"routes"` // airway segments through it
}

// ListFixes lists stored fixes whose identifier starts with q (any when empty), of one kind or all.
func (d *DB) ListFixes(q, kind string, limit, offset int) ([]FixRow, int, error) {
	where := `WHERE 1=1`
	var args []any
	if q = strings.ToUpper(strings.TrimSpace(q)); q != "" {
		where += ` AND f.ident LIKE ?`
		args = append(args, q+"%")
	}
	if kind = strings.ToUpper(kind); kind == "W" || kind == "V" || kind == "N" {
		where += ` AND f.kind = ?`
		args = append(args, kind)
	}
	var total int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM fix f `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := d.db.Query(`SELECT f.ident, f.region, f.kind, f.name, f.lat, f.lon, f.freq_hz,
		(SELECT COUNT(*) FROM route r WHERE r.ident = f.ident AND r.region = f.region AND r.kind = f.kind)
		FROM fix f `+where+` ORDER BY f.ident, f.region, f.kind LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []FixRow{}
	for rows.Next() {
		var r FixRow
		if err := rows.Scan(&r.Ident, &r.Region, &r.Kind, &r.Name, &r.Lat, &r.Lon, &r.FreqHz, &r.Routes); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// DeleteFix removes a stored fix and the airway segments through it. The identifier is no longer
// "asked as a whole", so the next route that cites it asks the simulator again.
func (d *DB) DeleteFix(ident, region, kind string) error {
	ident, region, kind = strings.ToUpper(ident), strings.ToUpper(region), strings.ToUpper(kind)
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM route WHERE ident = ? AND region = ? AND kind = ?`, ident, region, kind); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM fix WHERE ident = ? AND region = ? AND kind = ?`, ident, region, kind)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(`DELETE FROM ident_check WHERE ident = ?`, ident); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteAirport removes a stored airport with its runways, frequencies and procedures.
func (d *DB) DeleteAirport(icao string) error {
	res, err := d.db.Exec(`DELETE FROM airport WHERE icao = ?`, strings.ToUpper(icao))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// AirwayRow is one airway with how many segments of it are stored.
type AirwayRow struct {
	Name     string `json:"name"`
	Segments int    `json:"segments"`
}

// Airways lists the stored airways whose name starts with q.
func (d *DB) Airways(q string, limit int) ([]AirwayRow, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := d.db.Query(`SELECT airway, COUNT(*) FROM route WHERE airway <> '' AND airway LIKE ? GROUP BY airway ORDER BY airway LIMIT ?`,
		strings.ToUpper(strings.TrimSpace(q))+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AirwayRow{}
	for rows.Next() {
		var r AirwayRow
		if err := rows.Scan(&r.Name, &r.Segments); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// AirwaySegment is one stretch of an airway: from one fix to the next.
type AirwaySegment struct {
	From  string  `json:"from"`
	To    string  `json:"to"`
	Lat   float64 `json:"lat"`
	Lon   float64 `json:"lon"`
	ToLat float64 `json:"toLat"`
	ToLon float64 `json:"toLon"`
}

// AirwaySegments lists the segments stored for one airway (each fix lists the stretch that follows it).
func (d *DB) AirwaySegments(name string) ([]AirwaySegment, error) {
	rows, err := d.db.Query(`SELECT r.ident, r.next_ident, f.lat, f.lon, r.next_lat, r.next_lon FROM route r
		JOIN fix f ON f.ident = r.ident AND f.region = r.region AND f.kind = r.kind
		WHERE r.airway = ? AND r.next_ident <> '' ORDER BY r.ident`, strings.ToUpper(name))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AirwaySegment{}
	for rows.Next() {
		var s AirwaySegment
		if err := rows.Scan(&s.From, &s.To, &s.Lat, &s.Lon, &s.ToLat, &s.ToLon); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ---- points made by the user ----

// UserPoint is a place the user made: it can be cited in a route like any waypoint.
type UserPoint struct {
	ID        int64   `json:"id"`
	Ident     string  `json:"ident"`
	Name      string  `json:"name"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	AltFt     float64 `json:"altFt"`
	Type      string  `json:"type"` // Waypoint, Airport, Marker... (what a Little Navmap export says)
	Tags      string  `json:"tags"`
	Note      string  `json:"note"`
	CreatedAt string  `json:"createdAt"`
}

// ErrInvalid is returned for a user point that cannot be stored.
var ErrInvalid = errors.New("invalid point")

func cleanPoint(p *UserPoint) error {
	p.Ident = strings.ToUpper(strings.TrimSpace(p.Ident))
	if len(p.Ident) < 2 || len(p.Ident) > 12 {
		return errors.Join(ErrInvalid, errors.New("the identifier must have 2 to 12 characters"))
	}
	for _, c := range p.Ident {
		if !(c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return errors.Join(ErrInvalid, errors.New("the identifier takes letters and digits only"))
		}
	}
	if p.Lat < -90 || p.Lat > 90 || p.Lon < -180 || p.Lon > 180 {
		return errors.Join(ErrInvalid, errors.New("latitude or longitude out of range"))
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Note = strings.TrimSpace(p.Note)
	p.Type = strings.TrimSpace(p.Type)
	p.Tags = strings.TrimSpace(p.Tags)
	if p.Type == "" {
		p.Type = "Waypoint"
	}
	return nil
}

// UserPoints lists the user's points whose identifier starts with q.
func (d *DB) UserPoints(q string) ([]UserPoint, error) {
	rows, err := d.db.Query(`SELECT id, ident, name, lat, lon, alt_ft, type, tags, note, created_at FROM user_point WHERE ident LIKE ? ORDER BY ident, id`,
		strings.ToUpper(strings.TrimSpace(q))+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserPoint{}
	for rows.Next() {
		var p UserPoint
		var name, note, at, typ, tags sql.NullString
		var alt sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.Ident, &name, &p.Lat, &p.Lon, &alt, &typ, &tags, &note, &at); err != nil {
			return nil, err
		}
		p.Name, p.Note, p.CreatedAt, p.Type, p.Tags, p.AltFt = name.String, note.String, at.String, typ.String, tags.String, alt.Float64
		out = append(out, p)
	}
	return out, rows.Err()
}

// CreateUserPoint stores a point.
func (d *DB) CreateUserPoint(p UserPoint) (UserPoint, error) {
	if err := cleanPoint(&p); err != nil {
		return p, err
	}
	p.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	res, err := d.db.Exec(`INSERT INTO user_point (ident, name, lat, lon, alt_ft, type, tags, note, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		p.Ident, p.Name, p.Lat, p.Lon, p.AltFt, p.Type, p.Tags, p.Note, p.CreatedAt)
	if err != nil {
		return p, err
	}
	p.ID, _ = res.LastInsertId()
	return p, nil
}

// UpdateUserPoint changes a point.
func (d *DB) UpdateUserPoint(p UserPoint) error {
	if err := cleanPoint(&p); err != nil {
		return err
	}
	res, err := d.db.Exec(`UPDATE user_point SET ident = ?, name = ?, lat = ?, lon = ?, alt_ft = ?, type = ?, tags = ?, note = ? WHERE id = ?`,
		p.Ident, p.Name, p.Lat, p.Lon, p.AltFt, p.Type, p.Tags, p.Note, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUserPoint removes a point.
func (d *DB) DeleteUserPoint(id int64) error {
	res, err := d.db.Exec(`DELETE FROM user_point WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UserFixes returns the user's points with that identifier as fixes (waypoints in the region
// UserRegion), so that the route resolver treats them like any other waypoint.
func (d *DB) UserFixes(ident string) ([]navdata.Fix, error) {
	list, err := d.UserPoints("")
	if err != nil {
		return nil, err
	}
	ident = strings.ToUpper(ident)
	var out []navdata.Fix
	for _, p := range list {
		if p.Ident == ident {
			out = append(out, navdata.Fix{Ident: p.Ident, Region: UserRegion, Kind: navdata.FixWaypoint, Lat: p.Lat, Lon: p.Lon, Name: p.Name})
		}
	}
	return out, nil
}

// ---- identifiers read from the simulator's files ----

// SeedIdent is an identifier found in the simulator's package files.
type SeedIdent struct {
	Ident string
	Kind  int
	Lat   float64
	Lon   float64
}

// SaveSeed stores identifiers read from the files (the ones already stored are kept).
func (d *DB) SaveSeed(list []SeedIdent) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.Prepare(`INSERT OR IGNORE INTO seed_ident (ident, kind, lat, lon) VALUES (?,?,?,?)`)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, s := range list {
		if _, err := st.Exec(strings.ToUpper(s.Ident), s.Kind, s.Lat, s.Lon); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SeedPending lists up to limit identifiers read from the files that were not asked for as a
// whole yet, in alphabetical order.
func (d *DB) SeedPending(limit int) ([]string, error) {
	rows, err := d.db.Query(`SELECT s.ident FROM seed_ident s WHERE NOT EXISTS (SELECT 1 FROM ident_check c WHERE c.ident = s.ident)
		ORDER BY s.ident LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// migrateUserPoints adds the columns a table made by an earlier version does not have.
func (d *DB) migrateUserPoints() error {
	rows, err := d.db.Query(`PRAGMA table_info(user_point)`)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for rows.Next() {
		var cid, notnull, pk int
		var name, typ string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return err
		}
		have[name] = true
	}
	rows.Close()
	for _, c := range []struct{ name, def string }{{"alt_ft", "REAL"}, {"type", "TEXT"}, {"tags", "TEXT"}} {
		if !have[c.name] {
			if _, err := d.db.Exec(`ALTER TABLE user_point ADD COLUMN ` + c.name + ` ` + c.def); err != nil {
				return err
			}
		}
	}
	return nil
}

// ImportResult says what an import did.
type ImportResult struct {
	Added   int `json:"added"`
	Skipped int `json:"skipped"` // already there (same name, same place)
}

// identFromName makes an identifier out of a name: no accents, letters and digits only, at most 12.
func identFromName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if m, ok := accentFold[r]; ok {
			r = m
		}
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var accentFold = func() map[rune]rune {
	m := map[rune]rune{}
	for folded, from := range map[rune]string{
		'A': "ÁÀÂÃÄÅ", 'E': "ÉÈÊË", 'I': "ÍÌÎÏ", 'O': "ÓÒÔÕÖ", 'U': "ÚÙÛÜ", 'C': "Ç", 'N': "Ñ", 'Y': "Ý",
	} {
		for _, r := range from {
			m[r] = folded
		}
	}
	return m
}()

// ImportUserPoints stores points that came from a file. A point without a usable identifier gets
// one made from its name (and a number when that is taken). A point with the same name and place
// as one already stored is skipped, so importing the same file twice changes nothing.
func (d *DB) ImportUserPoints(list []UserPoint) (ImportResult, error) {
	var res ImportResult
	tx, err := d.db.Begin()
	if err != nil {
		return res, err
	}
	defer tx.Rollback()
	type key struct {
		name     string
		lat, lon int64
	}
	pos := func(v float64) int64 { return int64(v*100000 + 0.5*sign(v)) }
	haveKey := map[key]bool{}
	taken := map[string]bool{}
	rows, err := tx.Query(`SELECT ident, name, lat, lon FROM user_point`)
	if err != nil {
		return res, err
	}
	for rows.Next() {
		var id, name string
		var lat, lon float64
		if err := rows.Scan(&id, &name, &lat, &lon); err != nil {
			rows.Close()
			return res, err
		}
		taken[id] = true
		haveKey[key{strings.ToUpper(name), pos(lat), pos(lon)}] = true
	}
	rows.Close()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, p := range list {
		id := identFromName(p.Ident)
		if len(id) < 2 {
			id = identFromName(p.Name)
		}
		if len(id) < 2 {
			res.Skipped++
			continue
		}
		if len(id) > 12 {
			id = id[:12]
		}
		p.Ident = id
		if err := cleanPoint(&p); err != nil {
			res.Skipped++
			continue
		}
		k := key{strings.ToUpper(p.Name), pos(p.Lat), pos(p.Lon)}
		if haveKey[k] {
			res.Skipped++
			continue
		}
		for n := 2; taken[p.Ident]; n++ {
			suffix := strconv.Itoa(n)
			base := id
			if len(base)+len(suffix) > 12 {
				base = base[:12-len(suffix)]
			}
			p.Ident = base + suffix
		}
		if _, err := tx.Exec(`INSERT INTO user_point (ident, name, lat, lon, alt_ft, type, tags, note, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			p.Ident, p.Name, p.Lat, p.Lon, p.AltFt, p.Type, p.Tags, p.Note, now); err != nil {
			return res, err
		}
		taken[p.Ident] = true
		haveKey[k] = true
		res.Added++
	}
	return res, tx.Commit()
}

func sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}
