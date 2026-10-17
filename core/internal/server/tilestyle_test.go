package server

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// a small coloured "map": white ground, a green park, an orange road and a black label
func fakeOSMTile(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for y := 0; y < 256; y++ {
		for x := 0; x < 256; x++ {
			c := color.RGBA{248, 244, 240, 255} // OSM ground
			switch {
			case x > 40 && x < 120 && y > 40 && y < 120:
				c = color.RGBA{173, 209, 158, 255} // park
			case y > 150 && y < 158:
				c = color.RGBA{252, 214, 164, 255} // road
			case x > 180 && x < 200 && y > 60 && y < 64:
				c = color.RGBA{0, 0, 0, 255} // label
			}
			img.Set(x, y, c)
		}
	}
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, img))
	return b.Bytes()
}

func gray(t *testing.T, jpg []byte, x, y int) uint8 {
	t.Helper()
	img, err := jpeg.Decode(bytes.NewReader(jpg))
	require.NoError(t, err, "not a JPEG")
	r, _, _, _ := img.At(x, y).RGBA()
	return uint8(r >> 8)
}

func TestRestyleLightKeepsTheGroundLight(t *testing.T) {
	// Arrange
	src := fakeOSMTile(t)

	// Act
	out, err := restyle(src, BasemapLight)

	// Assert
	require.NoError(t, err)
	assert.GreaterOrEqual(t, int(gray(t, out, 5, 5)), 225)
}

func TestRestyleLightKeepsLabelsReadable(t *testing.T) {
	// Arrange
	src := fakeOSMTile(t)

	// Act
	out, err := restyle(src, BasemapLight)

	// Assert
	require.NoError(t, err)
	ground, label := int(gray(t, out, 5, 5)), int(gray(t, out, 190, 62))
	assert.Less(t, label, ground-80, "labels must still contrast with the ground")
}

func TestRestyleLightIsQuiet(t *testing.T) {
	// Arrange
	src := fakeOSMTile(t)

	// Act
	out, err := restyle(src, BasemapLight)

	// Assert
	require.NoError(t, err)
	ground, park := int(gray(t, out, 5, 5)), int(gray(t, out, 80, 80))
	assert.LessOrEqual(t, ground-park, 70, "a park must be only slightly different from the ground")
}

func TestRestyleDarkIsDark(t *testing.T) {
	// Arrange
	src := fakeOSMTile(t)

	// Act
	out, err := restyle(src, BasemapDark)

	// Assert
	require.NoError(t, err)
	assert.LessOrEqual(t, int(gray(t, out, 5, 5)), 60, "dark ground")
	assert.Greater(t, gray(t, out, 190, 62), gray(t, out, 5, 5), "labels must be lighter than the ground")
}

// styledTiles is a styled-tile handler over a fake OSM that counts how often it is asked.
func styledTiles(t *testing.T) (get func(path string) *httptest.ResponseRecorder, calls *int32) {
	t.Helper()
	calls = new(int32)
	tc, _ := newTestTiles(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(calls, 1)
		_, _ = w.Write(fakeOSMTile(t))
	}))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tiles/{style}/{z}/{x}/{file}", tc.handleStyled)
	return func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}, calls
}

func TestStyledTileIsAJPEG(t *testing.T) {
	// Arrange
	get, _ := styledTiles(t)

	// Act
	rec := get("/tiles/light/5/3/2.jpg")

	// Assert
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	_, err := jpeg.Decode(bytes.NewReader(rec.Body.Bytes()))
	assert.NoError(t, err)
}

// Styled tiles are made from the original, which is fetched from OSM only once.
func TestStyledTilesOfTheSameTileShareTheOriginal(t *testing.T) {
	// Arrange
	get, calls := styledTiles(t)
	get("/tiles/light/5/3/2.jpg")

	// Act
	dark := get("/tiles/dark/5/3/2.jpg")
	again := get("/tiles/light/5/3/2.jpg")

	// Assert
	assert.Equal(t, 200, dark.Code)
	assert.Equal(t, 200, again.Code)
	assert.EqualValues(t, 1, atomic.LoadInt32(calls), "no second download")
}

func TestStyledTileRefusesWhatIsNotAKnownStyleOrTile(t *testing.T) {
	cases := map[string]string{
		"an unknown style":    "/tiles/neon/5/3/2.jpg",
		"a tile out of range": "/tiles/light/3/99/2.jpg",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			get, _ := styledTiles(t)

			// Act
			rec := get(path)

			// Assert
			assert.Equal(t, 404, rec.Code)
		})
	}
}
