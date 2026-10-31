// Package chartfoxmock has a mock of chartfox.Connector, made with testify, for the tests of the packages that take the
// ChartFox account as an interface. A call the test did not set up panics (testify's rule).
package chartfoxmock

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/chartfox"
)

// Connector is a mock of the ChartFox account.
type Connector struct{ mock.Mock }

func (m *Connector) Status() chartfox.Status {
	st, _ := m.Called().Get(0).(chartfox.Status) // the zero Status when the test returned nil
	return st
}

func (m *Connector) Connect() error { return m.Called().Error(0) }

func (m *Connector) Disconnect() error { return m.Called().Error(0) }

// Catalog is a mock of the ChartFox catalogue.
type Catalog struct{ mock.Mock }

func (m *Catalog) Airport(ctx context.Context, icao string) (chartfox.Airport, error) {
	args := m.Called(ctx, icao)
	a, _ := args.Get(0).(chartfox.Airport)
	return a, args.Error(1)
}

func (m *Catalog) Charts(ctx context.Context, icao string) ([]chartfox.Chart, error) {
	args := m.Called(ctx, icao)
	c, _ := args.Get(0).([]chartfox.Chart)
	return c, args.Error(1)
}

func (m *Catalog) Chart(ctx context.Context, id string) (chartfox.Detail, error) {
	args := m.Called(ctx, id)
	d, _ := args.Get(0).(chartfox.Detail)
	return d, args.Error(1)
}

func (m *Catalog) Download(ctx context.Context, fileURL string) ([]byte, error) {
	args := m.Called(ctx, fileURL)
	b, _ := args.Get(0).([]byte)
	return b, args.Error(1)
}
