package navdb

import (
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// SaveFixes stores fixes and the airways through them, replacing what was there for the same fix.
func (d *DB) SaveFixes(fixes []navdata.Fix) error {
	if len(fixes) == 0 {
		return nil
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, f := range fixes {
		kind := string(rune(f.Kind))
		if _, err := tx.Exec(`DELETE FROM route WHERE ident = ? AND region = ? AND kind = ?`, f.Ident, f.Region, kind); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO fix (ident, region, kind, lat, lon, alt_m, freq_hz, name, fetched_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			f.Ident, f.Region, kind, f.Lat, f.Lon, f.AltM, f.FreqHz, f.Name, now); err != nil {
			return err
		}
		for _, r := range f.Routes {
			if _, err := tx.Exec(`INSERT INTO route (ident, region, kind, airway, type, next_ident, next_region, next_kind, next_lat, next_lon,
				prev_ident, prev_region, prev_kind, prev_lat, prev_lon) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
				f.Ident, f.Region, kind, r.Airway, r.Type, r.NextIdent, r.NextRegion, kindString(r.NextKind), r.NextLat, r.NextLon,
				r.PrevIdent, r.PrevRegion, kindString(r.PrevKind), r.PrevLat, r.PrevLon); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func kindString(k byte) string {
	if k == 0 {
		return ""
	}
	return string(rune(k))
}

// UncheckedIdents lists the identifiers of the waypoints, VORs and NDBs the procedures of an
// airport cite that were never asked for as a whole (see IdentChecked).
func (d *DB) UncheckedIdents(icao string) ([]string, error) {
	rows, err := d.db.Query(`SELECT DISTINCT l.fix FROM leg l
		JOIN transition t ON t.id = l.transition_id JOIN procedure p ON p.id = t.procedure_id
		WHERE p.airport = ? AND l.fix <> '' AND l.fix_type IN ('W','V','N')
		AND NOT EXISTS (SELECT 1 FROM ident_check c WHERE c.ident = l.fix)`,
		strings.ToUpper(icao))
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

// Fix returns a stored fix with its airways (the first kind found when an identifier is both a
// waypoint and a navaid in that region).
func (d *DB) Fix(ident, region string) (*navdata.Fix, error) {
	return d.fixOf(strings.ToUpper(ident), strings.ToUpper(region), "")
}

// fixOf loads one fix; an empty kind means any.
func (d *DB) fixOf(ident, region, kind string) (*navdata.Fix, error) {
	f := &navdata.Fix{}
	query := `SELECT ident, region, kind, lat, lon, alt_m, freq_hz, name FROM fix WHERE ident = ? AND region = ?`
	args := []any{ident, region}
	if kind != "" {
		query += ` AND kind = ?`
		args = append(args, kind)
	}
	query += ` ORDER BY kind LIMIT 1`
	err := d.db.QueryRow(query, args...).Scan(&f.Ident, &f.Region, &kind, &f.Lat, &f.Lon, &f.AltM, &f.FreqHz, &f.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(kind) == 1 {
		f.Kind = kind[0]
	}
	rows, err := d.db.Query(`SELECT airway, type, next_ident, next_region, next_kind, next_lat, next_lon, prev_ident, prev_region, prev_kind, prev_lat, prev_lon
		FROM route WHERE ident = ? AND region = ? AND kind = ? ORDER BY id`, f.Ident, f.Region, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r navdata.Route
		var nk, pk string
		if err := rows.Scan(&r.Airway, &r.Type, &r.NextIdent, &r.NextRegion, &nk, &r.NextLat, &r.NextLon,
			&r.PrevIdent, &r.PrevRegion, &pk, &r.PrevLat, &r.PrevLon); err != nil {
			return nil, err
		}
		if len(nk) == 1 {
			r.NextKind = nk[0]
		}
		if len(pk) == 1 {
			r.PrevKind = pk[0]
		}
		f.Routes = append(f.Routes, r)
	}
	return f, rows.Err()
}

// placeLegs fills the position of every leg whose fix is known: waypoints, VORs and NDBs from
// the fix table, runways and the airport itself from the airport's own data.
func (d *DB) placeLegs(a *navdata.Airport) error {
	rows, err := d.db.Query(`SELECT f.ident, f.region, f.kind, f.lat, f.lon FROM fix f WHERE EXISTS (
		SELECT 1 FROM leg l JOIN transition t ON t.id = l.transition_id JOIN procedure p ON p.id = t.procedure_id
		WHERE p.airport = ? AND l.fix = f.ident)`, a.ICAO)
	if err != nil {
		return err
	}
	type key struct{ ident, region, kind string }
	pos := map[key][2]float64{}
	only := map[[2]string][][2]float64{} // by identifier and kind, whatever the region
	for rows.Next() {
		var k key
		var lat, lon float64
		if err := rows.Scan(&k.ident, &k.region, &k.kind, &lat, &lon); err != nil {
			rows.Close()
			return err
		}
		pos[k] = [2]float64{lat, lon}
		only[[2]string{k.ident, k.kind}] = append(only[[2]string{k.ident, k.kind}], [2]float64{lat, lon})
	}
	rows.Close()

	for pi := range a.Procedures {
		for ti := range a.Procedures[pi].Transitions {
			legs := a.Procedures[pi].Transitions[ti].Legs
			for li := range legs {
				l := &legs[li]
				var p [2]float64
				ok := false
				switch l.FixType {
				case "R":
					p, ok = runwayThreshold(a, l.FixICAO)
				case "A":
					if strings.EqualFold(l.FixICAO, a.ICAO) {
						p, ok = [2]float64{a.Lat, a.Lon}, true
					}
				default:
					p, ok = pos[key{l.FixICAO, l.FixRegion, l.FixType}]
					if !ok {
						// stored without a region: it is this fix when the name is unique
						if c := only[[2]string{l.FixICAO, l.FixType}]; len(c) == 1 {
							p, ok = c[0], true
						}
					}
				}
				if ok {
					lat, lon := p[0], p[1]
					l.Lat, l.Lon = &lat, &lon
				}
			}
		}
	}
	return nil
}

// runwayThreshold finds the threshold of the runway a fix name like "RW34" or "RW16L" points at.
// The simulator gives the middle of a runway, its heading and length: the primary end is half a
// length behind the middle, the secondary end half a length ahead.
func runwayThreshold(a *navdata.Airport, name string) ([2]float64, bool) {
	n := strings.TrimPrefix(strings.ToUpper(name), "RW")
	for _, r := range a.Runways {
		sign := 0.0
		switch {
		case strings.EqualFold(r.PrimaryName, n):
			sign = -1
		case strings.EqualFold(r.SecondaryNam, n):
			sign = 1
		default:
			continue
		}
		h := r.HeadingTrue * math.Pi / 180
		half := r.LengthM / 2 * sign
		dLat := half * math.Cos(h) / 111320
		dLon := half * math.Sin(h) / (111320 * math.Cos(r.Lat*math.Pi/180))
		return [2]float64{r.Lat + dLat, r.Lon + dLon}, true
	}
	return [2]float64{}, false
}
