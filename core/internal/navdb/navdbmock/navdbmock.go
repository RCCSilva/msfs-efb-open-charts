// Package navdbmock has a mock of the methods of navdb.DB that other packages use, made with
// testify, for the tests of the packages that take them as an interface.
package navdbmock

import (
	"encoding/json"

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
