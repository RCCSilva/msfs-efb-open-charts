package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// tileCache proxies OpenStreetMap raster tiles and keeps them on disk, so the simulator only
// ever talks to http://127.0.0.1 (no TLS surprises inside Coherent GT) and OSM is hit once.
type tileCache struct {
	dir      string
	upstream string
	ua       string
	client   *http.Client
	slots    chan struct{} // limits simultaneous requests to the upstream (OSM asks for few)
	backoff  time.Duration // first retry delay; doubles on each attempt

	mu       sync.Mutex
	inflight map[string]*pending
}

type pending struct {
	done chan struct{}
	data []byte
	err  error
}

func newTileCache(dir string) *tileCache {
	return &tileCache{
		dir:      dir,
		upstream: "https://tile.openstreetmap.org/%d/%d/%d.png",
		// OSM's tile policy requires an identifying User-Agent.
		ua:       "OpenCharts/0.1 (personal non-commercial MSFS chart viewer)",
		client:   &http.Client{Timeout: 10 * time.Second},
		slots:    make(chan struct{}, 4),
		backoff:  400 * time.Millisecond,
		inflight: map[string]*pending{},
	}
}

const tileMaxZoom = 18

func (t *tileCache) handle(w http.ResponseWriter, r *http.Request) {
	z, err1 := strconv.Atoi(r.PathValue("z"))
	x, err2 := strconv.Atoi(r.PathValue("x"))
	y, err3 := strconv.Atoi(strings.TrimSuffix(r.PathValue("file"), ".png"))
	if err1 != nil || err2 != nil || err3 != nil || z < 0 || z > tileMaxZoom || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
		http.NotFound(w, r)
		return
	}
	data, err := t.truecolor(z, x, y)
	if err != nil {
		// Do not let the browser cache the failure; Retry-After lets a client try again soon.
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Retry-After", "2")
		http.Error(w, "tile fetch failed", http.StatusServiceUnavailable)
		return
	}
	sendTile(w, data)
}

// original returns the OpenStreetMap PNG of a tile: from the disk cache, or fetched (once, even
// when several requests ask for it at the same time).
func (t *tileCache) original(z, x, y int) ([]byte, error) {
	file := filepath.Join(t.dir, strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".png")
	if data, err := os.ReadFile(file); err == nil {
		return data, nil
	}
	key := fmt.Sprintf("%d/%d/%d", z, x, y)
	t.mu.Lock()
	p, ok := t.inflight[key]
	if !ok {
		p = &pending{done: make(chan struct{})}
		t.inflight[key] = p
		go func() {
			p.data, p.err = t.fetch(z, x, y, file)
			t.mu.Lock()
			delete(t.inflight, key)
			t.mu.Unlock()
			close(p.done)
		}()
	}
	t.mu.Unlock()
	<-p.done
	return p.data, p.err
}

func sendTile(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = w.Write(data)
}

// fetch gets one tile from the upstream, retrying on throttling (429) and server errors, with a
// small number of simultaneous requests. A failed tile used to stay failed until the user moved
// the map; now it is retried here and, if it still fails, logged.
func (t *tileCache) fetch(z, x, y int, file string) ([]byte, error) {
	const attempts = 3
	var last error
	delay := t.backoff
	for n := 1; n <= attempts; n++ {
		data, retry, err := t.fetchOnce(z, x, y)
		if err == nil {
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err == nil {
				tmp := file + ".tmp"
				if os.WriteFile(tmp, data, 0o644) == nil {
					_ = os.Rename(tmp, file)
				}
			}
			return data, nil
		}
		last = err
		if !retry || n == attempts {
			break
		}
		time.Sleep(delay)
		delay *= 2
	}
	log.Printf("tiles: %d/%d/%d: %v", z, x, y, last)
	return nil, last
}

// fetchOnce does a single upstream request. retry tells whether trying again may help.
func (t *tileCache) fetchOnce(z, x, y int) (data []byte, retry bool, err error) {
	t.slots <- struct{}{}
	defer func() { <-t.slots }()

	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(t.upstream, z, x, y), nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", t.ua)
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, true, err // network error or timeout
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		retry = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, retry, fmt.Errorf("upstream HTTP %d", resp.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, true, err
	}
	return data, false, nil
}
