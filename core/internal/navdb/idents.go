package navdb

import (
	"strings"
	"time"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
)

// An identifier is only unique together with its region, so having one fix called RBV says
// nothing about the others. The database therefore keeps a list of the identifiers that were asked
// for as a whole (without a region: the simulator then answers with every place of that name).
// For those, and only those, what is stored is everything there is, and an empty answer means that
// nothing by that name exists.

// IdentChecked tells whether the identifier was asked for as a whole.
func (d *DB) IdentChecked(ident string) (bool, error) {
	var n int
	err := d.db.QueryRow(`SELECT COUNT(*) FROM ident_check WHERE ident = ?`, strings.ToUpper(ident)).Scan(&n)
	return n > 0, err
}

// MarkIdentChecked records that the identifier was asked for as a whole, with how many places the
// simulator knew by that name.
func (d *DB) MarkIdentChecked(ident string, found int) error {
	_, err := d.db.Exec(`INSERT OR REPLACE INTO ident_check (ident, found, checked_at) VALUES (?,?,?)`,
		strings.ToUpper(ident), found, time.Now().UTC().Format(time.RFC3339))
	return err
}

// FixesByIdent returns every stored fix with that identifier (any region, any kind) with its airways.
func (d *DB) FixesByIdent(ident string) ([]navdata.Fix, error) {
	rows, err := d.db.Query(`SELECT ident, region, kind FROM fix WHERE ident = ? ORDER BY region, kind`, strings.ToUpper(ident))
	if err != nil {
		return nil, err
	}
	type key struct{ ident, region, kind string }
	var keys []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.ident, &k.region, &k.kind); err != nil {
			rows.Close()
			return nil, err
		}
		keys = append(keys, k)
	}
	rows.Close()
	out := make([]navdata.Fix, 0, len(keys))
	for _, k := range keys {
		f, err := d.fixOf(k.ident, k.region, k.kind)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, nil
}

// FixAt returns one stored fix by identifier, region and kind.
//
// An identifier that exists only once is stored without a region (the simulator does not say it).
// When a leg or an airway cites such a fix with its region, that is the fix, and the region is
// recorded from then on.
func (d *DB) FixAt(ref navdata.FixRef) (*navdata.Fix, error) {
	ident, region, kind := strings.ToUpper(ref.Ident), strings.ToUpper(ref.Region), string(rune(ref.Kind))
	f, err := d.fixOf(ident, region, kind)
	if err == nil {
		return f, nil
	}
	if err != ErrNotFound {
		return nil, err
	}
	if region == "" {
		return nil, nil
	}
	var n int
	if err := d.db.QueryRow(`SELECT COUNT(*) FROM fix WHERE ident = ? AND kind = ?`, ident, kind).Scan(&n); err != nil {
		return nil, err
	}
	if n != 1 {
		return nil, nil
	}
	f, err = d.fixOf(ident, "", kind)
	if err != nil {
		return nil, nil
	}
	if _, err := d.db.Exec(`UPDATE fix SET region = ? WHERE ident = ? AND region = '' AND kind = ?`, region, ident, kind); err == nil {
		_, _ = d.db.Exec(`UPDATE route SET region = ? WHERE ident = ? AND region = '' AND kind = ?`, region, ident, kind)
		f.Region = region
	}
	return f, nil
}
