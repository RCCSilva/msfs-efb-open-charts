package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func newTestTiles(t *testing.T, upstream http.Handler) (*tileCache, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)
	tc := newTileCache(filepath.Join(t.TempDir(), "tiles"))
	tc.upstream = srv.URL + "/%d/%d/%d.png"
	tc.backoff = time.Millisecond
	return tc, srv
}

func get(tc *tileCache, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tiles/{z}/{x}/{file}", tc.handle)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

// throttledThenFine answers 429 to the first n calls and a tile after them.
func throttledThenFine(n int32, calls *int32) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(calls, 1) <= n {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("PNGDATA"))
	})
}

// A throttled tile (429) must be retried and succeed, not stay black until the user zooms.
func TestTileRetriesOnThrottling(t *testing.T) {
	// Arrange
	var calls int32
	tc, _ := newTestTiles(t, throttledThenFine(2, &calls))

	// Act
	rec := get(tc, "/tiles/3/2/1.png")

	// Assert
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "PNGDATA", rec.Body.String())
	assert.EqualValues(t, 3, atomic.LoadInt32(&calls))
}

func TestTileIsServedFromTheDiskCacheTheSecondTime(t *testing.T) {
	// Arrange
	var calls int32
	tc, _ := newTestTiles(t, throttledThenFine(0, &calls))
	get(tc, "/tiles/3/2/1.png")

	// Act
	rec := get(tc, "/tiles/3/2/1.png")

	// Assert
	assert.Equal(t, 200, rec.Code)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls), "no further upstream call")
}

// A client error (404) is not retried, and the failure is not cacheable.
func TestTileNotFoundIsNotRetried(t *testing.T) {
	// Arrange
	var calls int32
	tc, _ := newTestTiles(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.NotFound(w, r)
	}))

	// Act
	rec := get(tc, "/tiles/3/2/1.png")

	// Assert
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"), "a failed tile must not be cacheable")
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))
}

// Many tiles asked at once must not hit the upstream with more than 4 simultaneous requests.
func TestTileUpstreamConcurrencyIsLimited(t *testing.T) {
	// Arrange
	var cur, peak int32
	tc, _ := newTestTiles(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&cur, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&cur, -1)
		_, _ = w.Write([]byte("x"))
	}))
	const total = 24

	// Act
	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			get(tc, "/tiles/6/"+strconv.Itoa(i)+"/5.png")
		}()
	}
	wg.Wait()

	// Assert
	assert.LessOrEqual(t, atomic.LoadInt32(&peak), int32(4))
}
