package navlookup_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdata"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb/navdbmock"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navlookup"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/simmock"
)

var errBoom = errors.New("boom")

func TestAirportInTheDatabaseIsNotAskedToTheSimulator(t *testing.T) {
	// Arrange
	store := new(navdbmock.Store)
	store.On("Airport", "SBSP").Return(&navdata.Airport{ICAO: "SBSP"}, nil)
	store.On("UncheckedIdents", "SBSP").Return(nil, nil) // every name its procedures cite is known
	nav := new(simmock.Nav)
	lookup := navlookup.New(store, nav)

	// Act
	a, err := lookup.Airport("SBSP")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "SBSP", a.ICAO)
	nav.AssertNotCalled(t, "Airport", "SBSP")
	store.AssertExpectations(t)
}

func TestAirportTheDatabaseDoesNotHaveIsAskedAndKept(t *testing.T) {
	// Arrange
	got := &navdata.Airport{ICAO: "SBSP", Name: "from the simulator"}
	stored := &navdata.Airport{ICAO: "SBSP", Name: "as stored"}
	store := new(navdbmock.Store)
	store.On("Airport", "SBSP").Return(nil, navdb.ErrNotFound).Once()
	store.On("SaveAirport", got).Return(nil)
	store.On("Airport", "SBSP").Return(stored, nil)
	store.On("UncheckedIdents", "SBSP").Return(nil, nil)
	nav := new(simmock.Nav)
	nav.On("Airport", "SBSP").Return(got, nil)
	lookup := navlookup.New(store, nav)

	// Act
	a, err := lookup.Airport("SBSP")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "as stored", a.Name, "what is returned is what was stored")
	store.AssertExpectations(t)
	nav.AssertExpectations(t)
}

func TestAirportTheSimulatorDoesNotKnowIsJustUnknown(t *testing.T) {
	// Arrange
	store := new(navdbmock.Store)
	store.On("Airport", "ZZZZ").Return(nil, navdb.ErrNotFound)
	nav := new(simmock.Nav)
	nav.On("Airport", "ZZZZ").Return(nil, navdata.ErrUnknown)

	// Act
	a, err := navlookup.New(store, nav).Airport("ZZZZ")

	// Assert
	assert.NoError(t, err)
	assert.Nil(t, a)
	store.AssertNotCalled(t, "SaveAirport")
}

func TestAirportWithoutASimulatorIsJustUnknown(t *testing.T) {
	// Arrange
	store := new(navdbmock.Store)
	store.On("Airport", "ZZZZ").Return(nil, navdb.ErrNotFound)

	// Act
	a, err := navlookup.New(store, nil).Airport("ZZZZ")

	// Assert
	assert.NoError(t, err)
	assert.Nil(t, a)
}

func TestAirportErrorsAreReported(t *testing.T) {
	cases := []struct {
		name  string
		setup func(store *navdbmock.Store, nav *simmock.Nav)
	}{
		{"the database fails", func(s *navdbmock.Store, n *simmock.Nav) {
			s.On("Airport", "SBSP").Return(nil, errBoom)
		}},
		{"the simulator fails", func(s *navdbmock.Store, n *simmock.Nav) {
			s.On("Airport", "SBSP").Return(nil, navdb.ErrNotFound)
			n.On("Airport", "SBSP").Return(nil, errBoom)
		}},
		{"what the simulator gave cannot be stored", func(s *navdbmock.Store, n *simmock.Nav) {
			a := &navdata.Airport{ICAO: "SBSP"}
			s.On("Airport", "SBSP").Return(nil, navdb.ErrNotFound)
			n.On("Airport", "SBSP").Return(a, nil)
			s.On("SaveAirport", a).Return(errBoom)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			store, nav := new(navdbmock.Store), new(simmock.Nav)
			c.setup(store, nav)

			// Act
			a, err := navlookup.New(store, nav).Airport("SBSP")

			// Assert
			assert.ErrorIs(t, err, errBoom)
			assert.Nil(t, a)
		})
	}
}

func TestAirportWithNamesNeverAskedGetsThemFromTheSimulator(t *testing.T) {
	// Arrange
	before := &navdata.Airport{ICAO: "SBSP", Name: "legs without positions"}
	after := &navdata.Airport{ICAO: "SBSP", Name: "legs placed"}
	found := []navdata.Fix{{Ident: "GERTU", Region: "SB"}}
	store := new(navdbmock.Store)
	store.On("Airport", "SBSP").Return(before, nil).Once()
	store.On("UncheckedIdents", "SBSP").Return([]string{"GERTU"}, nil)
	store.On("SaveFixes", found).Return(nil)
	store.On("MarkIdentChecked", "GERTU", 1).Return(nil)
	store.On("Airport", "SBSP").Return(after, nil)
	nav := new(simmock.Nav)
	nav.On("LookupFixes", []string{"GERTU"}).Return(found, nil)

	// Act
	a, err := navlookup.New(store, nav).Airport("SBSP")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "legs placed", a.Name, "the airport is read again after its names were stored")
	store.AssertExpectations(t)
}

func TestFixesAreAskedOnceAndThenComeFromTheDatabase(t *testing.T) {
	// Arrange
	fix := navdata.Fix{Ident: "GERTU", Region: "SB"}
	mine := navdata.Fix{Ident: "GERTU", Region: navdb.UserRegion}
	store := new(navdbmock.Store)
	store.On("IdentChecked", "GERTU").Return(false, nil)
	store.On("SaveFixes", []navdata.Fix{fix}).Return(nil)
	store.On("MarkIdentChecked", "GERTU", 1).Return(nil)
	store.On("FixesByIdent", "GERTU").Return([]navdata.Fix{fix}, nil)
	store.On("UserFixes", "GERTU").Return([]navdata.Fix{mine}, nil)
	nav := new(simmock.Nav)
	nav.On("LookupFixes", []string{"GERTU"}).Return([]navdata.Fix{fix}, nil)

	// Act
	got, err := navlookup.New(store, nav).Fixes("GERTU")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []navdata.Fix{fix, mine}, got, "the points of the user are places like any other")
	store.AssertExpectations(t)
}

func TestFixesStayUncheckedWhenTheSimulatorCannotBeReached(t *testing.T) {
	// Arrange
	fix := navdata.Fix{Ident: "GERTU", Region: "SB"}
	store := new(navdbmock.Store)
	store.On("IdentChecked", "GERTU").Return(false, nil)
	store.On("FixesByIdent", "GERTU").Return([]navdata.Fix{fix}, nil)
	store.On("UserFixes", "GERTU").Return(nil, nil)
	nav := new(simmock.Nav)
	nav.On("LookupFixes", []string{"GERTU"}).Return(nil, errBoom)

	// Act
	got, err := navlookup.New(store, nav).Fixes("GERTU")

	// Assert
	require.NoError(t, err, "what is stored is the answer")
	assert.Equal(t, []navdata.Fix{fix}, got)
	store.AssertNotCalled(t, "MarkIdentChecked")
}

func TestFixAtAPointOfTheUserIsFoundAmongTheirOwnPoints(t *testing.T) {
	// Arrange
	mine := navdata.Fix{Ident: "MYPT", Region: navdb.UserRegion}
	store := new(navdbmock.Store)
	store.On("UserFixes", "MYPT").Return([]navdata.Fix{mine}, nil)

	// Act
	got, err := navlookup.New(store, nil).FixAt(navdata.FixRef{Ident: "MYPT", Region: "usr"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "MYPT", got.Ident)
	store.AssertNotCalled(t, "FixAt")
}

func TestFixAtAsksTheSimulatorAboutANameNeverAsked(t *testing.T) {
	// Arrange
	ref := navdata.FixRef{Ident: "GERTU", Region: "SB", Kind: 'W'}
	fix := &navdata.Fix{Ident: "GERTU", Region: "SB"}
	store := new(navdbmock.Store)
	store.On("IdentChecked", "GERTU").Return(false, nil)
	store.On("SaveFixes", mock.Anything).Return(nil)
	store.On("MarkIdentChecked", "GERTU", 0).Return(nil)
	store.On("FixAt", ref).Return(fix, nil)
	nav := new(simmock.Nav)
	nav.On("LookupFixes", []string{"GERTU"}).Return([]navdata.Fix{}, nil)

	// Act
	got, err := navlookup.New(store, nav).FixAt(ref)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, fix, got)
	nav.AssertExpectations(t)
}

func TestLoadIdentsCountMarksTheNamesTheSimulatorDoesNotKnow(t *testing.T) {
	// Arrange
	found := []navdata.Fix{{Ident: "AAAAA"}, {Ident: "AAAAA"}, {Ident: "BBBBB"}}
	store := new(navdbmock.Store)
	store.On("SaveFixes", found).Return(nil)
	store.On("MarkIdentChecked", "AAAAA", 2).Return(nil)
	store.On("MarkIdentChecked", "BBBBB", 1).Return(nil)
	store.On("MarkIdentChecked", "NOWHERE", 0).Return(nil)
	nav := new(simmock.Nav)
	nav.On("LookupFixes", []string{"AAAAA", "BBBBB", "NOWHERE"}).Return(found, nil)

	// Act
	n, err := navlookup.New(store, nav).LoadIdentsCount([]string{"AAAAA", "BBBBB", "NOWHERE"}, true)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	store.AssertExpectations(t)
}

func TestLoadIdentsCountDoesNotRecordAnEmptyAnswerWhenTheCallerDoubtsIt(t *testing.T) {
	// Arrange
	store := new(navdbmock.Store)
	nav := new(simmock.Nav)
	nav.On("LookupFixes", []string{"AAAAA"}).Return(nil, nil)

	// Act
	n, err := navlookup.New(store, nav).LoadIdentsCount([]string{"AAAAA"}, false)

	// Assert
	require.NoError(t, err)
	assert.Zero(t, n)
	store.AssertNotCalled(t, "SaveFixes")
	store.AssertNotCalled(t, "MarkIdentChecked")
}

func TestLoadIdentsWithoutASimulatorDoesNothing(t *testing.T) {
	// Arrange
	store := new(navdbmock.Store)

	// Act
	err := navlookup.New(store, nil).LoadIdents([]string{"AAAAA"})

	// Assert
	assert.NoError(t, err)
	store.AssertNotCalled(t, "SaveFixes")
}
