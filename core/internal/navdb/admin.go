package navdb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
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

// AirwaysPage is a page of the airways whose name starts with q, with how many segments each has,
// and how many airways match in all.
func (d *DB) AirwaysPage(q string, limit, offset int) ([]AirwayRow, int, error) {
	like := strings.ToUpper(strings.TrimSpace(q)) + "%"
	var total int
	if err := d.db.QueryRow(`SELECT COUNT(DISTINCT airway) FROM route WHERE airway <> '' AND airway LIKE ?`, like).Scan(&total); err != nil {
		return nil, 0, err
	}
	if limit <= 0 {
		limit = 25
	}
	rows, err := d.db.Query(`SELECT airway, COUNT(*) FROM route WHERE airway <> '' AND airway LIKE ? GROUP BY airway ORDER BY airway LIMIT ? OFFSET ?`,
		like, limit, max(offset, 0))
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []AirwayRow{}
	for rows.Next() {
		var r AirwayRow
		if err := rows.Scan(&r.Name, &r.Segments); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	return out, total, rows.Err()
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

// AirwaySegments lists the segments stored for one airway. A segment is stored from both of its ends
// (the fix before lists the next one, the fix after lists the previous one) and the simulator and
// its files do not always agree on which end says it, so both are read and a segment is listed once.
func (d *DB) AirwaySegments(name string) ([]AirwaySegment, error) {
	out := []AirwaySegment{}
	seen := map[[2]string]bool{}
	for _, q := range []string{
		`SELECT r.ident, r.next_ident, f.lat, f.lon, r.next_lat, r.next_lon FROM route r
			JOIN fix f ON f.ident = r.ident AND f.region = r.region AND f.kind = r.kind
			WHERE r.airway = ? AND r.next_ident <> '' AND NOT (r.next_lat = 0 AND r.next_lon = 0) ORDER BY r.ident`,
		`SELECT r.prev_ident, r.ident, r.prev_lat, r.prev_lon, f.lat, f.lon FROM route r
			JOIN fix f ON f.ident = r.ident AND f.region = r.region AND f.kind = r.kind
			WHERE r.airway = ? AND r.prev_ident <> '' AND NOT (r.prev_lat = 0 AND r.prev_lon = 0) ORDER BY r.ident`,
	} {
		rows, err := d.db.Query(q, strings.ToUpper(name))
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var s AirwaySegment
			if err := rows.Scan(&s.From, &s.To, &s.Lat, &s.Lon, &s.ToLat, &s.ToLon); err != nil {
				rows.Close()
				return nil, err
			}
			key := [2]string{s.From, s.To}
			if s.From > s.To {
				key = [2]string{s.To, s.From}
			}
			if !seen[key] {
				seen[key] = true
				out = append(out, s)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, err
		}
		rows.Close()
	}
	return out, nil
}

// Meta reads a small value kept with the database (empty when there is none).
func (d *DB) Meta(key string) string {
	var v string
	if err := d.db.QueryRow(`SELECT value FROM meta WHERE key = ?`, key).Scan(&v); err != nil {
		return ""
	}
	return v
}

// SetMeta stores a small value with the database.
func (d *DB) SetMeta(key, value string) error {
	_, err := d.db.Exec(`INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)`, key, value)
	return err
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
	list, _, err := d.UserPointsPage(q, 0, 0)
	return list, err
}

// UserPointsPage is a page of the points whose identifier starts with q (limit <= 0: all of them),
// and how many match in all.
func (d *DB) UserPointsPage(q string, limit, offset int) ([]UserPoint, int, error) {
	like := strings.ToUpper(strings.TrimSpace(q)) + "%"
	var total int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM user_point WHERE ident LIKE ?`, like).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT id, ident, name, lat, lon, alt_ft, type, tags, note, created_at FROM user_point WHERE ident LIKE ? ORDER BY ident, id`
	args := []any{like}
	if limit > 0 {
		query += ` LIMIT ? OFFSET ?`
		args = append(args, limit, max(offset, 0))
	}
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []UserPoint{}
	for rows.Next() {
		var p UserPoint
		var name, note, at, typ, tags sql.NullString
		var alt sql.NullFloat64
		if err := rows.Scan(&p.ID, &p.Ident, &name, &p.Lat, &p.Lon, &alt, &typ, &tags, &note, &at); err != nil {
			return nil, 0, err
		}
		p.Name, p.Note, p.CreatedAt, p.Type, p.Tags, p.AltFt = name.String, note.String, at.String, typ.String, tags.String, alt.Float64
		out = append(out, p)
	}
	return out, total, rows.Err()
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

// The classes of fix the simulator's files tell apart, as the file format numbers them. Measured
// against the running simulator: 2 are VORs, 3 are NDBs, and 5 and 6 are waypoints.
const (
	classVOR = 2
	classNDB = 3
)

// SeedIdent is an identifier found in the simulator's package files.
type SeedIdent struct {
	Ident string
	// Classes has one bit (1 << class) for each class of fix the files give this name: a name can be
	// a VOR and a waypoint of another region. 0 means unknown.
	Classes int
	Lat     float64
	Lon     float64
}

// UnplacedBit is set in the classes of a name when the files hold a fix of it that they cannot place: its
// region is not a code of two letters or digits (the terminal waypoints carry a number that only the
// simulator can turn into a region). Such a name is still asked to the simulator.
const UnplacedBit = 1 << 40

// KindsToAsk says which kinds of fix to ask the simulator for a name, so that it is not asked as all
// three when the files already tell. The files do not hold every fix of a name (a short name that is
// a VOR somewhere is often also a waypoint of a region they lack), so a short name, and one the files
// give as a VOR or an NDB, is still asked as all three; a longer name that the files give only as a
// waypoint is asked as a waypoint. Checked against the simulator on 6,000 names: nothing was missed.
func KindsToAsk(ident string, classes int) []byte {
	all := []byte{navdata.FixWaypoint, navdata.FixVOR, navdata.FixNDB}
	if classes == 0 || len(ident) <= 3 || classes&(1<<classVOR) != 0 || classes&(1<<classNDB) != 0 {
		return all
	}
	return all[:1]
}

// SaveSeed stores identifiers read from the files. A name already stored keeps what was asked about
// it, and has its classes brought up to date.
func (d *DB) SaveSeed(list []SeedIdent) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.Prepare(`INSERT INTO seed_ident (ident, kind, lat, lon, classes) VALUES (?,0,?,?,?)
		ON CONFLICT(ident) DO UPDATE SET classes = excluded.classes`)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, s := range list {
		if _, err := st.Exec(strings.ToUpper(s.Ident), s.Lat, s.Lon, s.Classes); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SeedKinds says, for the names the files gave, which kinds of fix to ask the simulator for each
// (see KindsToAsk). A name the files did not give is absent from the answer.
func (d *DB) SeedKinds(idents []string) (map[string][]byte, error) {
	out := map[string][]byte{}
	if len(idents) == 0 {
		return out, nil
	}
	args := make([]any, len(idents))
	for i, id := range idents {
		args[i] = strings.ToUpper(id)
	}
	rows, err := d.db.Query(`SELECT ident, classes FROM seed_ident WHERE ident IN (?`+strings.Repeat(",?", len(idents)-1)+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var classes int
		if err := rows.Scan(&id, &classes); err != nil {
			return nil, err
		}
		out[id] = KindsToAsk(id, classes)
	}
	return out, rows.Err()
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

// ---- saved routes ----

// SavedRoute is a route the user wrote down to use again: a name and the text of the route as a
// pilot writes it. It is resolved (names turned into places) every time it is used.
type SavedRoute struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Text string `json:"text"`
	// Choices holds the user's picks between places that share a name (a JSON array, see
	// route.Choice); the store does not look inside.
	Choices json.RawMessage `json:"choices"`
	// Result is the route as it was resolved when it was saved (route.Result as JSON): the places
	// with their positions. The store does not look inside. Whoever loads the route uses it as it
	// is, instead of resolving the text again.
	Result    json.RawMessage `json:"result,omitempty"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
}

const savedRouteSchema = `
CREATE TABLE IF NOT EXISTS saved_route (
  id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, text TEXT NOT NULL, created_at TEXT, updated_at TEXT
);
`

func cleanRoute(r *SavedRoute) error {
	r.Name = strings.TrimSpace(r.Name)
	r.Text = strings.Join(strings.Fields(r.Text), " ")
	switch {
	case r.Name == "":
		return errors.Join(ErrInvalid, errors.New("the route needs a name"))
	case len([]rune(r.Name)) > 80:
		return errors.Join(ErrInvalid, errors.New("the name is too long (80 characters at most)"))
	case r.Text == "":
		return errors.Join(ErrInvalid, errors.New("the route is empty"))
	case len(r.Text) > 4000:
		return errors.Join(ErrInvalid, errors.New("the route is too long"))
	}
	if len(r.Choices) == 0 || string(r.Choices) == "null" {
		r.Choices = json.RawMessage("[]")
	}
	var list []json.RawMessage
	if err := json.Unmarshal(r.Choices, &list); err != nil || len(list) > 200 || len(r.Choices) > 32<<10 {
		return errors.Join(ErrInvalid, errors.New("the choices are not a list"))
	}
	return nil
}

// SavedRoutes lists the saved routes, the most recently changed first.
func (d *DB) SavedRoutes() ([]SavedRoute, error) {
	rows, err := d.db.Query(`SELECT id, name, text, choices, result, created_at, updated_at FROM saved_route ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedRoute{}
	for rows.Next() {
		var r SavedRoute
		var c, u, ch, rs sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &r.Text, &ch, &rs, &c, &u); err != nil {
			return nil, err
		}
		if rs.Valid && rs.String != "" {
			r.Result = json.RawMessage(rs.String)
		}
		r.Choices = json.RawMessage("[]")
		if ch.Valid && ch.String != "" {
			r.Choices = json.RawMessage(ch.String)
		}
		r.CreatedAt, r.UpdatedAt = c.String, u.String
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateSavedRoute stores a route.
func (d *DB) CreateSavedRoute(r SavedRoute) (SavedRoute, error) {
	if err := cleanRoute(&r); err != nil {
		return r, err
	}
	r.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	r.UpdatedAt = r.CreatedAt
	res, err := d.db.Exec(`INSERT INTO saved_route (name, text, choices, result, created_at, updated_at) VALUES (?,?,?,?,?,?)`, r.Name, r.Text, string(r.Choices), string(r.Result), r.CreatedAt, r.UpdatedAt)
	if err != nil {
		return r, err
	}
	r.ID, _ = res.LastInsertId()
	return r, nil
}

// UpdateSavedRoute changes the name and the text of a route.
func (d *DB) UpdateSavedRoute(r SavedRoute) (SavedRoute, error) {
	if err := cleanRoute(&r); err != nil {
		return r, err
	}
	r.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	res, err := d.db.Exec(`UPDATE saved_route SET name = ?, text = ?, choices = ?, result = ?, updated_at = ? WHERE id = ?`, r.Name, r.Text, string(r.Choices), string(r.Result), r.UpdatedAt, r.ID)
	if err != nil {
		return r, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return r, ErrNotFound
	}
	return r, nil
}

// SavedRoute returns one saved route.
func (d *DB) SavedRoute(id int64) (SavedRoute, error) {
	list, err := d.SavedRoutes()
	if err != nil {
		return SavedRoute{}, err
	}
	for _, r := range list {
		if r.ID == id {
			return r, nil
		}
	}
	return SavedRoute{}, ErrNotFound
}

// SetSavedRouteResult keeps the resolved route of a saved route that did not have one (the ones
// saved by an earlier version).
func (d *DB) SetSavedRouteResult(id int64, result json.RawMessage) error {
	_, err := d.db.Exec(`UPDATE saved_route SET result = ? WHERE id = ?`, string(result), id)
	return err
}

// DeleteSavedRoute removes a route.
func (d *DB) DeleteSavedRoute(id int64) error {
	res, err := d.db.Exec(`DELETE FROM saved_route WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- what exists in an area of the map ----

// AreaAirport is an airport in an area.
type AreaAirport struct {
	ICAO string  `json:"icao"`
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

// AreaSegment is a stretch of an airway.
type AreaSegment struct {
	Airway string  `json:"airway"`
	From   string  `json:"from"`
	To     string  `json:"to"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
	ToLat  float64 `json:"toLat"`
	ToLon  float64 `json:"toLon"`
}

// Area is what is stored inside a rectangle of the map.
type Area struct {
	Airports    []AreaAirport `json:"airports"`
	Fixes       []FixRow      `json:"fixes"`
	Segments    []AreaSegment `json:"segments"`
	FixesCut    bool          `json:"fixesCut"`    // there are more fixes than were sent
	SegmentsCut bool          `json:"segmentsCut"` // there are more segments than were sent
	// What was left out because the view is too wide to show it (the caps above are for what is too
	// much inside a view that is not): zoom in to see it.
	AirportsHidden bool `json:"airportsHidden"`
	FixesHidden    bool `json:"fixesHidden"`
	SegmentsHidden bool `json:"segmentsHidden"`
}

// InArea returns the airports, the fixes and the airway segments inside the rectangle. Fixes and
// segments are capped (the area of a whole country holds a hundred thousand of them); the caps
// are reported so that the map can ask the user to zoom in.
func (d *DB) InArea(minLat, minLon, maxLat, maxLon float64, maxFixes, maxSegments int, withFixes, withSegments bool) (Area, error) {
	a := Area{Airports: []AreaAirport{}, Fixes: []FixRow{}, Segments: []AreaSegment{}}
	rows, err := d.db.Query(`SELECT icao, name, lat, lon FROM airport WHERE lat BETWEEN ? AND ? AND lon BETWEEN ? AND ? LIMIT 2000`,
		minLat, maxLat, minLon, maxLon)
	if err != nil {
		return a, err
	}
	for rows.Next() {
		var x AreaAirport
		var name sql.NullString
		if err := rows.Scan(&x.ICAO, &name, &x.Lat, &x.Lon); err != nil {
			rows.Close()
			return a, err
		}
		x.Name = name.String
		a.Airports = append(a.Airports, x)
	}
	rows.Close()

	if withFixes {
		rows, err := d.db.Query(`SELECT ident, region, kind, name, lat, lon FROM fix WHERE lat BETWEEN ? AND ? AND lon BETWEEN ? AND ? LIMIT ?`,
			minLat, maxLat, minLon, maxLon, maxFixes+1)
		if err != nil {
			return a, err
		}
		for rows.Next() {
			var x FixRow
			var name sql.NullString
			if err := rows.Scan(&x.Ident, &x.Region, &x.Kind, &name, &x.Lat, &x.Lon); err != nil {
				rows.Close()
				return a, err
			}
			x.Name = name.String
			a.Fixes = append(a.Fixes, x)
		}
		rows.Close()
		if len(a.Fixes) > maxFixes {
			a.Fixes, a.FixesCut = a.Fixes[:maxFixes], true
		}
	}
	if withSegments {
		// A segment is stored twice: on its first fix (as "next") and on its last (as "prev"). Both
		// are used, so that a segment shows when only one of its ends is known to the database: an
		// airway is not broken where a fix was never loaded. A wider rectangle catches the
		// segments that cross the view with both ends outside it.
		padLat, padLon := (maxLat-minLat)*0.25, (maxLon-minLon)*0.25
		args := []any{minLat - padLat, maxLat + padLat, minLon - padLon, maxLon + padLon}
		seen := map[[5]string]bool{}
		for _, q := range []string{
			`SELECT r.airway, r.ident, r.next_ident, f.lat, f.lon, r.next_lat, r.next_lon
				FROM fix f JOIN route r ON r.ident = f.ident AND r.region = f.region AND r.kind = f.kind
				WHERE f.lat BETWEEN ? AND ? AND f.lon BETWEEN ? AND ? AND r.next_ident <> '' AND r.airway <> '' AND NOT (r.next_lat = 0 AND r.next_lon = 0)`,
			`SELECT r.airway, r.prev_ident, r.ident, r.prev_lat, r.prev_lon, f.lat, f.lon
				FROM fix f JOIN route r ON r.ident = f.ident AND r.region = f.region AND r.kind = f.kind
				WHERE f.lat BETWEEN ? AND ? AND f.lon BETWEEN ? AND ? AND r.prev_ident <> '' AND r.airway <> '' AND NOT (r.prev_lat = 0 AND r.prev_lon = 0)`,
		} {
			rows, err := d.db.Query(q, args...)
			if err != nil {
				return a, err
			}
			for rows.Next() {
				var s AreaSegment
				if err := rows.Scan(&s.Airway, &s.From, &s.To, &s.Lat, &s.Lon, &s.ToLat, &s.ToLon); err != nil {
					rows.Close()
					return a, err
				}
				if s.Lat == 0 && s.Lon == 0 || s.ToLat == 0 && s.ToLon == 0 {
					continue // an end whose position is not known
				}
				key := [5]string{s.Airway, strconv.FormatFloat(s.Lat, 'f', 3, 64), strconv.FormatFloat(s.Lon, 'f', 3, 64),
					strconv.FormatFloat(s.ToLat, 'f', 3, 64), strconv.FormatFloat(s.ToLon, 'f', 3, 64)}
				if seen[key] {
					continue
				}
				seen[key] = true
				if len(a.Segments) >= maxSegments {
					a.SegmentsCut = true
					continue
				}
				a.Segments = append(a.Segments, s)
			}
			rows.Close()
		}
	}
	return a, nil
}

// MissingNeighbors lists up to limit identifiers that airways cite as the next or previous fix of
// a stored fix, that are not stored and were never asked for as a whole. Asking for them closes
// the airways: every segment then has both of its ends.
func (d *DB) MissingNeighbors(limit int) ([]string, error) {
	rows, err := d.db.Query(`SELECT ident FROM (
			SELECT DISTINCT next_ident AS ident FROM route WHERE next_ident <> ''
			UNION
			SELECT DISTINCT prev_ident AS ident FROM route WHERE prev_ident <> ''
		) n WHERE NOT EXISTS (SELECT 1 FROM fix f WHERE f.ident = n.ident)
		AND NOT EXISTS (SELECT 1 FROM ident_check c WHERE c.ident = n.ident)
		ORDER BY ident LIMIT ?`, limit)
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

// ---- search ----

// SearchHit is one place found by a search.
type SearchHit struct {
	Type   string  `json:"type"` // airport, fix, point
	Ident  string  `json:"ident"`
	Region string  `json:"region"`
	Kind   string  `json:"kind"` // W, V, N for fixes
	Name   string  `json:"name"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

// Search finds airports, fixes and the user's points by identifier (starts with) or by name
// (contains), identifiers first.
func (d *DB) Search(q string, limit int) ([]SearchHit, error) {
	q = strings.TrimSpace(q)
	if len([]rune(q)) < 2 {
		return []SearchHit{}, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	up := strings.ToUpper(q)
	prefix := up + "%"
	contains := "%" + up + "%"
	out := []SearchHit{}
	add := func(rows *sql.Rows, typ string) error {
		defer rows.Close()
		for rows.Next() {
			h := SearchHit{Type: typ}
			var name, region, kind sql.NullString
			if err := rows.Scan(&h.Ident, &region, &kind, &name, &h.Lat, &h.Lon); err != nil {
				return err
			}
			h.Region, h.Kind, h.Name = region.String, kind.String, name.String
			out = append(out, h)
		}
		return rows.Err()
	}
	rows, err := d.db.Query(`SELECT icao, region, '', name, lat, lon FROM airport
		WHERE icao LIKE ? OR UPPER(name) LIKE ? ORDER BY (icao = ?) DESC, (icao LIKE ?) DESC, icao LIMIT ?`, prefix, contains, up, prefix, limit)
	if err != nil {
		return nil, err
	}
	if err := add(rows, "airport"); err != nil {
		return nil, err
	}
	rows, err = d.db.Query(`SELECT ident, '', '', name, lat, lon FROM user_point
		WHERE ident LIKE ? OR UPPER(name) LIKE ? ORDER BY (ident = ?) DESC, ident LIMIT ?`, prefix, contains, up, limit)
	if err != nil {
		return nil, err
	}
	if err := add(rows, "point"); err != nil {
		return nil, err
	}
	rows, err = d.db.Query(`SELECT ident, region, kind, name, lat, lon FROM fix
		WHERE ident LIKE ? OR UPPER(name) LIKE ? ORDER BY (ident = ?) DESC, (ident LIKE ?) DESC, ident LIMIT ?`, prefix, contains, up, prefix, limit)
	if err != nil {
		return nil, err
	}
	if err := add(rows, "fix"); err != nil {
		return nil, err
	}
	// the exact identifier first, then the ones that start with it, then the ones with it in the name
	rank := func(h SearchHit) int {
		switch {
		case strings.EqualFold(h.Ident, q):
			return 0
		case strings.HasPrefix(strings.ToUpper(h.Ident), up):
			return 1
		}
		return 2
	}
	sort.SliceStable(out, func(i, j int) bool { return rank(out[i]) < rank(out[j]) })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
