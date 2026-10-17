// Package navdbmock has a mock of the methods of navdb.DB that other packages use, made with
// testify, for the tests of the packages that take them as an interface.
package navdbmock

import (
	"encoding/json"
	"io"

	"github.com/stretchr/testify/mock"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
)

// Store is a mock of the airport and fix lookups of navdb.DB.
type Store struct{ mock.Mock }

func (m *Store) Airport(icao string) (*navdata.Airport, error) {
	args := m.Called(icao)
	a, _ := args.Get(0).(*navdata.Airport) // nil when the test returned nil
	return a, args.Error(1)
}

func (m *Store) SaveAirport(a *navdata.Airport) error {
	return m.Called(a).Error(0)
}

func (m *Store) UncheckedIdents(icao string) ([]string, error) {
	args := m.Called(icao)
	l, _ := args.Get(0).([]string)
	return l, args.Error(1)
}

func (m *Store) IdentChecked(ident string) (bool, error) {
	args := m.Called(ident)
	return args.Bool(0), args.Error(1)
}

func (m *Store) FixesByIdent(ident string) ([]navdata.Fix, error) {
	args := m.Called(ident)
	f, _ := args.Get(0).([]navdata.Fix)
	return f, args.Error(1)
}

func (m *Store) UserFixes(ident string) ([]navdata.Fix, error) {
	args := m.Called(ident)
	f, _ := args.Get(0).([]navdata.Fix)
	return f, args.Error(1)
}

func (m *Store) FixAt(ref navdata.FixRef) (*navdata.Fix, error) {
	args := m.Called(ref)
	f, _ := args.Get(0).(*navdata.Fix)
	return f, args.Error(1)
}

func (m *Store) SaveFixes(fixes []navdata.Fix) error {
	return m.Called(fixes).Error(0)
}

func (m *Store) MarkIdentChecked(ident string, found int) error {
	return m.Called(ident, found).Error(0)
}

// SavedRoutes is a mock of the saved routes of navdb.DB.
type SavedRoutes struct{ mock.Mock }

func (m *SavedRoutes) SavedRoutes() ([]navdb.SavedRoute, error) {
	args := m.Called()
	l, _ := args.Get(0).([]navdb.SavedRoute)
	return l, args.Error(1)
}

func (m *SavedRoutes) SavedRoute(id int64) (navdb.SavedRoute, error) {
	args := m.Called(id)
	r, _ := args.Get(0).(navdb.SavedRoute)
	return r, args.Error(1)
}

func (m *SavedRoutes) CreateSavedRoute(r navdb.SavedRoute) (navdb.SavedRoute, error) {
	args := m.Called(r)
	out, _ := args.Get(0).(navdb.SavedRoute)
	return out, args.Error(1)
}

func (m *SavedRoutes) UpdateSavedRoute(r navdb.SavedRoute) (navdb.SavedRoute, error) {
	args := m.Called(r)
	out, _ := args.Get(0).(navdb.SavedRoute)
	return out, args.Error(1)
}

func (m *SavedRoutes) DeleteSavedRoute(id int64) error {
	return m.Called(id).Error(0)
}

func (m *SavedRoutes) SetSavedRouteResult(id int64, result json.RawMessage) error {
	return m.Called(id, result).Error(0)
}

// Load is a mock of what the bulk load uses of navdb.DB.
type Load struct{ mock.Mock }

func (m *Load) SaveSeed(list []navdb.SeedIdent) error {
	return m.Called(list).Error(0)
}

func (m *Load) Counts() (navdb.Counts, error) {
	args := m.Called()
	c, _ := args.Get(0).(navdb.Counts)
	return c, args.Error(1)
}

func (m *Load) SeedPending(n int) ([]string, error) {
	args := m.Called(n)
	l, _ := args.Get(0).([]string)
	return l, args.Error(1)
}

func (m *Load) MissingNeighbors(n int) ([]string, error) {
	args := m.Called(n)
	l, _ := args.Get(0).([]string)
	return l, args.Error(1)
}

func (m *Load) Airports(prefix string) ([]navdb.Summary, error) {
	args := m.Called(prefix)
	l, _ := args.Get(0).([]navdb.Summary)
	return l, args.Error(1)
}

func (m *Load) SaveAirport(a *navdata.Airport) error {
	return m.Called(a).Error(0)
}

func (m *Load) Meta(key string) string {
	return m.Called(key).String(0)
}

func (m *Load) SetMeta(key, value string) error {
	return m.Called(key, value).Error(0)
}

// Disk is a mock of navdb.Disk.
type Disk struct{ mock.Mock }

func (m *Disk) List(dbPath string) ([]navdb.BackupInfo, error) {
	args := m.Called(dbPath)
	l, _ := args.Get(0).([]navdb.BackupInfo)
	return l, args.Error(1)
}

func (m *Disk) File(dbPath, name string) (string, error) {
	args := m.Called(dbPath, name)
	return args.String(0), args.Error(1)
}

func (m *Disk) Delete(dbPath, name string) error {
	return m.Called(dbPath, name).Error(0)
}

func (m *Disk) Prepare(dbPath, src string) (string, error) {
	args := m.Called(dbPath, src)
	return args.String(0), args.Error(1)
}

func (m *Disk) WriteIncoming(dbPath string, r io.Reader) error {
	return m.Called(dbPath, r).Error(0)
}

func (m *Disk) Discard(dbPath string) {
	m.Called(dbPath)
}

func (m *Disk) InspectIncoming(dbPath string) (int, error) {
	args := m.Called(dbPath)
	return args.Int(0), args.Error(1)
}

func (m *Disk) Install(dbPath string) error {
	return m.Called(dbPath).Error(0)
}

func (m *Disk) Remove(dbPath string) error {
	return m.Called(dbPath).Error(0)
}

func (m *Disk) SetAside(dbPath string) (string, error) {
	args := m.Called(dbPath)
	return args.String(0), args.Error(1)
}

func (m *Disk) Size(path string) (int64, bool) {
	args := m.Called(path)
	return int64(args.Int(0)), args.Bool(1)
}

// Database is a mock of what the maintenance uses of an open navdb.DB.
type Database struct{ mock.Mock }

func (m *Database) Backup(kind string) (string, error) {
	args := m.Called(kind)
	return args.String(0), args.Error(1)
}

func (m *Database) SchemaVersion() (int, error) {
	args := m.Called()
	return args.Int(0), args.Error(1)
}

func (m *Database) Migrated() navdb.Migration {
	args := m.Called()
	v, _ := args.Get(0).(navdb.Migration)
	return v
}
