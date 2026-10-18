package navdb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Migrations: how the schema of the file changes without the user losing what is in it.
//
// The version of the schema is the number in the file's own header (PRAGMA user_version). Every
// change is a migration with the next number, listed below in order; opening the file runs the ones
// the file has not had, each in a transaction (a failure undoes it and leaves the file as it was).
// Before the first of them runs on a file that has data, a copy of the file is made in the
// "backups" folder next to it (see backup.go; the newest few are kept).
//
// Rules, because users keep what is in this file:
//   - Never edit or reorder a migration that was released: add a new one at the end.
//   - Never drop or empty a table of the user's (user_point, saved_route): change it by adding
//     columns, or by copying the rows into the new table in the same migration.
//   - What is fetched from the simulator (airports, fixes...) may be rebuilt, but prefer to convert it:
//     fetching everything again takes minutes and needs the simulator running.
//   - A file with a newer schema than the program knows is not opened (a newer program made it; an
//     older one must not change it): ErrNewerSchema.
//   - A migration that recreates a table other tables refer to sets rebuild: foreign keys are off
//     while it runs and are checked before it commits.

// ErrNewerSchema is returned for a file made by a newer version of the program.
var ErrNewerSchema = errors.New("the navigation database was made by a newer version of Open Charts: update the program (the file was not changed)")

// keepBackups is how many copies made before a migration are kept.
const keepBackups = 3

// baselineSchema is the version of the meta table's "schema" row in the releases that came before
// the versions in the file header (0.3 and 0.4). A file with a different one is from an unreleased
// development version.
const baselineSchema = 3

type migration struct {
	version int
	name    string
	rebuild bool // recreates tables other tables refer to: see the rules above
	up      func(tx *sql.Tx) error
}

// migrations is every change of the schema, in order.
var migrations = []migration{
	{version: 1, name: "baseline: the schema of 0.3 and 0.4", up: migrateBaseline},
	{version: 2, name: "seed identifiers keep the classes of their fixes", up: migrateSeedClasses},
	{version: 3, name: "fixes say where they came from", up: migrateFixSource},
}

// migrateFixSource adds where a fix came from: the simulator (every fix stored until now) or the
// simulator's own files (see SaveFileFixes).
func migrateFixSource(tx *sql.Tx) error {
	return addColumns(tx, "fix", []column{{"source", "TEXT NOT NULL DEFAULT 'sim'"}})
}

// migrateSeedClasses adds the classes the simulator's files give each name. The names already stored
// have 0 (unknown), which is asked the way it always was; reading the files again fills them in.
func migrateSeedClasses(tx *sql.Tx) error {
	return addColumns(tx, "seed_ident", []column{{"classes", "INTEGER NOT NULL DEFAULT 0"}})
}

// Migration says what Open did to bring the file up to date. From == To when nothing was needed.
type Migration struct {
	From, To int
	Backup   string // the copy made before, when there was one
}

// Migrated says what Open did.
func (d *DB) Migrated() Migration { return d.migrated }

// SchemaVersion is the version of the schema of the open file.
func (d *DB) SchemaVersion() (int, error) {
	var v int
	err := d.db.QueryRow(`PRAGMA user_version`).Scan(&v)
	return v, err
}

func latestVersion() int { return migrations[len(migrations)-1].version }

// LatestSchema is the schema version this program writes.
func LatestSchema() int { return latestVersion() }

// cacheTables are what is fetched from the simulator, dropped only for a development schema.
var cacheTables = []string{"ident_check", "route", "fix", "leg", "transition", "procedure", "frequency", "runway", "airport"}

func (d *DB) migrate() error {
	// a damaged file is found before anything is done to it
	if err := d.checkHealth(); err != nil {
		return err
	}
	current, err := d.SchemaVersion()
	if err != nil {
		return corruptOr(err)
	}
	latest := latestVersion()
	if current > latest {
		return fmt.Errorf("%w (file: schema %d, this program: %d)", ErrNewerSchema, current, latest)
	}
	d.migrated = Migration{From: current, To: current}
	if current == latest {
		return nil
	}
	// a copy first, when there is something to lose: a file that already had versions, or one from
	// before them that the baseline would empty (an unreleased schema)
	if current >= 1 || d.legacyResets() {
		b, err := d.Backup(fmt.Sprintf("schema%d", current))
		if err != nil {
			return corruptOr(fmt.Errorf("could not copy the database before changing it: %w", err))
		}
		d.migrated.Backup = b
	}
	ctx := context.Background()
	conn, err := d.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if err := runMigration(ctx, conn, m); err != nil {
			return corruptOr(fmt.Errorf("migration %d (%s): %w", m.version, m.name, err))
		}
		d.migrated.To = m.version
	}
	return nil
}

func runMigration(ctx context.Context, conn *sql.Conn, m migration) error {
	if m.rebuild {
		if _, err := conn.ExecContext(ctx, `PRAGMA foreign_keys = OFF`); err != nil {
			return err
		}
		defer func() { _, _ = conn.ExecContext(ctx, `PRAGMA foreign_keys = ON`) }()
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := m.up(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if m.rebuild {
		rows, err := tx.Query(`PRAGMA foreign_key_check`)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
		broken := rows.Next()
		rows.Close()
		if broken {
			_ = tx.Rollback()
			return errors.New("the foreign keys do not hold after the migration")
		}
	}
	// the version is part of the same transaction: it changes if and only if the migration did
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, m.version)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// legacyResets tells that the file is from before the versions and the baseline would start it
// over (its "schema" row is not the released one).
func (d *DB) legacyResets() bool {
	var v string
	if err := d.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema'`).Scan(&v); err != nil {
		return false // no meta table, or no row: a new file
	}
	return v != fmt.Sprint(baselineSchema)
}

func modTime(p string) time.Time {
	if fi, err := os.Stat(p); err == nil {
		return fi.ModTime()
	}
	return time.Time{}
}

// column is a column a migration adds to a table.
type column struct{ name, def string }

// addColumns adds the columns the table does not have yet.
func addColumns(tx *sql.Tx, table string, cols []column) error {
	rows, err := tx.Query(`PRAGMA table_info(` + table + `)`)
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
		have[strings.ToLower(name)] = true
	}
	rows.Close()
	for _, c := range cols {
		if !have[strings.ToLower(c.name)] {
			if _, err := tx.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + c.name + ` ` + c.def); err != nil {
				return err
			}
		}
	}
	return nil
}

// migrateBaseline brings any file of the releases 0.3 and 0.4 (or a new one) to the schema of
// version 1. It only creates what is missing, so it changes nothing in a file that already has it.
func migrateBaseline(tx *sql.Tx) error {
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	var v string
	err := tx.QueryRow(`SELECT value FROM meta WHERE key = 'schema'`).Scan(&v)
	switch {
	case err == nil && v != fmt.Sprint(baselineSchema):
		// an unreleased development schema (a copy was made): what is fetched from the simulator
		// starts over; the user's own tables stay
		for _, t := range cacheTables {
			if _, e := tx.Exec(`DROP TABLE IF EXISTS ` + t); e != nil {
				return e
			}
		}
	case err != nil && !errors.Is(err, sql.ErrNoRows):
		return err
	}
	for _, q := range []string{schema, extraSchema, savedRouteSchema} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	if err := addColumns(tx, "user_point", []column{{"alt_ft", "REAL"}, {"type", "TEXT"}, {"tags", "TEXT"}}); err != nil {
		return err
	}
	if err := addColumns(tx, "saved_route", []column{{"choices", "TEXT"}, {"result", "TEXT"}}); err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT OR REPLACE INTO meta (key, value) VALUES ('schema', ?)`, fmt.Sprint(baselineSchema))
	return err
}
