package navdb

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnAircraftProfileIsStoredAndListedByName(t *testing.T) {
	// Arrange
	d := openTemp(t)
	_, err := d.CreateAircraftProfile(AircraftProfile{Name: "Zlin", Data: json.RawMessage(`{"a":1}`)})
	require.NoError(t, err)

	// Act
	b, err := d.CreateAircraftProfile(AircraftProfile{Name: " Beech ", Data: json.RawMessage(`{"a":2}`)})
	list, listErr := d.AircraftProfiles()

	// Assert
	require.NoError(t, err)
	require.NoError(t, listErr)
	assert.Equal(t, "Beech", b.Name)
	require.Len(t, list, 2)
	assert.Equal(t, "Beech", list[0].Name)
	assert.JSONEq(t, `{"a":2}`, string(list[0].Data))
}

func TestAnAircraftProfileNeedsANameAndValidData(t *testing.T) {
	for name, p := range map[string]AircraftProfile{
		"no name":  {Name: " ", Data: json.RawMessage(`{}`)},
		"no data":  {Name: "x"},
		"not json": {Name: "x", Data: json.RawMessage(`{`)},
	} {
		// Arrange
		d := openTemp(t)

		// Act
		_, err := d.CreateAircraftProfile(p)

		// Assert
		assert.ErrorIs(t, err, ErrInvalid, name)
	}
}

func TestAnAircraftProfileCanBeChangedAndRemoved(t *testing.T) {
	// Arrange
	d := openTemp(t)
	p, err := d.CreateAircraftProfile(AircraftProfile{Name: "A", Data: json.RawMessage(`{"v":1}`)})
	require.NoError(t, err)

	// Act
	p.Name, p.Data = "B", json.RawMessage(`{"v":2}`)
	_, updErr := d.UpdateAircraftProfile(p)
	got, getErr := d.AircraftProfile(p.ID)
	delErr := d.DeleteAircraftProfile(p.ID)
	_, goneErr := d.AircraftProfile(p.ID)
	missing := d.DeleteAircraftProfile(p.ID)

	// Assert
	require.NoError(t, updErr)
	require.NoError(t, getErr)
	assert.Equal(t, "B", got.Name)
	assert.JSONEq(t, `{"v":2}`, string(got.Data))
	assert.NoError(t, delErr)
	assert.ErrorIs(t, goneErr, ErrNotFound)
	assert.ErrorIs(t, missing, ErrNotFound)
}

func TestARouteKeepsItsAircraftAndAltitudeAndLosesTheAircraftWhenItIsRemoved(t *testing.T) {
	// Arrange
	d := openTemp(t)
	p, err := d.CreateAircraftProfile(AircraftProfile{Name: "A", Data: json.RawMessage(`{}`)})
	require.NoError(t, err)
	r, err := d.CreateSavedRoute(SavedRoute{Name: "r", Text: "SBSJ SBGR", ProfileID: &p.ID, CruiseFt: 8000})
	require.NoError(t, err)

	// Act
	before, err := d.SavedRoute(r.ID)
	require.NoError(t, err)
	require.NoError(t, d.DeleteAircraftProfile(p.ID))
	after, err := d.SavedRoute(r.ID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, before.ProfileID)
	assert.Equal(t, p.ID, *before.ProfileID)
	assert.Equal(t, 8000, before.CruiseFt)
	assert.Nil(t, after.ProfileID)
	assert.Equal(t, 8000, after.CruiseFt, "the altitude stays")
}

func TestARouteIsRefusedWithAnImpossibleAltitude(t *testing.T) {
	// Act
	_, err := openTemp(t).CreateSavedRoute(SavedRoute{Name: "r", Text: "SBSJ SBGR", CruiseFt: 90000})

	// Assert
	assert.ErrorIs(t, err, ErrInvalid)
}
