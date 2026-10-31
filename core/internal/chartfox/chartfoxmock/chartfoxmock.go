// Package chartfoxmock has a mock of chartfox.Connector, made with testify, for the tests of the packages that take the
// ChartFox account as an interface. A call the test did not set up panics (testify's rule).
package chartfoxmock

import (
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
