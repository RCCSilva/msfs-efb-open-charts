package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/settings"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
)

func favoriteServer(t *testing.T) (*Server, string) {
	t.Helper()
	s := providerServer(t, sim.Provider{})
	dir := t.TempDir()
	s.UseSettings(dir, dir, settings.Default(), true)
	return s, dir
}

func favorite(s *Server, method, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "/api/sources/"+id+"/favorite", nil)
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	s.handleFavoriteSource(w, r)
	return w
}

func favorites(t *testing.T, w *httptest.ResponseRecorder) []string {
	t.Helper()
	var list []sourceView
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	var out []string
	for _, v := range list {
		if v.Favorite {
			out = append(out, v.ID)
		}
	}
	return out
}

func TestSourcesStartWithoutFavorites(t *testing.T) {
	// Arrange
	s, _ := favoriteServer(t)

	// Act
	w := httptest.NewRecorder()
	s.handleSources(w, httptest.NewRequest("GET", "/api/sources", nil))

	// Assert
	require.Equal(t, 200, w.Code)
	assert.Empty(t, favorites(t, w))
}

func TestFavoritingASourceIsKeptAndShownInTheList(t *testing.T) {
	// Arrange
	s, dir := favoriteServer(t)

	// Act
	w := favorite(s, http.MethodPut, "chartfox")
	again := favorite(s, http.MethodPut, "chartfox")

	// Assert
	require.Equal(t, 200, w.Code)
	assert.Equal(t, []string{"chartfox"}, favorites(t, w))
	assert.Equal(t, []string{"chartfox"}, favorites(t, again), "favouriting twice keeps one")
	assert.Equal(t, []string{"chartfox"}, settings.Load(dir).FavoriteSources, "saved in the settings file")
}

func TestUnfavoritingASource(t *testing.T) {
	// Arrange
	s, dir := favoriteServer(t)
	favorite(s, http.MethodPut, "chartfox")
	favorite(s, http.MethodPut, "decea-br")

	// Act
	w := favorite(s, http.MethodDelete, "chartfox")

	// Assert
	assert.Equal(t, []string{"decea-br"}, favorites(t, w))
	assert.Equal(t, []string{"decea-br"}, settings.Load(dir).FavoriteSources)
}

func TestFavoritingAnUnknownSourceIsRefused(t *testing.T) {
	// Arrange
	s, dir := favoriteServer(t)

	// Act
	w := favorite(s, http.MethodPut, "no-such-source")

	// Assert
	assert.Equal(t, 404, w.Code)
	assert.Empty(t, settings.Load(dir).FavoriteSources)
}

func TestFavoritingIsOnlyForThisComputer(t *testing.T) {
	// Act and Assert
	assert.True(t, localOnlyPath("/api/sources/chartfox/favorite"))
	assert.False(t, localOnlyPath("/api/sources"))
}
