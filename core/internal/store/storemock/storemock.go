// Package storemock has a mock of store.Library, made with testify, for the tests of the packages
// that take the library as an interface. A call the test did not set up panics (testify's rule).
package storemock

import (
	"github.com/stretchr/testify/mock"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

// Library is a mock of the chart library.
type Library struct{ mock.Mock }

func (m *Library) Dir() string { return m.Called().String(0) }

func (m *Library) ChartDir(id string) string { return m.Called(id).String(0) }

func (m *Library) Get(id string) (*store.Chart, bool) {
	args := m.Called(id)
	c, _ := args.Get(0).(*store.Chart) // nil when the test returned nil
	return c, args.Bool(1)
}

func (m *Library) FindByOriginal(name string) (*store.Chart, bool) {
	args := m.Called(name)
	c, _ := args.Get(0).(*store.Chart)
	return c, args.Bool(1)
}

func (m *Library) Search(q store.Query) []*store.Chart {
	l, _ := m.Called(q).Get(0).([]*store.Chart)
	return l
}

func (m *Library) Count() int { return m.Called().Int(0) }

func (m *Library) Types() []string {
	l, _ := m.Called().Get(0).([]string)
	return l
}

func (m *Library) Airports(prefix string) []store.AirportSummary {
	l, _ := m.Called(prefix).Get(0).([]store.AirportSummary)
	return l
}

func (m *Library) UniqueID(base string) string { return m.Called(base).String(0) }

func (m *Library) AddFiles(id string, png, chartJSON, pdf []byte, extra map[string][]byte) (*store.Chart, error) {
	args := m.Called(id, png, chartJSON, pdf, extra)
	c, _ := args.Get(0).(*store.Chart)
	return c, args.Error(1)
}

func (m *Library) Update(id string, p store.Patch) (*store.Chart, error) {
	args := m.Called(id, p)
	c, _ := args.Get(0).(*store.Chart)
	return c, args.Error(1)
}

func (m *Library) Delete(id string) error { return m.Called(id).Error(0) }

func (m *Library) DeleteMany(ids []string) (int, error) {
	args := m.Called(ids)
	return args.Int(0), args.Error(1)
}

func (m *Library) Groups() store.GroupsState {
	g, _ := m.Called().Get(0).(store.GroupsState)
	return g
}

func (m *Library) CreateGroup(name string) (store.Group, error) {
	args := m.Called(name)
	g, _ := args.Get(0).(store.Group)
	return g, args.Error(1)
}

func (m *Library) SetCurrent(id string) error { return m.Called(id).Error(0) }

func (m *Library) RenameGroup(id, name string) error { return m.Called(id, name).Error(0) }

func (m *Library) DeleteGroup(id string) error { return m.Called(id).Error(0) }

func (m *Library) Pin(groupID, chartID string) error { return m.Called(groupID, chartID).Error(0) }

func (m *Library) Unpin(groupID, chartID string) error { return m.Called(groupID, chartID).Error(0) }
