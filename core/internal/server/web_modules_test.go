package server

import (
	"io/fs"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RCCSilva/msfs-efb-open-charts/core/internal/sim"
	"github.com/RCCSilva/msfs-efb-open-charts/core/web"
)

func TestEveryScriptOfTheManagerIsServedAsJavaScript(t *testing.T) {
	// Arrange
	s := providerServer(t, sim.Provider{})
	h := s.Handler()
	var scripts []string
	require.NoError(t, fs.WalkDir(web.Manager(), ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(p, ".js") {
			scripts = append(scripts, p)
		}
		return err
	}))
	require.NotEmpty(t, scripts)

	for _, p := range scripts {
		// Act
		r := httptest.NewRequest("GET", "http://127.0.0.1:43215/manager/"+p, nil)
		r.RemoteAddr = "127.0.0.1:50000"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		// Assert
		assert.Equal(t, 200, w.Code, p)
		assert.True(t, strings.HasPrefix(w.Header().Get("Content-Type"), "text/javascript"), "%s was served as %q", p, w.Header().Get("Content-Type"))
	}
}

func TestTheManagerLoadsItsScriptAsAModule(t *testing.T) {
	// Arrange
	data, err := fs.ReadFile(web.Manager(), "index.html")
	require.NoError(t, err)

	// Assert
	assert.Contains(t, string(data), `<script type="module" src="manager.js"></script>`)
}
