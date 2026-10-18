package navload_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

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
	f.store.On("FillRoutePositions").Return(0, nil).Maybe()
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
	seed := []navdb.SeedIdent{{Ident: "GERTU", Classes: 1 << 5}}
	f.seed.On("Locate").Return([]string{"a.fsarchive"})
	f.seed.On("Read", []string{"a.fsarchive"}, mock.Anything).Return(seed, nil)
	f.seed.On("Waypoints", []string{"a.fsarchive"}, mock.Anything).Return(nil, nil)
	f.store.On("SaveSeed", seed).Return(nil)
	f.store.On("MarkIdentsCovered", []string{"GERTU"}).Return(nil)

	// Act
	v := f.run(t, navload.Files, "")

	// Assert
	assert.Equal(t, "done", v.Phase)
	f.store.AssertCalled(t, "SaveSeed", seed)
	f.store.AssertNotCalled(t, "Counts")
}

func TestTheWaypointsOfTheFilesAreStoredAndTheirNamesNeedNoQuestionToTheSimulator(t *testing.T) {
	// Arrange
	f := newFixture()
	seed := []navdb.SeedIdent{
		{Ident: "GERTU", Classes: 1 << 5},       // a waypoint only, and not a short name: covered
		{Ident: "PTL", Classes: 1 << 2},         // a VOR: the simulator has its name and frequency
		{Ident: "BAD", Classes: 1 << 5},         // short: may be a VOR of a region the files lack
		{Ident: "LONGVR", Classes: 1<<5 | 1<<2}, // also a VOR
	}
	fixes := []navdata.Fix{{Ident: "GERTU", Region: "SB", Kind: navdata.FixWaypoint}}
	f.seed.On("Locate").Return([]string{"a.fsarchive"})
	f.seed.On("Read", mock.Anything, mock.Anything).Return(seed, nil)
	f.seed.On("Waypoints", mock.Anything, mock.Anything).Return(fixes, nil)
	f.store.On("SaveSeed", seed).Return(nil)
	f.store.On("SaveFileFixes", fixes).Return(1, nil)
	f.store.On("MarkIdentsCovered", []string{"GERTU"}).Return(nil)

	// Act
	v := f.run(t, navload.Files, "")

	// Assert
	assert.Equal(t, "done", v.Phase)
	f.store.AssertCalled(t, "SaveFileFixes", fixes)
	f.store.AssertCalled(t, "MarkIdentsCovered", []string{"GERTU"})
	f.store.AssertCalled(t, "FillRoutePositions")
}

func TestAnErrorReadingTheWaypointsEndsTheLoad(t *testing.T) {
	// Arrange
	f := newFixture()
	seed := []navdb.SeedIdent{{Ident: "GERTU"}}
	f.seed.On("Locate").Return([]string{"a.fsarchive"})
	f.seed.On("Read", mock.Anything, mock.Anything).Return(seed, nil)
	f.seed.On("Waypoints", mock.Anything, mock.Anything).Return(nil, errors.New("unreadable"))
	f.store.On("SaveSeed", seed).Return(nil)

	// Act
	v := f.run(t, navload.Files, "")

	// Assert
	assert.Equal(t, "error", v.Phase)
	assert.Contains(t, v.Message, "unreadable")
	f.store.AssertNotCalled(t, "SaveFileFixes")
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
	f.store.On("SeedPending", navload.IdentBatch).Return(first, nil).Once()
	f.store.On("SeedPending", navload.IdentBatch).Return(second, nil).Once()
	f.store.On("SeedPending", navload.IdentBatch).Return(nil, nil)
	f.store.On("MissingNeighbors", navload.IdentBatch).Return(neighbours, nil).Once()
	f.store.On("MissingNeighbors", navload.IdentBatch).Return(nil, nil)
	f.idents.On("LoadIdentsCount", mock.Anything, mock.Anything).Return(2, nil)

	// Act
	v := f.run(t, navload.Fixes, "")

	// Assert
	assert.Equal(t, "done", v.Phase)
	assert.Zero(t, v.Total, "the neighbours are a step of unknown size: there is no total")
	assert.Equal(t, 1, v.Done, "how many neighbours were asked")
	f.idents.AssertNumberOfCalls(t, "LoadIdentsCount", 3)
	f.idents.AssertCalled(t, "LoadIdentsCount", first, false) // nothing answered yet: an empty answer is not recorded
	f.idents.AssertCalled(t, "LoadIdentsCount", second, true) // the simulator has answered before
}

func TestThreeEmptyAnswersInARowMeanTheSimulatorIsAtItsMenu(t *testing.T) {
	// Arrange
	f := newFixture()
	f.seed.On("Locate").Return(nil)
	f.store.On("Counts").Return(navdb.Counts{SeedIdents: 500, SeedPending: 500}, nil)
	f.store.On("SeedPending", navload.IdentBatch).Return([]string{"AAAAA"}, nil)
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
	f.store.On("SeedPending", navload.IdentBatch).Return(nil, nil)
	f.store.On("MissingNeighbors", navload.IdentBatch).Return(nil, nil)
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

func TestALoadWritesWhereItsTimeWentToTheLog(t *testing.T) {
	// Arrange
	f := newFixture()
	f.seed.On("Locate").Return(nil)
	f.store.On("Counts").Return(navdb.Counts{SeedIdents: 2, SeedPending: 2}, nil)
	f.store.On("SeedPending", navload.IdentBatch).Return([]string{"AAAAA", "BBBBB"}, nil).Once()
	f.store.On("SeedPending", navload.IdentBatch).Return(nil, nil)
	f.store.On("MissingNeighbors", navload.IdentBatch).Return(nil, nil)
	f.idents.On("LoadIdentsCount", mock.Anything, mock.Anything).Return(2, nil)
	var lines []string
	timings := navload.NewTimings()
	deps := navload.Deps{Store: f.store, Airports: f.nav, Seed: f.seed, Idents: f.idents, Turn: new(sync.Mutex),
		Timings: timings, Logf: func(format string, args ...any) { lines = append(lines, fmt.Sprintf(format, args...)) }}
	require.NoError(t, f.job.Begin())

	// Act
	navload.New(f.job, deps).Run(navload.Fixes, "")

	// Assert
	log := strings.Join(lines, "\n")
	assert.Contains(t, log, "load: started")
	assert.Contains(t, log, "load: done")
	assert.Contains(t, log, "ask.batch (sim+db)")
	assert.Contains(t, log, "2 items")
}

func TestTimingsAddUpEachStepAndListTheSlowestFirst(t *testing.T) {
	// Arrange
	timings := navload.NewTimings()
	timings.Add("quick", 10*time.Millisecond, 5)
	timings.Add("slow", 2*time.Second, 40)
	timings.Add("slow", 1*time.Second, 40)

	// Act
	report := timings.Report()

	// Assert
	assert.Less(t, strings.Index(report, "slow"), strings.Index(report, "quick"), "the slowest first")
	assert.Contains(t, report, "3s in     2 runs")
	assert.Contains(t, report, "80 items")
	assert.Contains(t, report, "37.5 ms per item")
}

func TestNoTimingsRecordNothingAndDoNotFail(t *testing.T) {
	// Arrange
	var timings *navload.Timings

	// Act
	timings.Add("x", time.Second, 1)

	// Assert
	assert.Empty(t, timings.Report())
}
