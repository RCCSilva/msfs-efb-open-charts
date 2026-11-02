package navdb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// ---- aircraft profiles ----

// AircraftProfile is the performance profile of an aircraft the user keeps (see package perf). The
// store keeps the profile as JSON and does not look inside, except for the name.
type AircraftProfile struct {
	ID        int64           `json:"id"`
	Name      string          `json:"name"`
	Data      json.RawMessage `json:"data"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
}

const aircraftProfileSchema = `
CREATE TABLE IF NOT EXISTS aircraft_profile (
  id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL, data TEXT NOT NULL, created_at TEXT, updated_at TEXT
);
`

// migrateAircraftProfiles adds the aircraft profiles, and to a saved route the profile and the
// cruise altitude it is planned with.
func migrateAircraftProfiles(tx *sql.Tx) error {
	if _, err := tx.Exec(aircraftProfileSchema); err != nil {
		return err
	}
	return addColumns(tx, "saved_route", []column{{"profile_id", "INTEGER"}, {"cruise_ft", "INTEGER"}})
}

func cleanProfile(p *AircraftProfile) error {
	p.Name = strings.TrimSpace(p.Name)
	switch {
	case p.Name == "":
		return errors.Join(ErrInvalid, errors.New("the aircraft needs a name"))
	case len(p.Data) == 0 || len(p.Data) > 64<<10 || !json.Valid(p.Data):
		return errors.Join(ErrInvalid, errors.New("the profile is not valid"))
	}
	return nil
}

// AircraftProfiles lists the profiles by name.
func (d *DB) AircraftProfiles() ([]AircraftProfile, error) {
	rows, err := d.db.Query(`SELECT id, name, data, created_at, updated_at FROM aircraft_profile ORDER BY name COLLATE NOCASE, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AircraftProfile{}
	for rows.Next() {
		var p AircraftProfile
		var data string
		var c, u sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &data, &c, &u); err != nil {
			return nil, err
		}
		p.Data = json.RawMessage(data)
		p.CreatedAt, p.UpdatedAt = c.String, u.String
		out = append(out, p)
	}
	return out, rows.Err()
}

// AircraftProfile returns one profile.
func (d *DB) AircraftProfile(id int64) (AircraftProfile, error) {
	list, err := d.AircraftProfiles()
	if err != nil {
		return AircraftProfile{}, err
	}
	for _, p := range list {
		if p.ID == id {
			return p, nil
		}
	}
	return AircraftProfile{}, ErrNotFound
}

// CreateAircraftProfile stores a profile.
func (d *DB) CreateAircraftProfile(p AircraftProfile) (AircraftProfile, error) {
	if err := cleanProfile(&p); err != nil {
		return p, err
	}
	p.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	p.UpdatedAt = p.CreatedAt
	res, err := d.db.Exec(`INSERT INTO aircraft_profile (name, data, created_at, updated_at) VALUES (?,?,?,?)`, p.Name, string(p.Data), p.CreatedAt, p.UpdatedAt)
	if err != nil {
		return p, err
	}
	p.ID, _ = res.LastInsertId()
	return p, nil
}

// UpdateAircraftProfile replaces a profile.
func (d *DB) UpdateAircraftProfile(p AircraftProfile) (AircraftProfile, error) {
	if err := cleanProfile(&p); err != nil {
		return p, err
	}
	p.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	res, err := d.db.Exec(`UPDATE aircraft_profile SET name = ?, data = ?, updated_at = ? WHERE id = ?`, p.Name, string(p.Data), p.UpdatedAt, p.ID)
	if err != nil {
		return p, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return p, ErrNotFound
	}
	return p, nil
}

// DeleteAircraftProfile removes a profile. The routes that used it stay, with no aircraft.
func (d *DB) DeleteAircraftProfile(id int64) error {
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`DELETE FROM aircraft_profile WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(`UPDATE saved_route SET profile_id = NULL WHERE profile_id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}
