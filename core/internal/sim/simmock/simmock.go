// Package simmock has mocks of the interfaces of package sim, made with testify, for the tests of the
// packages that take them. A test that needs something more specific makes its own mock.
package simmock

import (
	"github.com/stretchr/testify/mock"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
)

// Position is a mock of sim.PositionSource.
type Position struct{ mock.Mock }

func (m *Position) Position() sim.Position {
	args := m.Called()
	p, _ := args.Get(0).(sim.Position)
	return p
}

// Nav is a mock of sim.NavSource.
type Nav struct{ mock.Mock }

func (m *Nav) Airport(icao string) (*navdata.Airport, error) {
	args := m.Called(icao)
	a, _ := args.Get(0).(*navdata.Airport)
	return a, args.Error(1)
}

func (m *Nav) Airports(icaos []string) ([]sim.AirportResult, error) {
	args := m.Called(icaos)
	r, _ := args.Get(0).([]sim.AirportResult)
	return r, args.Error(1)
}

func (m *Nav) ListAirports() ([]sim.AirportRef, error) {
	args := m.Called()
	l, _ := args.Get(0).([]sim.AirportRef)
	return l, args.Error(1)
}

func (m *Nav) LookupFixes(idents []string) ([]navdata.Fix, error) {
	args := m.Called(idents)
	f, _ := args.Get(0).([]navdata.Fix)
	return f, args.Error(1)
}

// Seed is a mock of sim.SeedSource.
type Seed struct{ mock.Mock }

func (m *Seed) Locate() []string {
	args := m.Called()
	f, _ := args.Get(0).([]string)
	return f
}

func (m *Seed) Read(files []string, progress func(done, total int) bool) ([]navdb.SeedIdent, error) {
	args := m.Called(files, progress)
	l, _ := args.Get(0).([]navdb.SeedIdent)
	return l, args.Error(1)
}

// Plans is a mock of sim.FlightPlanLoader.
type Plans struct{ mock.Mock }

func (m *Plans) Load(file string) error {
	return m.Called(file).Error(0)
}
