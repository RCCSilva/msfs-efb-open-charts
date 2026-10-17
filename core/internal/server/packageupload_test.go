package server

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPackageFileNameIsSafe(t *testing.T) {
	cases := map[string]string{
		"sbsp_adc_adc_20261001.pdf":           "",
		"../../evil.zip":                      "evil.zip",
		`C:\Users\x\pacote-full-20261001.zip`: "pacote-full-20261001.zip",
		"a:b*c?.zip":                          "a_b_c_.zip",
		"notes.txt":                           "",
		"..zip":                               "",
		".zip":                                "",
		"":                                    "",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			// Act
			got := packageFileName(in)

			// Assert
			assert.Equal(t, want, got)
		})
	}
}

func uploadPackage(t *testing.T, s *Server, name string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", name)
	require.NoError(t, err)
	_, _ = fw.Write(body)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/packages/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.RemoteAddr = "127.0.0.1:50000"
	req.Host = "127.0.0.1:43210"
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestPackageUploadKeepsTheZipInThePackagesFolder(t *testing.T) {
	// Arrange
	s, _ := newAccessServer(t, false)

	// Act
	rec := uploadPackage(t, s, "pacote-full-20261001.zip", []byte("PK\x03\x04 not a real zip"))

	// Assert
	require.Equal(t, 201, rec.Code, rec.Body.String())
	var got struct {
		Name string `json:"name"`
		Path string `json:"path"`
		Size int64  `json:"size"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	assert.Equal(t, "pacote-full-20261001.zip", got.Name)
	assert.NotZero(t, got.Size)
	assert.Equal(t, s.packagesDir(), filepath.Dir(got.Path), "saved outside the packages folder")
	assert.FileExists(t, got.Path)
}

func TestPackageUploadKeepsOnlyTheBaseNameOfAPath(t *testing.T) {
	// Arrange
	s, _ := newAccessServer(t, false)

	// Act
	rec := uploadPackage(t, s, "../../escape.zip", []byte("PK\x03\x04"))

	// Assert
	require.Equal(t, 201, rec.Code)
	_, err := os.Stat(filepath.Join(s.packagesDir(), "escape.zip"))
	assert.NoError(t, err, "the base name should have been kept in the packages folder")
}

func TestPackageUploadRefusesOtherTypes(t *testing.T) {
	// A PDF goes through the import flow, not through here.
	cases := map[string][]byte{"a.pdf": []byte("%PDF-1.4"), "script.exe": []byte("MZ")}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			s, _ := newAccessServer(t, false)

			// Act
			rec := uploadPackage(t, s, name, body)

			// Assert
			assert.Equal(t, 400, rec.Code)
		})
	}
}
