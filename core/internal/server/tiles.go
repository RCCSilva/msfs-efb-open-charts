package server

import (
	"fmt"
	"io"
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
	file := filepath.Join(t.dir, strconv.Itoa(z), strconv.Itoa(x), strconv.Itoa(y)+".png")
	if data, err := os.ReadFile(file); err == nil {
		sendTile(w, data)
		return
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
	if p.err != nil {
		http.Error(w, "tile fetch failed", http.StatusBadGateway)
		return
	}
	sendTile(w, p.data)
}

func sendTile(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=604800")
	_, _ = w.Write(data)
}

func (t *tileCache) fetch(z, x, y int, file string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf(t.upstream, z, x, y), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", t.ua)
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upstream HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err == nil {
		tmp := file + ".tmp"
		if os.WriteFile(tmp, data, 0o644) == nil {
			_ = os.Rename(tmp, file)
		}
	}
	return data, nil
}
