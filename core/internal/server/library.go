package server

import "github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"

// Library is the chart library as the handlers use it (store.Library is what the program gives;
// store/storemock has a mock for the tests).
type Library interface {
	Dir() string
	ChartDir(id string) string

	Get(id string) (*store.Chart, bool)
	FindByOriginal(name string) (*store.Chart, bool)
	Search(q store.Query) []*store.Chart
	Count() int
	Types() []string
	Airports(prefix string) []store.AirportSummary

	UniqueID(base string) string
	AddFiles(id string, png, chartJSON, pdf []byte, extra map[string][]byte) (*store.Chart, error)
	Update(id string, p store.Patch) (*store.Chart, error)
	Delete(id string) error
	DeleteMany(ids []string) (int, error)
	DeleteAll() (int, error)

	Groups() store.GroupsState
	CreateGroup(name string) (store.Group, error)
	SetCurrent(id string) error
	RenameGroup(id, name string) error
	DeleteGroup(id string) error
	Pin(groupID, chartID string) error
	Unpin(groupID, chartID string) error
}

var _ Library = (*store.Library)(nil)
