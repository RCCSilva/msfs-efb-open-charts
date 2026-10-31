package server

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim/simmock"
	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/store"
)

func providerServer(t *testing.T, p sim.Provider) *Server {
	t.Helper()
	lib, err := store.Open(t.TempDir())
	require.NoError(t, err)
	return New(lib, nil, p, "127.0.0.1:43215")
}

func TestSimReportsTheProviderAndWhatItCanDo(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{ID: "xsim", Name: "X Sim", Nav: new(simmock.Nav), App: new(simmock.App)})

	// Act
	w := httptest.NewRecorder()
	s.handleSim(w, httptest.NewRequest("GET", "/api/sim", nil))

	// Assert
	require.Equal(t, 200, w.Code)
	var got struct {
		ID, Name     string
		Capabilities sim.Capabilities
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "xsim", got.ID)
	assert.Equal(t, "X Sim", got.Name)
	assert.True(t, got.Capabilities.AskNav)
	assert.True(t, got.Capabilities.InSimApp)
	assert.False(t, got.Capabilities.LoadPlans)
}

func TestSetupListsTheInstallationsTheAppInstallerFinds(t *testing.T) {
	// Arrange
	app := new(simmock.App)
	app.On("Detect", mock.Anything).Return([]sim.AppInstallation{{
		Edition: "steam", Target: `C:\sim\Community`, AutoStartFile: `C:\sim\EXE.xml`, AutoStart: true,
		App: sim.AppState{Installed: true, ServerURL: "http://127.0.0.1:1/"}, // not where the server is
	}})
	s := providerServer(t, sim.Provider{App: app})

	// Act
	w := httptest.NewRecorder()
	s.handleSetup(w, httptest.NewRequest("GET", "/api/setup", nil))

	// Assert
	require.Equal(t, 200, w.Code)
	var got struct {
		Installs []struct {
			Community, ExeXML string
			AutoStart         bool
			EFB               struct{ Installed, URLMismatch bool }
		}
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Installs, 1)
	assert.Equal(t, `C:\sim\Community`, got.Installs[0].Community, "the manager still reads community and exeXml")
	assert.Equal(t, `C:\sim\EXE.xml`, got.Installs[0].ExeXML)
	assert.True(t, got.Installs[0].AutoStart)
	assert.True(t, got.Installs[0].EFB.Installed)
	assert.True(t, got.Installs[0].EFB.URLMismatch, "the app points to another address than the server's")
}

func TestSetupWithoutAnAppInstallerListsNothing(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{})

	// Act
	w := httptest.NewRecorder()
	s.handleSetup(w, httptest.NewRequest("GET", "/api/setup", nil))

	// Assert
	require.Equal(t, 200, w.Code)
	assert.Contains(t, w.Body.String(), `"installs":[]`)
}

func TestInstallPutsTheAppWhereTheServerIs(t *testing.T) {
	// Arrange
	target := filepath.Clean("/sim/Community")
	app := new(simmock.App)
	s := providerServer(t, sim.Provider{App: app})
	app.On("ValidTarget", target).Return(nil)
	app.On("Install", target, s.serverURL()).Return(nil)
	app.On("State", target).Return(sim.AppState{Installed: true, UpToDate: true})

	// Act
	w := httptest.NewRecorder()
	s.handleInstallEFB(w, httptest.NewRequest("POST", "/api/setup/efb", strings.NewReader(`{"community":"/sim/Community"}`)))

	// Assert
	require.Equal(t, 200, w.Code, w.Body.String())
	app.AssertExpectations(t)
	assert.Contains(t, w.Body.String(), `"upToDate":true`)
}

func TestInstallNeedsAnAppInstaller(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{})

	// Act
	w := httptest.NewRecorder()
	s.handleInstallEFB(w, httptest.NewRequest("POST", "/api/setup/efb", strings.NewReader(`{"community":"/sim/Community"}`)))

	// Assert
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "no app to install")
}

func TestOpenDataFolderShowsTheLibraryFolderAndNothingElse(t *testing.T) {
	// Arrange
	s, _ := libServer(t)
	var opened []string
	s.UseOpener(func(path string) { opened = append(opened, path) })

	// Act
	w := post(s.handleOpenDataFolder, `{"path":"C:\\Windows"}`)

	// Assert
	require.Equal(t, 204, w.Code, w.Body.String())
	assert.Equal(t, []string{s.lib.Dir()}, opened)
}

func TestOpenDataFolderWithoutAnOpenerIsNotAvailable(t *testing.T) {
	// Arrange
	s, _ := libServer(t)

	// Act
	w := post(s.handleOpenDataFolder, ``)

	// Assert
	assert.Equal(t, 501, w.Code)
}

func TestOpenDataFolderIsOnlyForTheComputerItself(t *testing.T) {
	// Arrange / Act / Assert
	assert.True(t, localOnlyPath("/api/setup/open-data-folder"))
}
