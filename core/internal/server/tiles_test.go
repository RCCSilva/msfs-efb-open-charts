package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
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

// A throttled tile (429) must be retried and succeed, not stay black until the user zooms.
func TestTileRetriesOnThrottling(t *testing.T) {
	var calls int32
	tc, _ := newTestTiles(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("PNGDATA"))
	}))
	rec := get(tc, "/tiles/3/2/1.png")
	if rec.Code != 200 || rec.Body.String() != "PNGDATA" {
		t.Fatalf("got %d %q, want 200 PNGDATA", rec.Code, rec.Body.String())
	}
	if n := atomic.LoadInt32(&calls); n != 3 {
		t.Fatalf("upstream called %d times, want 3", n)
	}
	// now cached: no further upstream call
	if rec := get(tc, "/tiles/3/2/1.png"); rec.Code != 200 || atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("second request must be served from the disk cache")
	}
}

// A client error (404) is not retried, and the failure is not cacheable.
func TestTileNotFoundIsNotRetried(t *testing.T) {
	var calls int32
	tc, _ := newTestTiles(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		http.NotFound(w, r)
	}))
	rec := get(tc, "/tiles/3/2/1.png")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("got %d, want 503", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("a failed tile must not be cacheable, got %q", rec.Header().Get("Cache-Control"))
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("upstream called %d times, want 1", n)
	}
}

// Many tiles asked at once must not hit the upstream with more than 4 simultaneous requests.
func TestTileUpstreamConcurrencyIsLimited(t *testing.T) {
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
	done := make(chan struct{})
	const total = 24
	for i := 0; i < total; i++ {
		go func(i int) {
			get(tc, "/tiles/6/"+itoa(i)+"/5.png")
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < total; i++ {
		<-done
	}
	if p := atomic.LoadInt32(&peak); p > 4 {
		t.Fatalf("peak concurrent upstream requests = %d, want <= 4", p)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}
