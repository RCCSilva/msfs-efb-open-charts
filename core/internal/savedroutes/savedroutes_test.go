package savedroutes_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/navdb/navdbmock"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/route"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/savedroutes"
)

var errBoom = errors.New("boom")

// resolverMock is a mock of savedroutes.Resolver.
type resolverMock struct{ mock.Mock }

func (m *resolverMock) Resolve(text string, choices []route.Choice) (*route.Result, error) {
	args := m.Called(text, choices)
	r, _ := args.Get(0).(*route.Result)
	return r, args.Error(1)
}

func found(idents ...string) *route.Result {
	res := &route.Result{}
	for _, id := range idents {
		res.Points = append(res.Points, route.Point{Ident: id, Kind: "airport"})
	}
	return res
}

func TestName(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"nothing written", "  ", ""},
		{"one place", "sbsp", "SBSP"},
		{"the first and the last place", "SBSP GERTU UZ25 PABIN SBBR", "SBSP > SBBR"},
		{"what is not a place of the route is left out", "sbsj dct BOBI3A/N0400F250 DAGUB uz6 SBGR", "SBSJ > SBGR"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange: the text of the case

			// Act
			got := savedroutes.Name(c.text)

			// Assert
			assert.Equal(t, c.want, got)
		})
	}
}

func TestCheckResolvesTheTextWithItsChoices(t *testing.T) {
	// Arrange
	choices := []route.Choice{{Token: "RBV", Nth: 0, Region: "K6"}}
	raw, _ := json.Marshal(choices)
	resolver := new(resolverMock)
	resolver.On("Resolve", "SBSP RBV SBGR", choices).Return(found("SBSP", "RBV", "SBGR"), nil)
	svc := savedroutes.New(new(navdbmock.SavedRoutes), resolver)

	// Act
	res, err := svc.Check("  SBSP   RBV SBGR ", raw)

	// Assert
	require.NoError(t, err)
	assert.Len(t, res.Points, 3)
	resolver.AssertExpectations(t)
}

func TestCheckRefusesWhatWouldBeIncomplete(t *testing.T) {
	cases := []struct {
		name   string
		text   string
		result *route.Result
		err    error
		check  func(t *testing.T, err error)
	}{
		{"nothing written", "   ", nil, nil, func(t *testing.T, err error) { assert.ErrorIs(t, err, savedroutes.ErrEmpty) }},
		{"a name that was not found", "SBSP ZZZZZ SBBR", &route.Result{
			Points:   []route.Point{{Ident: "SBSP"}, {Ident: "SBBR"}},
			Problems: []route.Problem{{Token: "ZZZZZ", Message: "not found"}},
		}, nil, func(t *testing.T, err error) {
			var nf *savedroutes.NotFoundError
			require.ErrorAs(t, err, &nf)
			assert.Equal(t, []string{"ZZZZZ"}, nf.Names)
			assert.Contains(t, err.Error(), "ZZZZZ")
		}},
		{"nothing was found", "ZZZZZ", &route.Result{}, nil, func(t *testing.T, err error) { assert.ErrorIs(t, err, savedroutes.ErrNothingFound) }},
		{"the navigation data fails", "SBSP SBBR", nil, errBoom, func(t *testing.T, err error) {
			var re *savedroutes.ResolveError
			require.ErrorAs(t, err, &re)
			assert.ErrorIs(t, err, errBoom)
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Arrange
			resolver := new(resolverMock)
			resolver.On("Resolve", mock.Anything, mock.Anything).Return(c.result, c.err)
			svc := savedroutes.New(new(navdbmock.SavedRoutes), resolver)

			// Act
			res, err := svc.Check(c.text, nil)

			// Assert
			assert.Nil(t, res)
			c.check(t, err)
		})
	}
}

func TestCheckAcceptsANameWithADoubtBecauseItWasResolved(t *testing.T) {
	// Arrange
	resolver := new(resolverMock)
	resolver.On("Resolve", mock.Anything, mock.Anything).Return(&route.Result{
		Points:   []route.Point{{Ident: "SBSP"}, {Ident: "RBV"}},
		Problems: []route.Problem{{Token: "RBV", Candidates: []route.Candidate{{Region: "FA"}, {Region: "K6"}}}},
	}, nil)
	svc := savedroutes.New(new(navdbmock.SavedRoutes), resolver)

	// Act
	res, err := svc.Check("SBSP RBV", nil)

	// Assert
	require.NoError(t, err)
	assert.Len(t, res.Points, 2)
}

func TestCreateKeepsTheRouteWithItsPlacesAndNamesIt(t *testing.T) {
	// Arrange
	resolver := new(resolverMock)
	resolver.On("Resolve", "SBSP SBBR", mock.Anything).Return(found("SBSP", "SBBR"), nil)
	store := new(navdbmock.SavedRoutes)
	store.On("CreateSavedRoute", mock.MatchedBy(func(r navdb.SavedRoute) bool {
		return r.Name == "SBSP > SBBR" && r.Text == "SBSP SBBR" && len(r.Result) > 0 && r.ID == 0
	})).Return(navdb.SavedRoute{ID: 7, Name: "SBSP > SBBR", Text: "SBSP SBBR", Result: json.RawMessage(`{"points":[]}`)}, nil)
	svc := savedroutes.New(store, resolver)

	// Act
	got, err := svc.Create(navdb.SavedRoute{Text: "SBSP SBBR"})

	// Assert
	require.NoError(t, err)
	assert.EqualValues(t, 7, got.ID)
	assert.Equal(t, "SBSP > SBBR", got.Summary)
	assert.Equal(t, 2, got.Names)
	store.AssertExpectations(t)
}

func TestCreateKeepsTheNameTheUserGave(t *testing.T) {
	// Arrange
	resolver := new(resolverMock)
	resolver.On("Resolve", mock.Anything, mock.Anything).Return(found("SBSP", "SBBR"), nil)
	store := new(navdbmock.SavedRoutes)
	store.On("CreateSavedRoute", mock.MatchedBy(func(r navdb.SavedRoute) bool { return r.Name == "Sunday flight" })).
		Return(navdb.SavedRoute{ID: 1, Name: "Sunday flight", Text: "SBSP SBBR"}, nil)
	svc := savedroutes.New(store, resolver)

	// Act
	got, err := svc.Create(navdb.SavedRoute{Name: "Sunday flight", Text: "SBSP SBBR"})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Sunday flight", got.Name)
}

func TestACreatedRouteThatIsNotValidIsNotStored(t *testing.T) {
	// Arrange
	resolver := new(resolverMock)
	resolver.On("Resolve", mock.Anything, mock.Anything).Return(&route.Result{
		Points:   []route.Point{{Ident: "SBSP"}},
		Problems: []route.Problem{{Token: "ZZZZZ"}},
	}, nil)
	store := new(navdbmock.SavedRoutes)
	svc := savedroutes.New(store, resolver)

	// Act
	_, err := svc.Create(navdb.SavedRoute{Text: "SBSP ZZZZZ"})

	// Assert
	var nf *savedroutes.NotFoundError
	assert.ErrorAs(t, err, &nf)
	store.AssertNotCalled(t, "CreateSavedRoute", mock.Anything)
}

func TestUpdateReplacesTheRouteWithThatID(t *testing.T) {
	// Arrange
	resolver := new(resolverMock)
	resolver.On("Resolve", mock.Anything, mock.Anything).Return(found("SBSP", "SBGR"), nil)
	store := new(navdbmock.SavedRoutes)
	store.On("UpdateSavedRoute", mock.MatchedBy(func(r navdb.SavedRoute) bool { return r.ID == 4 && r.Text == "SBSP SBGR" })).
		Return(navdb.SavedRoute{ID: 4, Name: "SBSP > SBGR", Text: "SBSP SBGR"}, nil)
	svc := savedroutes.New(store, resolver)

	// Act
	got, err := svc.Update(4, navdb.SavedRoute{Text: "SBSP SBGR"})

	// Assert
	require.NoError(t, err)
	assert.EqualValues(t, 4, got.ID)
	store.AssertExpectations(t)
}

func TestUpdateOfARouteThatDoesNotExist(t *testing.T) {
	// Arrange
	resolver := new(resolverMock)
	resolver.On("Resolve", mock.Anything, mock.Anything).Return(found("SBSP", "SBGR"), nil)
	store := new(navdbmock.SavedRoutes)
	store.On("UpdateSavedRoute", mock.Anything).Return(nil, navdb.ErrNotFound)
	svc := savedroutes.New(store, resolver)

	// Act
	_, err := svc.Update(99, navdb.SavedRoute{Text: "SBSP SBGR"})

	// Assert
	assert.ErrorIs(t, err, navdb.ErrNotFound)
}

func TestListKeepsTheRoutesSavedBeforeThePlacesWereKept(t *testing.T) {
	// Arrange
	old := navdb.SavedRoute{ID: 1, Name: "old", Text: "SBSP SBBR"}                                             // no places kept
	kept := navdb.SavedRoute{ID: 2, Name: "kept", Text: "SBSP SBGR", Result: json.RawMessage(`{"points":[]}`)} // has them
	broken := navdb.SavedRoute{ID: 3, Name: "broken", Text: "SBSP ZZZZZ"}                                      // cannot be resolved
	store := new(navdbmock.SavedRoutes)
	store.On("SavedRoutes").Return([]navdb.SavedRoute{old, kept, broken}, nil)
	store.On("SetSavedRouteResult", int64(1), mock.Anything).Return(nil)
	resolver := new(resolverMock)
	resolver.On("Resolve", "SBSP SBBR", mock.Anything).Return(found("SBSP", "SBBR"), nil)
	resolver.On("Resolve", "SBSP ZZZZZ", mock.Anything).Return(&route.Result{
		Points: []route.Point{{Ident: "SBSP"}}, Problems: []route.Problem{{Token: "ZZZZZ"}},
	}, nil)
	svc := savedroutes.New(store, resolver)

	// Act
	list, err := svc.List()

	// Assert
	require.NoError(t, err)
	require.Len(t, list, 3)
	assert.NotEmpty(t, list[0].Result, "the old one was resolved and kept")
	assert.JSONEq(t, `{"points":[]}`, string(list[1].Result), "the one that had its places is not resolved again")
	assert.Empty(t, list[2].Result, "one that cannot be resolved stays as it was")
	store.AssertNotCalled(t, "SetSavedRouteResult", int64(2), mock.Anything)
	store.AssertNotCalled(t, "SetSavedRouteResult", int64(3), mock.Anything)
	assert.Equal(t, "SBSP > SBBR", list[0].Summary)
}

func TestListReportsAFailureOfTheStore(t *testing.T) {
	// Arrange
	store := new(navdbmock.SavedRoutes)
	store.On("SavedRoutes").Return(nil, errBoom)

	// Act
	list, err := savedroutes.New(store, new(resolverMock)).List()

	// Assert
	assert.ErrorIs(t, err, errBoom)
	assert.Nil(t, list)
}

func TestDeleteAndGetGoToTheStore(t *testing.T) {
	// Arrange
	store := new(navdbmock.SavedRoutes)
	store.On("DeleteSavedRoute", int64(5)).Return(nil)
	store.On("SavedRoute", int64(5)).Return(navdb.SavedRoute{ID: 5, Text: "SBSP SBBR"}, nil)
	svc := savedroutes.New(store, new(resolverMock))

	// Act
	delErr := svc.Delete(5)
	got, getErr := svc.Get(5)

	// Assert
	assert.NoError(t, delErr)
	require.NoError(t, getErr)
	assert.Equal(t, "SBSP SBBR", got.Text)
	store.AssertExpectations(t)
}
