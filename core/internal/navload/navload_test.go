package navload_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb/navdbmock"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navload"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/simmock"
)

var errBoom = errors.New("boom")

// identsMock is a mock of navload.IdentLoader.
type identsMock struct{ mock.Mock }

func (m *identsMock) LoadIdentsCount(idents []string, markEmpty bool) (int, error) {
	args := m.Called(idents, markEmpty)
	return args.Int(0), args.Error(1)
}

// echoNav answers every airport it is asked for, except SBXX, which it does not know. It remembers
// how many were asked in each batch and can ask the job to stop after the first.
type echoNav struct {
	simmock.Nav
	sizes     []int
	stopAfter *navload.Job
	failWith  error
}

func (e *echoNav) Airports(codes []string) ([]sim.AirportResult, error) {
	e.sizes = append(e.sizes, len(codes))
	if e.stopAfter != nil {
		e.stopAfter.Stop()
	}
	out := make([]sim.AirportResult, len(codes))
	for i, c := range codes {
		switch {
		case c == "SBXX":
			out[i].Err = navdata.ErrUnknown
		case e.failWith != nil:
			out[i].Err = e.failWith
		default:
			out[i].Airport = &navdata.Airport{ICAO: c, Name: c}
		}
	}
	return out, nil
}

// fixture is a loader with its mocks. The store accepts the saving of the state of the job.
type fixture struct {
	job    *navload.Job
	store  *navdbmock.Load
	nav    *echoNav
	idents *identsMock
	seed   *simmock.Seed
}

func newFixture() *fixture {
	f := &fixture{job: new(navload.Job), store: new(navdbmock.Load), nav: new(echoNav), idents: new(identsMock), seed: new(simmock.Seed)}
	f.store.On("SetMeta", mock.Anything, mock.Anything).Return(nil)
	return f
}

func (f *fixture) loader() *navload.Loader {
	deps := navload.Deps{Store: f.store, Airports: f.nav, Seed: f.seed, Idents: f.idents, Turn: new(sync.Mutex)}
	return navload.New(f.job, deps)
}

func (f *fixture) run(t *testing.T, mode navload.Mode, prefix string) navload.View {
	t.Helper()
	require.NoError(t, f.job.Begin())
	f.loader().Run(mode, prefix)
	return f.job.Snapshot()
}

func refs(codes ...string) []sim.AirportRef {
	var out []sim.AirportRef
	for _, c := range codes {
		out = append(out, sim.AirportRef{ICAO: c})
	}
	return out
}

func TestOnlyTheMissingAirportsThatMatchThePrefixAreAsked(t *testing.T) {
	// Arrange
	f := newFixture()
	f.nav.On("ListAirports").Return(refs("SBSJ", "SBGR", "KJFK", "SBXX", "SBSJ"), nil)
	f.store.On("Airports", "").Return([]navdb.Summary{{ICAO: "SBGR"}}, nil) // already stored
	f.store.On("SaveAirport", mock.Anything).Return(nil)

	// Act
	v := f.run(t, navload.Airports, "sb")

	// Assert
	assert.Equal(t, "done", v.Phase)
	assert.Equal(t, []int{2}, f.nav.sizes, "SBSJ once and SBXX: not SBGR, not KJFK")
	assert.Equal(t, 2, v.Total)
	assert.Equal(t, 2, v.Done)
	f.store.AssertNumberOfCalls(t, "SaveAirport", 1) // the one the simulator did not know is skipped
}

func TestAirportsAreAskedManyAtATime(t *testing.T) {
	// Arrange
	f := newFixture()
	var list []sim.AirportRef
	for i := 0; i < 450; i++ {
		list = append(list, sim.AirportRef{ICAO: "SB" + string(rune('A'+i/26/26%26)) + string(rune('A'+i/26%26)) + string(rune('A'+i%26))})
	}
	f.nav.On("ListAirports").Return(list, nil)
	f.store.On("Airports", "").Return([]navdb.Summary{}, nil)
	f.store.On("SaveAirport", mock.Anything).Return(nil)

	// Act
	v := f.run(t, navload.Airports, "SB")

	// Assert
	assert.Equal(t, []int{200, 200, 50}, f.nav.sizes)
	assert.Equal(t, 450, v.Total)
	assert.Equal(t, 450, v.Done)
	assert.Equal(t, "done", v.Phase)
}

func TestFiveFailuresInARowEndTheLoadWithAnError(t *testing.T) {
	// Arrange
	f := newFixture()
	f.nav.failWith = errBoom
	f.nav.On("ListAirports").Return(refs("SBAA", "SBBB", "SBCC", "SBDD", "SBEE"), nil)
	f.store.On("Airports", "").Return([]navdb.Summary{}, nil)

	// Act
	v := f.run(t, navload.Airports, "")

	// Assert
	assert.Equal(t, "error", v.Phase)
	assert.Equal(t, "boom", v.Message)
	assert.False(t, v.Running)
}

func TestAnAirportThatCannotBeStoredCountsAsAFailure(t *testing.T) {
	// Arrange
	f := newFixture()
	f.nav.On("ListAirports").Return(refs("SBAA", "SBBB", "SBCC", "SBDD", "SBEE"), nil)
	f.store.On("Airports", "").Return([]navdb.Summary{}, nil)
	f.store.On("SaveAirport", mock.Anything).Return(errBoom)

	// Act
	v := f.run(t, navload.Airports, "")

	// Assert
	assert.Equal(t, "error", v.Phase)
	assert.Equal(t, "boom", v.Message)
}

func TestStoppingEndsTheLoadWhereItIsAndKeepsWhatWasLoaded(t *testing.T) {
	// Arrange
	f := newFixture()
	f.nav.stopAfter = f.job // the first batch asks the job to stop
	var list []sim.AirportRef
	for i := 0; i < 450; i++ {
		list = append(list, sim.AirportRef{ICAO: "SB" + string(rune('A'+i/26/26%26)) + string(rune('A'+i/26%26)) + string(rune('A'+i%26))})
	}
	f.nav.On("ListAirports").Return(list, nil)
	f.store.On("Airports", "").Return([]navdb.Summary{}, nil)
	f.store.On("SaveAirport", mock.Anything).Return(nil)

	// Act
	v := f.run(t, navload.Airports, "")

	// Assert
	assert.Equal(t, "stopped", v.Phase)
	assert.Equal(t, []int{200}, f.nav.sizes, "no second batch")
	assert.Equal(t, 200, v.Done)
	f.store.AssertNumberOfCalls(t, "SaveAirport", 200)
}

func TestAirportsWithoutASimulatorToAskIsAnError(t *testing.T) {
	// Arrange
	job := new(navload.Job)
	store := new(navdbmock.Load)
	store.On("SetMeta", mock.Anything, mock.Anything).Return(nil)
	loader := navload.New(job, navload.Deps{Store: store, Turn: new(sync.Mutex)}) // no Airports
	require.NoError(t, job.Begin())

	// Act
	loader.Run(navload.Airports, "")

	// Assert
	v := job.Snapshot()
	assert.Equal(t, "error", v.Phase)
	assert.Contains(t, v.Message, "no link to the simulator")
}

func TestTheFilesStepReadsTheIdentifiersAndStops(t *testing.T) {
	// Arrange
	f := newFixture()
	seed := []navdb.SeedIdent{{Ident: "GERTU"}}
	f.seed.On("Locate").Return([]string{"a.fsarchive"})
	f.seed.On("Read", []string{"a.fsarchive"}, mock.Anything).Return(seed, nil)
	f.store.On("SaveSeed", seed).Return(nil)

	// Act
	v := f.run(t, navload.Files, "")

	// Assert
	assert.Equal(t, "done", v.Phase)
	f.store.AssertCalled(t, "SaveSeed", seed)
	f.store.AssertNotCalled(t, "Counts")
}

func TestTheFilesStepWithoutFilesIsAnError(t *testing.T) {
	// Arrange
	f := newFixture()
	f.seed.On("Locate").Return(nil)

	// Act
	v := f.run(t, navload.Files, "")

	// Assert
	assert.Equal(t, "error", v.Phase)
	assert.Contains(t, v.Message, "no navigation package")
}

func TestTheIdentifiersAreAskedInBatchesThenTheNeighboursTheAirwaysCite(t *testing.T) {
	// Arrange
	f := newFixture()
	f.seed.On("Locate").Return(nil) // the names are already in the database
	f.store.On("Counts").Return(navdb.Counts{SeedIdents: 80, SeedPending: 80}, nil)
	first := []string{"AAAAA", "BBBBB"}
	second := []string{"CCCCC"}
	neighbours := []string{"NNNNN"}
	f.store.On("SeedPending", 40).Return(first, nil).Once()
	f.store.On("SeedPending", 40).Return(second, nil).Once()
	f.store.On("SeedPending", 40).Return(nil, nil)
	f.store.On("MissingNeighbors", 40).Return(neighbours, nil).Once()
	f.store.On("MissingNeighbors", 40).Return(nil, nil)
	f.idents.On("LoadIdentsCount", mock.Anything, mock.Anything).Return(2, nil)

	// Act
	v := f.run(t, navload.Fixes, "")

	// Assert
	assert.Equal(t, "done", v.Phase)
	assert.Equal(t, 81, v.Total, "the neighbours are added to what is to be asked")
	assert.Equal(t, 4, v.Done)
	f.idents.AssertNumberOfCalls(t, "LoadIdentsCount", 3)
	f.idents.AssertCalled(t, "LoadIdentsCount", first, false) // nothing answered yet: an empty answer is not recorded
	f.idents.AssertCalled(t, "LoadIdentsCount", second, true) // the simulator has answered before
}

func TestThreeEmptyAnswersInARowMeanTheSimulatorIsAtItsMenu(t *testing.T) {
	// Arrange
	f := newFixture()
	f.seed.On("Locate").Return(nil)
	f.store.On("Counts").Return(navdb.Counts{SeedIdents: 500, SeedPending: 500}, nil)
	f.store.On("SeedPending", 40).Return([]string{"AAAAA"}, nil)
	f.idents.On("LoadIdentsCount", mock.Anything, false).Return(0, nil)

	// Act
	v := f.run(t, navload.Fixes, "")

	// Assert
	assert.Equal(t, "error", v.Phase)
	assert.Contains(t, v.Message, "load a flight")
	f.idents.AssertNumberOfCalls(t, "LoadIdentsCount", 3)
}

func TestWithNoIdentifiersToAskTheLoadSaysSo(t *testing.T) {
	// Arrange
	f := newFixture()
	f.seed.On("Locate").Return(nil)
	f.store.On("Counts").Return(navdb.Counts{}, nil)

	// Act
	v := f.run(t, navload.Fixes, "")

	// Assert
	assert.Equal(t, "error", v.Phase)
	assert.Contains(t, v.Message, "no identifiers to ask for")
}

func TestAllDoesTheIdentifiersAndThenTheAirports(t *testing.T) {
	// Arrange
	f := newFixture()
	f.seed.On("Locate").Return(nil)
	f.store.On("Counts").Return(navdb.Counts{SeedIdents: 1}, nil)
	f.store.On("SeedPending", 40).Return(nil, nil)
	f.store.On("MissingNeighbors", 40).Return(nil, nil)
	f.nav.On("ListAirports").Return(refs("SBAA"), nil)
	f.store.On("Airports", "").Return([]navdb.Summary{}, nil)
	f.store.On("SaveAirport", mock.Anything).Return(nil)

	// Act
	v := f.run(t, navload.All, "")

	// Assert
	assert.Equal(t, "done", v.Phase)
	assert.Equal(t, []int{1}, f.nav.sizes)
}

func TestOnlyOneLoadRunsAtATime(t *testing.T) {
	// Arrange
	job := new(navload.Job)
	require.NoError(t, job.Begin())

	// Act
	err := job.Begin()

	// Assert
	assert.ErrorIs(t, err, navload.ErrRunning)
	assert.True(t, job.Running())
}

func TestParseModeFallsBackToAll(t *testing.T) {
	cases := map[string]navload.Mode{"files": navload.Files, "fixes": navload.Fixes, "airports": navload.Airports, "all": navload.All, "": navload.All, "whatever": navload.All}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			// Arrange: the text

			// Act
			got := navload.ParseMode(in)

			// Assert
			assert.Equal(t, want, got)
		})
	}
}

func TestARunningLoadIsRestoredAsInterrupted(t *testing.T) {
	// Arrange
	meta := new(navdbmock.Load)
	meta.On("Meta", "navload").Return(`{"running":true,"phase":"asking","total":10,"done":4,"message":"x"}`)
	job := new(navload.Job)

	// Act
	job.Restore(meta)

	// Assert
	v := job.Snapshot()
	assert.False(t, v.Running)
	assert.Equal(t, "interrupted", v.Phase)
	assert.Equal(t, 4, v.Done)
	assert.Empty(t, v.Message)
}

func TestARestoreFromADatabaseWithNoLoadShowsNone(t *testing.T) {
	// Arrange
	meta := new(navdbmock.Load)
	meta.On("Meta", "navload").Return("")
	job := new(navload.Job)
	require.NoError(t, job.Begin()) // something from another database
	meta.On("SetMeta", mock.Anything, mock.Anything).Return(nil)

	// Act
	job.Restore(meta)

	// Assert
	v := job.Snapshot()
	assert.False(t, v.Running)
	assert.Equal(t, "idle", v.Phase)
}

func TestTheStateIsKeptWithTheDatabase(t *testing.T) {
	// Arrange
	meta := new(navdbmock.Load)
	meta.On("SetMeta", "navload", mock.MatchedBy(func(s string) bool { return len(s) > 0 })).Return(nil)
	job := new(navload.Job)
	require.NoError(t, job.Begin())

	// Act
	job.Save(meta)

	// Assert
	meta.AssertExpectations(t)
}
